.PHONY: help
# 帮助：列出常用命令（把项目里的编译/启动/回测/扫描脚本统一到 make 入口）
help:
	@echo "Targets:"
	@echo "  make build            # go build (mac/linux) -> ./stock"
	@echo "  make build-stockd      # go build -> ./stockd"
	@echo "  make build-stockctl    # go build -> ./stockctl"
	@echo "  make build-windows    # go build (windows)   -> ./stock.exe"
	@echo "  make test             # go test ./..."
	@echo "  make serve            # run server (./stock -config \$$CONFIG if exists)"
	@echo "  make serve-stockd      # run server (./stockd)"
	@echo "  make run              # start.sh (server mode)"
	@echo "  make quick            # quick start (build web if needed + run server)"
	@echo "  make cli              # start.sh -c (CLI standalone mode)"
	@echo "  make cli-api           # CLI via stockd API (\$$SERVER)"
	@echo "  make cli-ai           # run-cli-ai.sh"
	@echo "  make cli-basic        # run-cli.sh"
	@echo "  make package          # build.sh (build web + package windows dist)"
	@echo "  make web-dev          # (cd web && npm run dev)"
	@echo "  make web-build        # (cd web && npm run build)"
	@echo "  make backtest         # run backtest using \$$BT_CONFIG -> \$$BT_OUT"
	@echo "  make scan             # scan latest bar signals"
	@echo "  make scan-only         # scan latest bar signals (only symbols with signals)"
	@echo "  make analyze          # 1y analysis (JSON/CSV + charts + index.html) -> \$$ANALYZE_OUT_DIR"
	@echo "  make llm-scan          # ollama scan advice -> \$$ADVICE_OUT"
	@echo ""
	@echo "Vars:"
	@echo "  SERVER=http://localhost:19527  # stockd base url (for cli-api)"
	@echo "  SCAN_DAYS=365          # 覆盖扫描窗口：最近 N 天（自然日）"
	@echo "  SCAN_CHART=1           # 输出带画线的 SVG 图（$(SCAN_CHART_DIR)/*.svg）"
	@echo "  SCAN_CHART_BARS=220    # 每张图输出最近 N 根K线"
	@echo "  ANALYZE_OUT_DIR=runtime/analysis  # 分析输出目录"
	@echo "  ANALYZE_DAYS=365        # 分析窗口：最近 N 个自然日（与 ANALYZE_BARS 互斥）"
	@echo "  ANALYZE_BARS=252        # 分析窗口：最近 N 根日K（优先级更高）"
	@echo "  LLM_TIMEOUT=10m        # Ollama 超时（首次加载模型可加大）"
	@echo "  QUICK_WEB=1            # quick 启动时是否自动构建 web（0 跳过）"

# Go 编译器
GO ?= go
# 默认二进制名称（mac/linux）
BIN ?= stock
# 服务配置文件（用于行情服务/扫描合并标的；默认读取当前目录 config.yaml）
CONFIG ?= config.yaml
SERVER ?= http://localhost:19527

# 运行时产物统一输出目录（默认会被 .gitignore 忽略）
RUNTIME_DIR ?= runtime

# 回测配置与输出
# 优先使用 backtest.yaml；若不存在则回退到 config.yaml（支持单文件整合）
BT_CONFIG ?= $(if $(wildcard backtest.yaml),backtest.yaml,config.yaml)
BT_OUT ?= $(RUNTIME_DIR)/report.json

# 扫描配置（默认复用回测配置）；SCAN_JSON=1 输出 JSON
SCAN_CONFIG ?= $(BT_CONFIG)
SCAN_OUT ?=
SCAN_JSON ?= 0
SCAN_DAYS ?= 0
SCAN_CHART ?= 0
SCAN_CHART_DIR ?= $(RUNTIME_DIR)/scan_charts
SCAN_CHART_BARS ?= 220

# 一年量价分析输出与窗口
ANALYZE_OUT_DIR ?= $(RUNTIME_DIR)/analysis
ANALYZE_DAYS ?= 365
ANALYZE_BARS ?= 0

# 本地大模型（Ollama）配置
LLM_URL ?= http://localhost:11434
LLM_MODEL ?= qwen2.5-coder:14b
LLM_TIMEOUT ?= 10m
# LLM 扫描建议：读取哪个配置文件、输出到哪里
ADVICE_CONFIG ?= $(BT_CONFIG)
ADVICE_OUT ?= $(RUNTIME_DIR)/advice.md

QUICK_WEB ?= 1

.PHONY: build
# 编译（mac/linux）
build:
	$(GO) build -o $(BIN) ./cmd/stock

.PHONY: build-stockd
build-stockd:
	$(GO) build -o stockd ./cmd/stockd

.PHONY: build-stockctl
build-stockctl:
	$(GO) build -o stockctl ./cmd/stockctl

.PHONY: build-windows
# 交叉编译 Windows amd64（输出 stock.exe）
build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BIN).exe ./cmd/stock

.PHONY: test
# 运行单测
test:
	$(GO) test ./...

.PHONY: serve
serve: build
	@if [ -f "$(CONFIG)" ]; then \
		echo "[serve] using config: $(CONFIG)"; \
		./$(BIN) -config $(CONFIG); \
	else \
		echo "[serve] config not found: $(CONFIG) (using defaults/env)"; \
		./$(BIN); \
	fi

