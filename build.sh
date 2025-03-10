#!/bin/bash

EXPORT CGO_ENABLED=0
EXPORT GOOS=linux
EXPORT GOARCH=amd64

rm -rf ./dist
mkdir -p ./dist

echo "=== build frontend ==="
cd frontend
npm install
npm run build
cd ..
copy ./frontend/dist ./backend/dist

echo "===build backend ==="
cd backend
go mod tidy
go build -ldflags="-s -w" -o ../dist/proxy-subscription .
cd ..

echo "=== build success ==="
echo "Binary file located at: ./dist/proxy-subscription"