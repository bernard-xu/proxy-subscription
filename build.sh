#!/bin/bash

set -euo pipefail

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

rm -rf ./dist
mkdir -p ./dist


rm -rf ./backend/dist
mkdir -p ./backend/dist

echo "=== build frontend ==="
cd frontend
npm install
npm run build
cd ..
cp -R ./frontend/dist/. ./backend/dist/

echo "===build backend ==="
cd backend
go mod tidy
go build -ldflags="-s -w" -o ../dist/proxy-subscription .
cd ..

echo "=== build success ==="
echo "Binary file located at: ./dist/proxy-subscription"