.PHONY: serve-stockd
serve-stockd: build-stockd
	@if [ -f "$(CONFIG)" ]; then \
		echo "[serve-stockd] using config: $(CONFIG)"; \
		./stockd -config $(CONFIG); \
	else \
		echo "[serve-stockd] config not found: $(CONFIG) (using defaults/env)"; \
		./stockd; \
	fi

.PHONY: run
# 启动 HTTP 服务（使用 start.sh，内部会自动 build）
run:
	./start.sh

.PHONY: quick
# 快速启动：需要先构建前端（若未构建）；然后启动 HTTP 服务（优先使用 config.yaml）
quick: build
	@if [ "$(QUICK_WEB)" != "0" ]; then \
		if [ ! -d "web/dist" ] || [ -z "$$(ls -A web/dist 2>/dev/null)" ]; then \
			echo "[quick] web/dist not found, building web..."; \
			cd web && npm install && npm run build; \
		else \
			echo "[quick] web/dist exists, skip web build (set QUICK_WEB=0 to always skip)"; \
		fi; \
	else \
		echo "[quick] QUICK_WEB=0, skip web build"; \
	fi
	@if [ -f "$(CONFIG)" ]; then \
		echo "[quick] using config: $(CONFIG)"; \
		./$(BIN) -config $(CONFIG); \
	else \
		echo "[quick] config not found: $(CONFIG) (using defaults/env)"; \
		./$(BIN); \
	fi

.PHONY: cli
# 启动终端行情模式（start.sh -c）
cli:
	./start.sh -c

.PHONY: cli-api
# 终端行情模式（通过 stockd API；要求 stockd 已在运行）
cli-api: build
	./$(BIN) -cli -server $(SERVER)

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
	@mkdir -p $(dir $(BT_OUT))
	./$(BIN) -backtest -bt-config $(BT_CONFIG) -bt-out $(BT_OUT)

.PHONY: scan
# 扫描最新一根日K：是否有信号（次日开盘执行）；输出含 STOP/TARGET
scan: build
	@mkdir -p $(SCAN_CHART_DIR) >/dev/null 2>&1 || true
	./$(BIN) -scan -bt-config $(SCAN_CONFIG) $(if $(SCAN_OUT),-scan-out $(SCAN_OUT),) $(if $(filter 1,$(SCAN_JSON)),-scan-json,) $(if $(filter-out 0,$(SCAN_DAYS)),-scan-days $(SCAN_DAYS),) $(if $(filter 1,$(SCAN_CHART)),-scan-chart -scan-chart-dir $(SCAN_CHART_DIR) -scan-chart-bars $(SCAN_CHART_BARS),)

.PHONY: scan-only
# 扫描最新一根日K：只输出有信号的标的（错误仍输出）
scan-only: build
	@mkdir -p $(SCAN_CHART_DIR) >/dev/null 2>&1 || true
	./$(BIN) -scan -scan-only-signal -bt-config $(SCAN_CONFIG) $(if $(SCAN_OUT),-scan-out $(SCAN_OUT),) $(if $(filter 1,$(SCAN_JSON)),-scan-json,) $(if $(filter-out 0,$(SCAN_DAYS)),-scan-days $(SCAN_DAYS),) $(if $(filter 1,$(SCAN_CHART)),-scan-chart -scan-chart-dir $(SCAN_CHART_DIR) -scan-chart-bars $(SCAN_CHART_BARS),)

.PHONY: analyze
# 一年量价分析（蔡森破底翻）：输出 JSON/CSV + 价格/成交量 K 线图（SVG）
analyze: build
	@mkdir -p $(ANALYZE_OUT_DIR) >/dev/null 2>&1 || true
	@if [ -f "$(CONFIG)" ]; then \
		if [ "$(ANALYZE_BARS)" != "0" ]; then \
			./$(BIN) -analyze -config $(CONFIG) -bt-config $(BT_CONFIG) -analyze-out-dir $(ANALYZE_OUT_DIR) -analyze-bars $(ANALYZE_BARS); \
		else \
			./$(BIN) -analyze -config $(CONFIG) -bt-config $(BT_CONFIG) -analyze-out-dir $(ANALYZE_OUT_DIR) -analyze-window-days $(ANALYZE_DAYS); \
		fi; \
	else \
		echo "[analyze] config not found: $(CONFIG)"; \
		exit 2; \
	fi

.PHONY: llm-scan
# 使用本地 Ollama 将扫描结果生成可读的执行清单（Markdown）
llm-scan: build
	@mkdir -p $(dir $(ADVICE_OUT))
	./$(BIN) -llm-scan -bt-config $(ADVICE_CONFIG) -llm-url $(LLM_URL) -llm-model $(LLM_MODEL) -llm-timeout $(LLM_TIMEOUT) -llm-out $(ADVICE_OUT) $(if $(filter-out 0,$(SCAN_DAYS)),-scan-days $(SCAN_DAYS),) $(if $(filter 1,$(SCAN_CHART)),-scan-chart -scan-chart-dir $(SCAN_CHART_DIR) -scan-chart-bars $(SCAN_CHART_BARS),)
