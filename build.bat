@echo off

:: 设置构建环境变量
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

:: 清理旧的构建产物
if exist .\dist rmdir /s /q .\dist
mkdir .\dist

:: Build the frontend
echo === Build the frontend ===
cd frontend
call npm install
call npm run build
cd ..
xcopy .\frontend\dist .\backend\dist /E /H /Y /D /Q

:: Building the backend
echo === Building the backend ===
cd backend
call go mod tidy
call go build -ldflags="-s -w" -o ..\dist\proxy-subscription.exe .
cd ..

echo === Build Complete ===
echo The binary files are located at: .\dist\proxy-subscription.exe