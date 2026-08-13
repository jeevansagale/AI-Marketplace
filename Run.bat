@echo off
title AI Marketplace Development Setup

echo =======================================
echo    AI Marketplace Development Setup
echo =======================================
echo.

where git >nul 2>nul
if %errorlevel% neq 0 (
    echo Git is not installed.
    pause
    exit /b
)

where docker >nul 2>nul
if %errorlevel% neq 0 (
    echo Docker is not installed.
    pause
    exit /b
)

where go >nul 2>nul
if %errorlevel% neq 0 (
    echo Go is not installed.
    pause
    exit /b
)

echo Starting Docker services...
docker compose up -d postgres redis

echo.
echo Waiting for services...
timeout /t 10 >nul

echo.
echo Downloading Go modules...
go mod download

echo.
echo Building project...
go build ./...

echo.
echo =======================================
echo Setup Complete!
echo =======================================
echo.
echo To start the gateway:
echo.
echo go run ./apps/gateway
echo.
pause