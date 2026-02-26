#!/bin/bash

# 默认值
TOKEN=""
BASE_URL=""
MODEL=""
CLI_MODE=false

# 显示帮助
show_help() {
    echo "A股/期货实时行情服务 启动脚本"
    echo ""
    echo "用法: ./start.sh [选项]"
    echo ""
    echo "选项:"
    echo "  -t, --token <TOKEN>      设置 API Token (ANTHROPIC_AUTH_TOKEN)"
    echo "  -b, --base <BASE_URL>    设置 API Base URL (ANTHROPIC_BASE_URL)"
    echo "  -m, --model <MODEL>      设置模型名称 (ANTHROPIC_MODEL)"
    echo "  -c, --cli                终端实时行情模式"
    echo "  -h, --help               显示帮助信息"
    echo ""
    echo "示例:"
    echo "  ./start.sh                           # 启动 HTTP 服务"
    echo "  ./start.sh -c                        # 启动终端模式"
    echo "  ./start.sh -t sk-xxx -b http://proxy.com -m claude-sonnet-4-5-20250929"
    echo ""
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--token)
            TOKEN="$2"
            shift 2
            ;;
        -b|--base)
            BASE_URL="$2"
            shift 2
            ;;
        -m|--model)
            MODEL="$2"
            shift 2
            ;;
        -c|--cli)
            CLI_MODE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
done

echo ""
echo "========================================"
echo "    A Stock / Futures Real-time Service"
echo "========================================"
echo ""

# 设置环境变量
if [ -n "$TOKEN" ]; then
    export ANTHROPIC_AUTH_TOKEN="$TOKEN"
    echo "[CONFIG] API Token set"
fi

if [ -n "$BASE_URL" ]; then
    export ANTHROPIC_BASE_URL="$BASE_URL"
    echo "[CONFIG] API Base URL: $BASE_URL"
fi

if [ -n "$MODEL" ]; then
    export ANTHROPIC_MODEL="$MODEL"
    echo "[CONFIG] Model: $MODEL"
fi

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 检查是否已编译
if [ ! -f "./stock" ] && [ ! -f "./stock.exe" ]; then
    echo "[BUILD] Compiling..."
    go build -o stock ./cmd/stock
    if [ $? -ne 0 ]; then
        echo "[ERROR] Build failed"
        exit 1
    fi
    echo "[BUILD] Done"
fi

# 确定可执行文件名
EXE="./stock"
if [ -f "./stock.exe" ]; then
    EXE="./stock.exe"
fi

echo "[START] Starting service..."
echo ""

# 启动服务
if [ "$CLI_MODE" = true ]; then
    $EXE -cli -standalone
else
    $EXE
fi
