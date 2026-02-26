# 工程概览：`stock`（A 股/期货行情 + 扫描/回测 + 可选 AI/LLM）

> 目的：把这个仓库“是什么、怎么跑、主要模块怎么协作、有哪些边界/坑”一次讲清楚，便于后续迭代与交接。

## 1. 这是什么（定位与边界）

本工程提供三类能力：

1) **实时行情服务**（核心主线）  
- 拉取 A 股与期货（含部分外盘 `hf_`）的实时行情，写入内存缓存  
- 对外提供 HTTP REST API（并可内置/独立运行前端）  
- 支持终端 CLI 实时看盘

2) **策略工具**（实验）  
- 日线回测：**收盘确认产生信号 → 次日开盘成交**的执行模型  
- 最新信号扫描：检查“最新一根日 K 收盘是否产生信号”，并输出持仓/止损/目标等信息  
- 可选输出趋势上下文图（SVG，含支撑/压力/止损/目标等画线）

3) **AI/LLM 辅助**（可选）  
- Claude：基于最近约 3 个月日 K 数据生成简短走势分析（默认不超过 300 字）  
- Claude 分析结果会持久化到 `runtime/ai/analysis.json`，服务重启后会自动加载（删除该文件可清空历史）  
- Ollama：  
  - 自然语言 → 生成并校验 `backtest.yaml`（严格 JSON → YAML）  
  - `runtime/report.json` → 复盘总结（Markdown）  
  - 扫描结果 → 可读的执行清单建议（Markdown）

**边界/限制（很重要）**
- 回测/扫描目前支持：**A 股（`sh/sz`）与国内期货（`nf_`）的日线**。  
- 外盘 `hf_`（如 `hf_CL/hf_SI`）目前只用于**实时行情监控**，回测/扫描会跳过（见 `scan_config.go`、`backtest/config.go` 的过滤逻辑）。
- 数据源基于公开接口（新浪/东方财富等），存在不可用/字段变更风险。

---

## 2. 运行形态（你在跑的到底是哪条链路）

本仓库提供 3 个二进制入口：
- `stockd`：仅服务端（HTTP API + 可选内置前端）
- `stockctl`：仅 CLI 工具（实时行情 CLI + scan/backtest/analyze/llm）
- `stock`：wrapper（默认等同 `stockd`；传入 `-cli/-scan/-backtest/-analyze/-llm-*` 时等同 `stockctl`，用于兼容原有用法）

### 2.1 服务模式（HTTP + 可选内置前端）
- 入口：`./stockd`（或 `./stock` 不带 CLI/扫描参数）
- 启动后：  
  - API：`/api/...`  
  - 健康检查：`/health`  
  - 前端：若已构建 `web/dist` 并内嵌，则 `/`、`/assets/...` 可访问

### 2.2 CLI 模式（终端实时看盘）
- 入口：`./stockctl -cli`（或 `./stock -cli` / `./start.sh -c`）
- 交易时段：每 5 秒刷新一次终端输出  
- 非交易时段：等待（最多 60 秒）AI 分析结果后打印一次，随后不再刷新（直到 Ctrl+C）

### 2.3 回测/扫描/LLM 模式（一次性运行并退出）
这些模式不会启动 HTTP 服务：
- 回测：`./stockctl -backtest -bt-config backtest.yaml [-bt-out runtime/report.json]`（或 `./stock -backtest ...`）
- 扫描：`./stockctl -scan -bt-config backtest.yaml [...]`（或 `./stock -scan ...`）
- LLM（Ollama）：`-llm-gen-bt` / `-llm-analyze` / `-llm-scan`

---

## 3. 数据源与刷新逻辑（实时与日线各走各的）

### 3.1 实时行情
- A 股实时：`fetcher/stock.go`（新浪 `hq.sinajs.cn/list=...`，GBK 解码）
- 期货实时：`fetcher/futures.go`（新浪 `hq.sinajs.cn/list=...`，兼容 `nf_` 与 `hf_` 不同字段布局）

### 3.2 日线 K 线（用于 Claude / 回测 / 扫描）
- A 股日线：`fetcher/kline.go` 使用东方财富 `push2his.eastmoney.com`  
- 国内期货日线：`fetcher/kline.go` 使用新浪 `InnerFuturesNewService.getDailyKLine`  
- `hf_` 外盘：当前没有对应日线拉取实现，因此被排除在回测/扫描之外

