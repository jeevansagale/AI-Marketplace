#!/usr/bin/env bash

set -e

echo "======================================="
echo " AI Marketplace Development Setup"
echo "======================================="

echo ""
echo "Checking required tools..."

command -v git >/dev/null 2>&1 || { echo "Git not installed."; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Docker not installed."; exit 1; }
command -v docker-compose >/dev/null 2>&1 || command -v docker >/dev/null 2>&1 || {
    echo "Docker Compose not found."
    exit 1
}
command -v go >/dev/null 2>&1 || { echo "Go not installed."; exit 1; }

echo "All dependencies found."

echo ""
echo "Starting infrastructure..."

docker compose up -d postgres redis

echo ""
echo "Waiting for services..."
sleep 10

echo ""
echo "Downloading Go modules..."

go mod download

echo ""
echo "Building project..."

go build ./...

echo ""
echo "======================================="
echo "Setup Complete!"
echo "======================================="
echo ""
echo "Start API Gateway:"
echo ""
echo "go run ./apps/gateway"
echo ""
echo "Or:"
echo ""
echo "make run"