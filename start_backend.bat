@echo off
chcp 65001 >nul
echo ============================================
echo   物理实验3D模拟 - 后端启动脚本
echo ============================================
echo.

REM 1. 启动 MySQL（如果还没启动）
echo [1/3] 启动 MySQL...
set MYSQL_BIN="C:\Program Files\MySQL\MySQL Server 8.4\bin"
set MYSQL_DATA=C:\Users\%USERNAME%\mysql_data

if not exist "%MYSQL_DATA%" (
    echo 初始化 MySQL 数据目录...
    mkdir "%MYSQL_DATA%"
    %MYSQL_BIN%\mysqld.exe --initialize-insecure --datadir="%MYSQL_DATA%" --console
)

REM 以后台方式启动 MySQL
start "MySQL" /B %MYSQL_BIN%\mysqld.exe --datadir="%MYSQL_DATA%" --port=3306
echo MySQL 启动中，等待 3 秒...
timeout /t 3 /nobreak >nul

REM 2. 初始化数据库（首次运行）
echo [2/3] 初始化数据库...
%MYSQL_BIN%\mysql.exe -u root -h 127.0.0.1 -P 3306 -e "CREATE DATABASE IF NOT EXISTS physics_lab DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>nul
%MYSQL_BIN%\mysql.exe -u root -h 127.0.0.1 -P 3306 -e "CREATE USER IF NOT EXISTS 'physics'@'%%' IDENTIFIED BY 'physics123'; GRANT ALL PRIVILEGES ON physics_lab.* TO 'physics'@'%%'; FLUSH PRIVILEGES;" 2>nul
echo 数据库已就绪

REM 3. 启动 Go 后端
echo [3/3] 启动 Go 后端...
cd /d "%~dp0后端代码"
set CONFIG_PATH=%~dp0后端代码\configs\config.yaml
go run cmd/server/main.go cmd/server/seed.go

pause