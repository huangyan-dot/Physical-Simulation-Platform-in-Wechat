@echo off
chcp 65001 >nul
title 数值分析实验 - 后端环境启动
echo ============================================
echo   数值分析实验小程序 - 一键启动 MySQL + 后端
echo ============================================
echo.

REM 1. 启动 MySQL（数据目录固定为 C:\Users\当前用户\mysql_data）
set MYSQL_BIN="C:\Program Files\MySQL\MySQL Server 8.4\bin"
set MYSQL_DATA=C:\Users\%USERNAME%\mysql_data
set SQL_DIR=%~dp0后端\backend\migrations
set BACKEND_DIR=%~dp0后端\backend

echo [1/4] 启动 MySQL...
netstat -ano | findstr ":3306" | findstr LISTENING >nul
if %errorlevel%==0 (
    echo       MySQL 已在运行，跳过。
) else (
    if not exist "%MYSQL_DATA%\ibdata1" (
        echo       首次运行：初始化 MySQL 数据目录...
        mkdir "%MYSQL_DATA%" 2>nul
        %MYSQL_BIN%\mysqld.exe --initialize-insecure --datadir="%MYSQL_DATA%"
    )
    start "MySQL(physics_lab)" /min %MYSQL_BIN%\mysqld.exe --datadir="%MYSQL_DATA%" --port=3306
    echo       等待 MySQL 就绪...
    timeout /t 5 /nobreak >nul
    %MYSQL_BIN%\mysql.exe -u root -h 127.0.0.1 -e "CREATE DATABASE IF NOT EXISTS physics_lab DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>nul
    %MYSQL_BIN%\mysql.exe -u root -h 127.0.0.1 -e "CREATE USER IF NOT EXISTS 'physics'@'%%' IDENTIFIED BY 'physics123'; GRANT ALL PRIVILEGES ON physics_lab.* TO 'physics'@'%%'; FLUSH PRIVILEGES;" 2>nul
)

REM 2. 幂等补跑数值分析实验迁移 006-025（绝不跑 004/005，那两个会清空实验）
echo [2/4] 检查/同步实验数据（migrations 006-025，均为可重复执行）...
for %%N in (006 007 008 009 010 011 012 013 014 015 016 017 018 019 020 021 022 023 024 025) do (
    for %%F in ("%SQL_DIR%\%%N"_*.sql) do (
        %MYSQL_BIN%\mysql.exe -u physics -pphysics123 --default-character-set=utf8mb4 physics_lab < "%%F" 2>nul
    )
)
echo       实验数据已同步（当前应为 19 个数值分析实验）。

REM 3. 检查后端端口 8080
echo [3/4] 检查后端服务...
netstat -ano | findstr ":8080" | findstr LISTENING >nul
if %errorlevel%==0 (
    echo       后端已在运行，无需重复启动。
    goto done
)

REM 4. 启动 Go 后端（需要本机已安装 Go）
echo [4/4] 启动 Go 后端（首次编译约需 1 分钟）...
cd /d "%BACKEND_DIR%"
start "Go后端(:8080)" cmd /k "go run ./cmd/server"

:done
echo.
echo ============================================
echo   完成！现在打开微信开发者工具：
echo   模拟器登录请点下方 学生/教师/管理员
echo   （微信一键登录需先在 config.yaml 配 AppID/AppSecret）
echo ============================================
timeout /t 8 /nobreak >nul