### 3.3 刷新调度与交易时间
在 `main.go:runDataSync`：
- 国内（A 股/国内期货）只在 `trading.IsTradingTime()` 为 true 时刷新  
- 外盘（`hf_`）单独 ticker，**始终刷新**（避免与国内时段耦合）

交易时间判断在 `trading/time.go`（CST，简化规则）：
- A 股：9:30–11:30，13:00–15:00（周一至周五）
- 期货：日盘 + 夜盘（夜盘按最大范围近似）

---

## 4. 配置体系（两套 YAML，且支持“单文件整合”）

### 4.1 服务配置：`config.yaml`
读取逻辑：`config/config.go:GetConfig`  
优先级（以实际实现为准）：**默认值 < 配置文件 < 环境变量**（仅对 token/base/model 这类“支持环境变量覆盖”的字段生效）

关键字段（与 `README.md` 一致）：
- `api.token/base_url/model`：Claude 访问参数
- `monitor.stocks`、`monitor.futures`：监控标的
- `server.port`、`server.enable_ai`、`server.sync_interval`

环境变量（向后兼容）：
- Token：`ANTHROPIC_AUTH_TOKEN`（优先）/ `ANTHROPIC_API_KEY` / `CLAUDE_API_KEY`
- Base：`ANTHROPIC_BASE_URL` / `ANTHROPIC_API_BASE`
- Model：`ANTHROPIC_MODEL`

期货代码会做归一化（支持简写）：
- `pp2605` / `AU0` → `nf_PP2605` / `nf_AU0`
- `CL`/`WTI`、`SI`/`SILVER` → `hf_CL` / `hf_SI`

### 4.2 回测/扫描配置：`backtest.yaml`
读取逻辑：`backtest/config.go:LoadRunConfig`

核心结构：
- `backtest.*`：时间范围、资金、滑点/手续费、仓位、保证金参数等
- `backtest.instruments.stocks/futures`：回测标的
- `strategy.type`：`tsai_sen`（默认）或 `patterns`
- `strategy.params`：策略参数

**单文件整合**：如果 `backtest.instruments.*` 没填，程序会尝试从同一个 YAML 的 `monitor.stocks/futures` 读取标的列表（方便只维护一个 `config.yaml`）。

### 4.3 扫描时的“配置合并”
扫描入口：`scan_config.go:loadScanRunConfig`
- 先读 `bt-config`（策略/回测参数来源）  
- 再读 `config.yaml`（监控标的来源；默认会尝试当前目录 `config.yaml`）  
- 将 `monitor.stocks` + `monitor.futures(过滤 hf_)` 合并进 instruments（避免漏扫）

---

## 5. 命令行参数总览（建议把它当成“功能开关面板”）

`main.go` 主要参数：
- 基础：`-config`、`-cli`、`-ai`
- 回测：`-backtest`、`-bt-config`、`-bt-out`
- 扫描：`-scan`、`-scan-out`、`-scan-json`、`-scan-only-signal`、`-scan-days`、`-scan-chart`、`-scan-chart-dir`、`-scan-chart-bars`
- Ollama：`-llm-gen-bt`、`-llm-analyze <report.json>`、`-llm-scan`、`-llm-url`、`-llm-model`、`-llm-out`、`-llm-bt-config`、`-llm-timeout`、`-llm-scan-only-signal`

---

## 6. HTTP API（给前端/外部系统用）

路由实现：`api/server.go`、`api/handler.go`（Gin）

核心接口：
- `GET /api/stocks`、`GET /api/stock/:code`
- `GET /api/futures`、`GET /api/futures/:code`
- `GET /api/analysis`、`GET /api/analysis/:code`（仅 AI 启用且有结果时可用）
- `GET /api/status`（交易状态、缓存更新时间、数量、AI 是否启用）
- `GET /health`

前端静态资源（如果 `web/dist` 已内嵌）：`/`、`/assets/...`、`/vite.svg`、以及兼容 `/static/...`

---

## 7. 代码结构（按“职责”而不是按目录凑数）

