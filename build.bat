@echo off
chcp 65001 >nul
setlocal

echo ========================================
echo  LLMUX 一键构建脚本
echo ========================================
echo.

echo [1/3] 安装前端依赖...
cd webui
call pnpm install
if %errorlevel% neq 0 (
    echo 前端依赖安装失败！
    cd ..
    pause
    exit /b 1
)

echo.
echo [2/3] 构建前端...
call pnpm run build
if %errorlevel% neq 0 (
    echo 前端构建失败！
    cd ..
    pause
    exit /b 1
)
cd ..

echo.
echo [3/3] 编译 Go 后端...
go build -o llmux.exe .
if %errorlevel% neq 0 (
    echo Go 编译失败！
    pause
    exit /b 1
)

echo.
echo ========================================
echo  构建完成！生成 llmux.exe
echo ========================================
pause
