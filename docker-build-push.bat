@echo off
echo ========================================
echo Docker Build and Push Script
echo ========================================
echo.

echo Step 1: Building Docker image...
docker build -t qkf688/llmux:latest .

if %errorlevel% neq 0 (
    echo.
    echo ❌ Docker build failed!
    pause
    exit /b 1
)

echo.
echo ✅ Docker build completed successfully!
echo.

echo Step 2: Pushing image to Docker Hub...
docker push qkf688/llmux:latest

if %errorlevel% neq 0 (
    echo.
    echo ❌ Docker push failed! Please check your Docker Hub credentials.
    pause
    exit /b 1
)

echo.
echo ✅ Docker push completed successfully!
echo.
echo ========================================
echo 🎉 All operations completed successfully!
echo ========================================
pause