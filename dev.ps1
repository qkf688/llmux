# LLMux 本地开发：后端 (7070) + 前端 Vite HMR (5173)
# 用法:
#   .\dev.ps1
#   $env:TOKEN="xxx"; .\dev.ps1
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

function Stop-PortListener {
    param([int]$Port)
    $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
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
    if ($env:TOKEN) {
        Write-Host " TOKEN    : 已设置"
    } else {
        Write-Host " TOKEN    : 未设置（本地无鉴权）"
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

    # 给后端一点启动时间，减少首屏 API 连不上的概率
    if (-not $FrontendOnly) {
        Start-Sleep -Seconds 1
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
            break
        }
    }
} catch {
    Write-Host "错误: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    Stop-DevChildren
}