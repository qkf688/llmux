@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion
cd /d "%~dp0"

REM 整包启动：构建 webui（可跳过）+ go run，并释放 7070
REM 用法:
REM   run.bat
REM   run.bat --no-build     跳过前端构建（使用已有 webui/dist）
REM   run.bat --help

set "NO_BUILD=0"
if /i "%~1"=="--no-build" set "NO_BUILD=1"
if /i "%~1"=="-NoBuild" set "NO_BUILD=1"
if /i "%~1"=="--help" goto :help
if /i "%~1"=="-h" goto :help

echo Checking if port 7070 is in use...
for /f "tokens=5" %%a in ('netstat -ano ^| findstr :7070 ^| findstr LISTENING') do (
    echo Found process using port 7070: PID %%a
    echo Stopping process %%a...
    taskkill /F /PID %%a >nul 2>&1
    if !errorlevel! equ 0 (
        echo Process %%a stopped successfully.
    ) else (
        echo Failed to stop process %%a, continuing anyway...
    )
    timeout /t 2 /nobreak >nul
)

if "%NO_BUILD%"=="1" (
    echo Skipping webui build (--no-build^)
    if not exist "webui\dist\index.html" (
        echo 警告: 未找到 webui\dist\index.html，内嵌管理页可能不可用。
        echo 请先执行: cd webui ^&^& pnpm run build
        echo 或去掉 --no-build 参数。
    )
    goto :start_server
)

echo Building webui...
cd /d "%~dp0webui"
if not exist "package.json" (
    echo Failed to change directory to webui or package.json not found
    pause
    exit /b 1
)

where pnpm >nul 2>&1
if errorlevel 1 (
    echo pnpm not found in PATH
    pause
    exit /b 1
)

call pnpm run build
if errorlevel 1 (
    echo Failed to build webui
    cd /d "%~dp0"
    pause
    exit /b 1
)

cd /d "%~dp0"

:start_server
echo Starting server...
echo API / 内嵌管理页: http://localhost:7070
echo 日常前端开发请用: dev.bat  ^(http://localhost:5173^)
go run .
set EXITCODE=%ERRORLEVEL%
if %EXITCODE% neq 0 (
    echo go run 退出码: %EXITCODE%
)
pause
exit /b %EXITCODE%

:help
echo.
echo run.bat — 构建 WebUI 并启动 Go 服务（embed 整包）
echo.
echo 用法:
echo   run.bat              构建 webui 后启动
echo   run.bat --no-build   跳过构建，直接 go run（需已有 webui\dist）
echo.
echo 日常开发（前后端分离 + HMR）请使用:
echo   dev.bat
echo   或  .\dev.ps1
echo.
pause
exit /b 0