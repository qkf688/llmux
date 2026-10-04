# LLMux 本地开发：后端 (7070) + 前端 Vite HMR (5173)
# 用法:
#   .\dev.ps1
#   $env:JWT_SECRET="xxx"; .\dev.ps1   # 后端只认 JWT_SECRET，缺则启动即退出
#   .\dev.ps1 -NoKillPort
#   .\dev.ps1 -BackendOnly
#   .\dev.ps1 -FrontendOnly
param(
    [switch]$NoKillPort,
    [switch]$BackendOnly,
    [switch]$FrontendOnly
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
$Webui = Join-Path $Root "webui"
$BackendProc = $null
$FrontendProc = $null

function Get-PortListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
}

function Stop-PortListener {
    param([int]$Port)
    $conns = Get-PortListener -Port $Port
    if (-not $conns) { return }
    $pids = $conns | Select-Object -ExpandProperty OwningProcess -Unique
    foreach ($procId in $pids) {
        if ($procId -le 4) { continue }
        try {
            $p = Get-Process -Id $procId -ErrorAction Stop
            Write-Host "释放端口 $Port：停止 PID $procId ($($p.ProcessName))"
            Stop-Process -Id $procId -Force -ErrorAction Stop
        } catch {
            Write-Host "警告：无法停止 PID $procId：$($_.Exception.Message)"
        }
    }
    Start-Sleep -Seconds 1
}

function Assert-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "未找到命令: $Name（请确认已安装并在 PATH 中）"
    }
}

# TCP 直连探活：比 Test-NetConnection 快一个量级，适合 200ms 级轮询。
# 连上即说明后端已 listen（Go 侧不做 TLS，握手后立即断开属正常）。
function Test-PortReady {
    param([int]$Port, [int]$ProbeTimeoutMs = 300)
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne($ProbeTimeoutMs, $false)) { return $false }
        $client.EndConnect($async)
        return $true
    } catch {
        return $false
    } finally {
        $client.Close()
    }
}

