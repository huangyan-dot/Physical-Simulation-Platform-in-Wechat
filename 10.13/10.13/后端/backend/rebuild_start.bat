@echo off
cd /d %~dp0
taskkill /F /IM server.exe >nul 2>&1
echo building server.exe (current source, incl. L29 cholesky_solve) ...
go build -o server.exe ./cmd/server
if errorlevel 1 (echo BUILD FAILED & pause & exit /b 1)
start "physics-lab-backend" /min server.exe
echo backend rebuilt and started on :8080
