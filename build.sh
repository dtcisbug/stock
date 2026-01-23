#!/bin/bash

# 股票行情服务打包脚本
# 支持 Windows (amd64) 平台编译

set -e  # 遇到错误立即退出

# 配置变量
VERSION="${VERSION:-1.0.0}"
APP_NAME="stock-行情服务"
BUILD_DIR="dist"
PLATFORMS="windows/amd64"

echo "======================================"
echo "  股票行情服务打包脚本"
echo "  版本: ${VERSION}"
echo "======================================"
echo ""

# 清理旧的构建产物
echo "[1/5] 清理旧的构建产物..."
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

# 构建前端
echo ""
echo "[2/5] 构建前端资源..."
cd web

# 检查 node_modules 是否存在
if [ ! -d "node_modules" ]; then
    echo "  安装前端依赖..."
    npm install
fi

echo "  执行前端构建..."
npm run build

cd ..

if [ ! -d "web/dist" ]; then
    echo "❌ 错误: 前端构建失败,web/dist 目录不存在"
    exit 1
fi

echo "✅ 前端构建完成"

# 构建 Go 程序
echo ""
echo "[3/5] 编译 Go 程序..."

# 解析平台信息
IFS='/' read -r GOOS GOARCH <<< "$PLATFORMS"

# 设置输出文件名
OUTPUT_NAME="${APP_NAME}-v${VERSION}-${GOOS}-${GOARCH}"
if [ "$GOOS" = "windows" ]; then
    BINARY_NAME="stock.exe"
else
    BINARY_NAME="stock"
fi

echo "  目标平台: ${GOOS}/${GOARCH}"
echo "  输出文件: ${BINARY_NAME}"

# 编译
GOOS=$GOOS GOARCH=$GOARCH go build -o ${BUILD_DIR}/${BINARY_NAME} \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    .

if [ ! -f "${BUILD_DIR}/${BINARY_NAME}" ]; then
    echo "❌ 错误: 编译失败"
    exit 1
fi

echo "✅ 编译完成: ${BUILD_DIR}/${BINARY_NAME}"

# 准备发布目录
echo ""
echo "[4/5] 准备发布目录..."

RELEASE_DIR="${BUILD_DIR}/${OUTPUT_NAME}"
mkdir -p ${RELEASE_DIR}

# 复制文件
cp ${BUILD_DIR}/${BINARY_NAME} ${RELEASE_DIR}/
cp config.yaml.example ${RELEASE_DIR}/
cp README.md ${RELEASE_DIR}/ 2>/dev/null || echo "  警告: README.md 不存在,跳过"

# 创建 Windows 启动脚本(如果是 Windows 平台)
if [ "$GOOS" = "windows" ]; then
    cat > ${RELEASE_DIR}/start.bat << 'EOF'
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
EOF
    echo "✅ 已创建 start.bat"
fi

# 创建 README (如果原来没有)
if [ ! -f "${RELEASE_DIR}/README.md" ]; then
    cat > ${RELEASE_DIR}/README.md << 'EOF'
# 股票行情服务

A股/期货实时行情监控系统,集成 Claude AI 走势分析功能。

## 快速开始

### 1. 配置

首次使用需要配置 API Token:

```bash
# Windows
copy config.yaml.example config.yaml

# 然后编辑 config.yaml 填入你的 API Token
```

### 2. 启动

```bash
# Windows - 双击运行
start.bat

# 或命令行运行
stock.exe -config config.yaml
```

### 3. 访问

打开浏览器访问: http://localhost:19527

## 配置说明

编辑 `config.yaml` 文件:

- `api.token`: Anthropic API Token (必填)
- `api.base_url`: API 地址,默认官方地址
- `api.model`: 使用的模型名称
- `monitor.stocks`: 监控的股票列表
- `monitor.futures`: 监控的期货列表
- `server.port`: HTTP 服务端口
- `server.enable_ai`: 是否启用 AI 分析
- `server.sync_interval`: 数据同步间隔(秒)

## API 接口

- `GET /api/stocks` - 获取所有股票行情
- `GET /api/stock/:code` - 获取单只股票
- `GET /api/futures` - 获取所有期货行情
- `GET /api/futures/:code` - 获取单个期货
- `GET /api/analysis` - 获取所有 AI 分析
- `GET /api/analysis/:code` - 获取单个 AI 分析
- `GET /api/status` - 服务状态

## 常见问题

### Q: 如何获取 API Token?
A: 访问 https://console.anthropic.com/settings/keys 创建 API Key

### Q: 如何修改监控的股票?
A: 编辑 config.yaml 中的 monitor.stocks 列表

### Q: 如何关闭 AI 分析?
A: 在 config.yaml 中设置 server.enable_ai: false
EOF
    echo "✅ 已创建 README.md"
fi

echo "✅ 发布目录准备完成: ${RELEASE_DIR}"

# 打包压缩
echo ""
echo "[5/5] 创建压缩包..."

cd ${BUILD_DIR}

# 创建 ZIP 压缩包
if command -v zip &> /dev/null; then
    zip -r "${OUTPUT_NAME}.zip" "${OUTPUT_NAME}" > /dev/null
    echo "✅ 已创建: ${OUTPUT_NAME}.zip"
else
    echo "⚠ 未安装 zip 命令,跳过 ZIP 打包"
fi

# 创建 tar.gz 压缩包
if command -v tar &> /dev/null; then
    tar -czf "${OUTPUT_NAME}.tar.gz" "${OUTPUT_NAME}"
    echo "✅ 已创建: ${OUTPUT_NAME}.tar.gz"
else
    echo "⚠ 未安装 tar 命令,跳过 tar.gz 打包"
fi

cd ..

# 显示构建结果
echo ""
echo "======================================"
echo "  ✅ 打包完成！"
echo "======================================"
echo ""
echo "产物位置:"
ls -lh ${BUILD_DIR}/*.zip ${BUILD_DIR}/*.tar.gz 2>/dev/null || echo "  ${BUILD_DIR}/${OUTPUT_NAME}/"
echo ""
echo "文件大小:"
du -sh ${BUILD_DIR}/${OUTPUT_NAME}
echo ""
echo "内容清单:"
ls -lh ${BUILD_DIR}/${OUTPUT_NAME}/
echo ""