- `main.go`：运行模式分发（服务/CLI/回测/扫描/LLM），以及定时同步与 AI 调度
- `config/`：服务配置加载、环境变量覆盖、期货代码归一化
- `fetcher/`：实时行情 + 日线 K 线数据拉取与解析
- `cache/`：内存缓存（`sync.Map`），给 API 与 CLI 读
- `api/`：Gin HTTP 服务（REST + 静态资源）
- `analyzer/`：Claude 分析器（拉取日线 → 拼 prompt → 调 Anthropic messages API → 缓存结果）
- `backtest/`：日线回测引擎、扫描、策略实现（`tsai_sen`、`patterns`）与 SVG 出图
- `llm/`：Ollama 客户端 + prompt 模板 + scan/report 摘要结构
- `trading/`：交易时间判断（简化版）
- `web/`：Vue3 + Vite 前端（可独立开发，也可构建后内嵌）
- `dist/`：打包产物（通常由 `build.sh` 生成）

---

## 8. 常用工作流（建议照着跑）

更详细的“扫描/回测/年度分析”说明见：`docs/SCAN_BACKTEST_ANALYZE.md`。

### 8.1 启动服务（最常用）
1) `cp config.yaml.example config.yaml`（填 token 可选）  
2) `make quick` 或 `./stockd -config config.yaml`（也可直接 `./stockd`/`./stock`，会自动优先使用 `./config.yaml`）  

说明：
- `./stockd`/`./stock` 若未显式 `-config`，会自动优先使用当前目录的 `config.yaml`（若存在）。  
- `./start.sh` 也依赖上述行为：更适合“开箱即用”，需要时再显式传 `-t/-b/-m` 覆盖环境变量。

### 8.2 CLI 看盘
- `./stockctl -cli -standalone -config config.yaml`（或 `./stock -cli -standalone ...` / `./start.sh -c`）

### 8.3 开启 Claude 分析
满足任一即可：
- `./stock -ai -config config.yaml`  
- 或配置 `server.enable_ai: true` 且提供 token  

### 8.4 回测 / 扫描 / 画图
- 回测：`./stock -backtest -bt-config backtest.yaml -bt-out runtime/report.json`
- 扫描：`./stock -scan -bt-config backtest.yaml -scan-only-signal`
- 扫描 + 图：`./stock -scan -bt-config backtest.yaml -scan-days 365 -scan-chart -scan-chart-bars 220`

### 8.5 Ollama（本地大模型）
- 生成回测配置：`echo "...策略描述..." | ./stock -llm-gen-bt -llm-url http://localhost:11434 -llm-model qwen2.5-coder:14b -llm-out backtest.yaml`
- 复盘：`./stock -llm-analyze runtime/report.json -llm-bt-config backtest.yaml > runtime/review.md`
- 扫描建议：`./stock -llm-scan -bt-config backtest.yaml -scan-chart -llm-out runtime/advice.md`

### 8.6 Makefile 快捷入口
- `make help`：查看常用目标
- `make build` / `make build-windows`：编译二进制
- `make run` / `make cli`：用脚本启动（更贴近“开箱即用”体验）
- `make backtest` / `make scan` / `make llm-scan`：常用策略工作流封装

---

## 9. 输出物与目录约定

建议统一写到 `runtime/`（Makefile 也按这个约定）：
- `runtime/report.json`：回测结果
- `runtime/review.md`：LLM 复盘输出
- `runtime/advice.md`：LLM 扫描建议输出
- `runtime/scan_charts/*.svg`：扫描图（每个标的一个）

---

## 10. 常见问题（定位思路）

- **AI 分析接口返回“未启用”**：检查 token 是否设置（`config.yaml` 的 `api.token` 或环境变量），以及是否启用了 `-ai` / `server.enable_ai`。  
- **回测/扫描没有任何标的**：`backtest.instruments.*` 为空时会尝试 `monitor.*`；如果两者都没配就会报错/空结果。  
- **回测/扫描提示 hf_ 相关**：外盘 `hf_` 目前实时可用，但缺少日线数据通道，回测/扫描会过滤。  
- **实时数据全是空**：大多是代码格式不对（如股票必须 `sh/sz` 前缀），或数据源接口不可达/被限流。