# 等后端真正可连，而不是睡固定秒数（go run 的编译+链接+迁移常要 6~10s）。
# 返回 $true=已就绪，$false=超时（调用方决定是否继续拉前端）。
# 后端早退直接抛错：watch 循环一旦发现它退出就会连带杀掉前端，
# 与其让用户对着一个秒塌的会话猜原因，不如立刻报真实错误。
function Wait-BackendReady {
    param(
        [System.Diagnostics.Process]$Process,
        [int]$Port = 7070,
        [int]$TimeoutSeconds = 30
    )
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        if ($Process.HasExited) {
            throw "后端进程已退出 (code $($Process.ExitCode))。原因见上方 [backend] 输出；常见项：JWT_SECRET 未设置、7070 仍被占用、数据库迁移失败"
        }
        if (Test-PortReady -Port $Port) { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

# 后端只认 JWT_SECRET（main.go 硬校验），banner 必须如实说明它从哪来。
function Get-JwtSecretSource {
    if ($env:JWT_SECRET) { return "环境变量" }
    $envFile = Join-Path $Root ".env"
    if ((Test-Path $envFile) -and
        (Select-String -Path $envFile -Pattern '^\s*JWT_SECRET\s*=\s*\S+' -Quiet)) {
        return ".env"
    }
    return "未设置"
}

# 通过 cmd 启动，兼容 go.exe / pnpm.cmd（避免直接 Start-Process pnpm.ps1）
function Start-CmdProcess {
    param(
        [Parameter(Mandatory = $true)][string]$CommandLine,
        [Parameter(Mandatory = $true)][string]$WorkingDirectory
    )
    return Start-Process -FilePath "cmd.exe" `
        -ArgumentList @("/c", $CommandLine) `
        -WorkingDirectory $WorkingDirectory `
        -NoNewWindow -PassThru
}

function Stop-DevChildren {
    foreach ($proc in @($FrontendProc, $BackendProc)) {
        if ($null -eq $proc) { continue }
        try {
            if (-not $proc.HasExited) {
                # /T 结束进程树，避免 go/node 子进程残留
                & taskkill.exe /F /T /PID $proc.Id 2>$null | Out-Null
            }
        } catch { }
    }
}

try {
    if ($BackendOnly -and $FrontendOnly) {
        throw "不能同时指定 -BackendOnly 与 -FrontendOnly"
    }

    Assert-Command "go"
    if (-not $BackendOnly) {
        Assert-Command "pnpm"
        if (-not (Test-Path (Join-Path $Webui "package.json"))) {
            throw "未找到 webui/package.json"
        }
    }

    if (-not $NoKillPort) {
        if (-not $FrontendOnly) { Stop-PortListener -Port 7070 }
        if (-not $BackendOnly) { Stop-PortListener -Port 5173 }
    } elseif (-not $BackendOnly -and (Get-PortListener -Port 5173)) {
        # Vite 没开 strictPort：端口被占会静默改用下一个，而 banner 仍打印 5173，
        # 结果就是打开的是上一个实例或干脆打不开。提前说清楚，别等用户自己撞。
        Write-Host "警告：端口 5173 已被占用且未释放（-NoKillPort）。Vite 会自动改用下一个端口，banner 里的 5173 将失效——请以 [webui] 输出里的 Local 地址为准" -ForegroundColor Yellow
    }

    $dbDir = Join-Path $Root "db"
    if (-not (Test-Path $dbDir)) {
        New-Item -ItemType Directory -Path $dbDir | Out-Null
    }

    Write-Host ""
    Write-Host "========================================"
    Write-Host " LLMux 开发模式"
    Write-Host "========================================"
    if (-not $FrontendOnly) {
        Write-Host " 后端 API : http://localhost:7070"
    }
    if (-not $BackendOnly) {
        Write-Host " 管理界面 : http://localhost:5173  (Vite HMR)"
        Write-Host " /api 代理 -> http://localhost:7070"
    }
    $jwtSource = Get-JwtSecretSource
    if (-not $FrontendOnly) {
        Write-Host " JWT_SECRET: $jwtSource"
        if ($jwtSource -eq "未设置") {
            Write-Host "            [警告] 后端缺 JWT_SECRET 会立即退出（main.go 硬校验）" -ForegroundColor Yellow
        }
    }
    Write-Host " 退出     : Ctrl+C"
    Write-Host "========================================"
    Write-Host ""

    if (-not $FrontendOnly) {
        Write-Host "[backend] go run ."
        $BackendProc = Start-CmdProcess -CommandLine "go run ." -WorkingDirectory $Root
    }

    if ($BackendOnly) {
        Write-Host "仅后端模式，等待进程结束..."
        Wait-Process -Id $BackendProc.Id
        exit $BackendProc.ExitCode
    }

    # 后端真正 listen 后再拉前端：Vite 的 /api 代理写死 target 7070，
    # 固定 sleep 只能赌运气，冷编译时首屏必然连不上。
    if (-not $FrontendOnly) {
        if (Wait-BackendReady -Process $BackendProc) {
            Write-Host "[backend] 已就绪 http://localhost:7070"
        } else {
            Write-Host "警告：30s 内后端仍未监听 7070，继续启动前端（代理会暂时连不上，直到后端起来）" -ForegroundColor Yellow
        }
    }

    if (-not (Test-Path (Join-Path $Webui "node_modules"))) {
        Write-Host "[webui] pnpm install ..."
        Push-Location $Webui
        try {
            & pnpm install
            if ($LASTEXITCODE -ne 0) { throw "pnpm install 失败" }
        } finally {
            Pop-Location
        }
    }

    Write-Host "[webui] pnpm dev"
    $FrontendProc = Start-CmdProcess -CommandLine "pnpm dev" -WorkingDirectory $Webui

    # 任一子进程退出则结束开发会话
    while ($true) {
        Start-Sleep -Milliseconds 500
        if ($FrontendProc.HasExited) {
            Write-Host "[webui] 已退出 (code $($FrontendProc.ExitCode))"
            break
        }
        if ($null -ne $BackendProc -and $BackendProc.HasExited) {
            Write-Host "[backend] 已退出 (code $($BackendProc.ExitCode))"
            Write-Host "          原因见上方 [backend] 输出；常见项：JWT_SECRET 未设置、7070 被占用、迁移失败" -ForegroundColor Yellow
            break
        }
    }
} catch {
    Write-Host "错误: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    Stop-DevChildren
}