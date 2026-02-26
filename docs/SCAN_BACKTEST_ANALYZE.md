# 扫描 / 回测 / 年度分析使用说明

本仓库的策略工具链都基于“日线 bar（daily bars）”，并统一采用：

> **收盘确认产生信号 → 下一根日 K 的开盘价成交（next-day open execution）**

这条假设会影响你对 `scan` 的“信号时点”、对 `backtest` 的“成交价/滑点”、对 `analyze` 的“最新动作建议”的解读。

---

## 1. 输入配置：`config.yaml` 与 `backtest.yaml`

### 1.1 `config.yaml`（服务/CLI监控清单）
- 模板：`config.yaml.example`
- 主要用于：
  - `stockd` 实时行情监控列表（`monitor.stocks/futures`）
  - `stockctl -analyze` 的标的来源（股票 + 国内期货）

### 1.2 `backtest.yaml`（回测/扫描策略参数）
- 模板：`backtest.yaml.example`
- 主要用于：
  - `stockctl -backtest`：回测窗口/成本/仓位/策略参数
  - `stockctl -scan`：扫描策略参数（并可叠加 `-scan-days` 覆盖窗口）
  - `stockctl -analyze`：读取策略参数（目前要求 `strategy.type=tsai_sen`）

### 1.3 合并逻辑（避免踩坑）
- `scan/analyze` 会把 `backtest.yaml` 与 `config.yaml` 的标的合并：
  - 若 `config.yaml` 存在，会把其中股票与**国内期货**补充进来（实现见 `internal/stockctl/scan_config.go:11`）
  - 外盘期货 `hf_` 仅用于实时行情；回测/扫描/分析会跳过（缺少日线通道）

---

## 2. 扫描（`-scan`）：看“最新一根日 K”有没有新信号

### 2.1 常用命令
- 表格输出（默认）：`./stock -scan -bt-config backtest.yaml`
- 只看有信号/有错误的标的：`./stock -scan -scan-only-signal -bt-config backtest.yaml`
- 覆盖窗口：`./stock -scan -scan-days 365 -bt-config backtest.yaml`
- 生成 SVG 图（带支撑/压力/止损/目标/入场画线）：  
  `./stock -scan -scan-chart -scan-chart-dir runtime/scan_charts -scan-chart-bars 220 -bt-config backtest.yaml`
- JSON 输出（便于脚本/LLM）：`./stock -scan -scan-json -bt-config backtest.yaml`

### 2.2 输出字段怎么理解
扫描核心输出结构见 `backtest/scan.go:1`（`ScanResult`），常见字段：
- `last_date/last_close`：最新 bar 的日期与收盘价（信号“确认”的那根）
- `next_action`：下一步动作（`buy/sell/short/cover`），**含义是：下一交易日开盘执行**
- `position_side/position_qty`：扫描模型下的当前持仓状态（按历史信号模拟得出）
- `support/resistance`：策略在最新 bar 下识别到的关键位（用于上下文/画线）
- `suggested_stop/suggested_target`：策略给出的“计划止损/目标”（用于执行参考）
- `chart_path`：如果开了 `-scan-chart`，会写入对应 SVG 路径

### 2.3 “为什么明明有信号但我今天不能买？”
因为信号在**收盘确认**，执行在**下一交易日开盘**。如果你在盘中运行扫描，看到的仍是“上一根日 K”的信号结论。

---

## 3. 回测（`-backtest`）：按日线模拟交易并产出 `report.json`

### 3.1 常用命令
- 输出到 stdout：`./stock -backtest -bt-config backtest.yaml`
- 输出到文件：`./stock -backtest -bt-config backtest.yaml -bt-out runtime/report.json`

Makefile 快捷入口：
- `make backtest`（默认输出 `runtime/report.json`，并自动创建目录）

### 3.2 指标字段（`backtest.Result`）
结果结构定义见 `backtest/engine.go:16`：
- `final_equity`：期末权益（现金 + 持仓按收盘价估值）
- `max_drawdown_pct`：最大回撤（按权益曲线计算）
- `win_rate_pct` / `total_trades`：胜率与交易数
- `trades`：每笔交易的进出场时间/价格、收益、原因

### 3.3 常见“回测跑不出结果”的原因
- 日线 bars 不足（引擎会要求至少 ~50 根，见 `backtest/engine.go:89`）
- 标的代码格式不正确（股票必须 `sh/sz` 前缀；期货建议 `nf_` 或简写如 `pp2605`）
- 数据源接口暂不可用/被限流（东方财富/新浪偶发）

---

## 4. 年度分析（`-analyze`）：生成可浏览的静态报告（JSON/CSV/SVG/HTML）

### 4.1 常用命令
- 默认输出到 `runtime/analysis`：  
  `./stock -analyze -config config.yaml -bt-config backtest.yaml`
- 设窗口（自然日）：  
  `./stock -analyze -config config.yaml -bt-config backtest.yaml -analyze-window-days 365`
- 设窗口（bars）：  
  `./stock -analyze -config config.yaml -bt-config backtest.yaml -analyze-bars 252`

Makefile 快捷入口：
- `make analyze`（输出到 `runtime/analysis`）

### 4.2 输出物清单（`runtime/analysis/`）
由 `internal/stockctl/analyze_cmd.go` 生成：
- `analysis.json`：机器可读的全量结果
- `analysis.csv`：摘要表（每个标的一行）
- `trades.csv`：明细成交记录（从年度回测结果展开）
- `charts/*.svg`：每个标的一张“价格 + 成交量”图（含关键线/信号点）
- `index.html`：单文件报告页（内嵌 JSON，可离线打开）

### 4.3 在服务端里查看（推荐）
`stockd` 会把本地 `runtime/analysis/` 挂到：
- `http://localhost:19527/analysis/`

如果你运行了 `make analyze` 但页面提示找不到报告，检查是否生成了 `runtime/analysis/index.html`。

---

## 5. 一句话工作流（建议）
- 每天收盘后：`make scan-only SCAN_DAYS=365 SCAN_CHART=1`
- 每周复盘：`make backtest` →（可选）`./stock -llm-analyze runtime/report.json -llm-bt-config backtest.yaml > runtime/review.md`
- 月度检查：`make analyze` → 打开 `http://localhost:19527/analysis/`

