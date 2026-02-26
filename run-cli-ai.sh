#!/bin/bash
cd "$(dirname "$0")"

# AI分析配置
# 方式一: 使用配置文件(推荐)
# 创建 config.yaml 文件并填入你的 API 配置
CONFIG_FILE="config.yaml"

# 方式二: 使用环境变量
# export ANTHROPIC_AUTH_TOKEN="your-api-token-here"
# export ANTHROPIC_MODEL="claude-sonnet-4-5-20250929"
# export ANTHROPIC_BASE_URL="https://api.anthropic.com"

# 检查并关闭已运行的 stock 进程
PIDS=$(pgrep -f "./stock")
if [ -n "$PIDS" ]; then
    echo "发现已运行的进程，正在关闭..."
    kill $PIDS 2>/dev/null
    sleep 1
fi

# 编译最新版本
echo "正在编译最新版本..."
go build -o stock ./cmd/stock

# 检查编译是否成功
if [ $? -eq 0 ]; then
    echo "编译成功，启动程序（启用AI分析）..."

    # 检查是否存在配置文件
    if [ -f "$CONFIG_FILE" ]; then
        echo "使用配置文件: $CONFIG_FILE"
        ./stock -cli -standalone -config "$CONFIG_FILE"
    else
        echo "警告: 未找到配置文件 $CONFIG_FILE，使用环境变量配置"
        ./stock -cli -standalone -ai
    fi
else
    echo "编译失败，请检查代码错误"
    exit 1
fi
