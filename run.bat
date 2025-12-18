@echo off
setlocal enabledelayedexpansion
echo Checking if port 7070 is in use...

REM 检查端口7070是否被占用
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

echo Building webui...
cd /d "%~dp0webui"
if not exist "package.json" (
    echo Failed to change directory to webui or package.json not found
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
echo Starting server...
go run main.go
pause