@echo off
cd /d %~dp0
tasklist /FI "IMAGENAME eq server.exe" | find /I "server.exe" >nul && (echo backend already running & exit /b)
start "physics-lab-backend" /min server.exe
echo backend started on :8080
