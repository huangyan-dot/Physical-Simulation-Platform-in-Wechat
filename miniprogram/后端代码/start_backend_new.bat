@echo off
chcp 65001 >nul
cd /d "%~dp0miniprogram\后端代码"
set CONFIG_PATH=%~dp0miniprogram\后端代码\configs\config.yaml
go run cmd/server/main.go cmd/server/seed.go
pause
