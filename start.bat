@echo off
chcp 65001 >nul
title 股票行情服务

echo ====================================
echo   股票行情服务
echo ====================================
echo.

REM 检查配置文件
if not exist config.yaml (
    echo [错误] 配置文件 config.yaml 不存在！
    echo.
    echo 请按照以下步骤操作:
    echo   1. 复制 config.yaml.example 为 config.yaml
    echo   2. 编辑 config.yaml 填入你的 API Token
    echo   3. 重新运行本脚本
    echo.
    pause
    exit /b 1
)

echo [信息] 正在启动服务...
echo.

stock.exe -config config.yaml

if errorlevel 1 (
    echo.
    echo [错误] 服务启动失败！
    pause
)
