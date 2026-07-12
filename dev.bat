@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

REM 转发参数给 dev.ps1
REM 用法: dev.bat
REM       dev.bat -NoKillPort
REM       dev.bat -BackendOnly
REM       dev.bat -FrontendOnly

where powershell >nul 2>&1
if errorlevel 1 (
    echo 未找到 PowerShell，无法启动 dev.ps1
    pause
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0dev.ps1" %*
set EXITCODE=%ERRORLEVEL%
if %EXITCODE% neq 0 (
    echo.
    echo 开发脚本退出码: %EXITCODE%
    pause
)
exit /b %EXITCODE%