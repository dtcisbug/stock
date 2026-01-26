.PHONY: help
# 帮助：列出常用命令（把项目里的编译/启动/回测/扫描脚本统一到 make 入口）
help:
	@echo "Targets:"
	@echo "  make build            # go build (mac/linux) -> ./stock"
	@echo "  make build-windows    # go build (windows)   -> ./stock.exe"
	@echo "  make test             # go test ./..."
	@echo "  make run              # start.sh (server mode)"
	@echo "  make cli              # start.sh -c (CLI mode)"
	@echo "  make cli-ai           # run-cli-ai.sh"
	@echo "  make cli-basic        # run-cli.sh"
	@echo "  make package          # build.sh (build web + package windows dist)"
	@echo "  make web-dev          # (cd web && npm run dev)"
	@echo "  make web-build        # (cd web && npm run build)"
	@echo "  make backtest         # run backtest using \$$BT_CONFIG -> \$$BT_OUT"
	@echo "  make scan             # scan latest bar signals"
	@echo "  make scan-only         # scan latest bar signals (only symbols with signals)"
	@echo "  make llm-scan          # ollama scan advice -> \$$ADVICE_OUT"
	@echo ""
	@echo "Vars:"
	@echo "  SCAN_DAYS=365          # 覆盖扫描窗口：最近 N 天（自然日）"
	@echo "  SCAN_CHART=1           # 输出带画线的 SVG 图（scan_charts/*.svg）"
	@echo "  SCAN_CHART_BARS=220    # 每张图输出最近 N 根K线"
	@echo "  LLM_TIMEOUT=10m        # Ollama 超时（首次加载模型可加大）"

# Go 编译器
GO ?= go
# 默认二进制名称（mac/linux）
BIN ?= stock
# 服务配置文件（用于行情服务/扫描合并标的；默认读取当前目录 config.yaml）
CONFIG ?= config.yaml

# 回测配置与输出
BT_CONFIG ?= backtest.yaml
BT_OUT ?= report.json

# 扫描配置（默认复用回测配置）；SCAN_JSON=1 输出 JSON
SCAN_CONFIG ?= $(BT_CONFIG)
SCAN_OUT ?=
SCAN_JSON ?= 0
SCAN_DAYS ?= 0
SCAN_CHART ?= 0
SCAN_CHART_DIR ?= scan_charts
SCAN_CHART_BARS ?= 220

# 本地大模型（Ollama）配置
LLM_URL ?= http://localhost:11434
LLM_MODEL ?= qwen2.5-coder:14b
LLM_TIMEOUT ?= 10m
# LLM 扫描建议：读取哪个配置文件、输出到哪里
ADVICE_CONFIG ?= $(BT_CONFIG)
ADVICE_OUT ?= advice.md

.PHONY: build
# 编译（mac/linux）
build:
	$(GO) build -o $(BIN) .

.PHONY: build-windows
# 交叉编译 Windows amd64（输出 stock.exe）
build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN).exe .

.PHONY: test
# 运行单测
test:
	$(GO) test ./...

.PHONY: run
# 启动 HTTP 服务（使用 start.sh，内部会自动 build）
run:
	./start.sh

.PHONY: cli
# 启动终端行情模式（start.sh -c）
cli:
	./start.sh -c

.PHONY: cli-basic
# 终端模式（run-cli.sh：会重新编译并启动）
cli-basic:
	./run-cli.sh

.PHONY: cli-ai
# 终端 + AI（run-cli-ai.sh：会重新编译并启动）
cli-ai:
	./run-cli-ai.sh

.PHONY: package
# 打包（build.sh：构建 web 前端并输出 dist 发布包）
package:
	./build.sh

.PHONY: web-dev
# 前端开发（首次会 npm install）
web-dev:
	cd web && npm install && npm run dev

.PHONY: web-build
# 前端构建（首次会 npm install）
web-build:
	cd web && npm install && npm run build

.PHONY: backtest
# 运行回测（收盘确认，次日开盘成交）
backtest: build
	./$(BIN) -backtest -bt-config $(BT_CONFIG) -bt-out $(BT_OUT)

.PHONY: scan
# 扫描最新一根日K：是否有信号（次日开盘执行）；输出含 STOP/TARGET
scan: build
	./$(BIN) -scan -bt-config $(SCAN_CONFIG) $(if $(SCAN_OUT),-scan-out $(SCAN_OUT),) $(if $(filter 1,$(SCAN_JSON)),-scan-json,) $(if $(filter-out 0,$(SCAN_DAYS)),-scan-days $(SCAN_DAYS),) $(if $(filter 1,$(SCAN_CHART)),-scan-chart -scan-chart-dir $(SCAN_CHART_DIR) -scan-chart-bars $(SCAN_CHART_BARS),)

.PHONY: scan-only
# 扫描最新一根日K：只输出有信号的标的（错误仍输出）
scan-only: build
	./$(BIN) -scan -scan-only-signal -bt-config $(SCAN_CONFIG) $(if $(SCAN_OUT),-scan-out $(SCAN_OUT),) $(if $(filter 1,$(SCAN_JSON)),-scan-json,) $(if $(filter-out 0,$(SCAN_DAYS)),-scan-days $(SCAN_DAYS),) $(if $(filter 1,$(SCAN_CHART)),-scan-chart -scan-chart-dir $(SCAN_CHART_DIR) -scan-chart-bars $(SCAN_CHART_BARS),)

.PHONY: llm-scan
# 使用本地 Ollama 将扫描结果生成可读的执行清单（Markdown）
llm-scan: build
	./$(BIN) -llm-scan -bt-config $(ADVICE_CONFIG) -llm-url $(LLM_URL) -llm-model $(LLM_MODEL) -llm-timeout $(LLM_TIMEOUT) -llm-out $(ADVICE_OUT) $(if $(filter-out 0,$(SCAN_DAYS)),-scan-days $(SCAN_DAYS),) $(if $(filter 1,$(SCAN_CHART)),-scan-chart -scan-chart-dir $(SCAN_CHART_DIR) -scan-chart-bars $(SCAN_CHART_BARS),)
