# A股/期货实时行情服务

一个基于 Golang 的 A 股和期货市场实时行情服务，支持 REST API、终端实时显示和 AI 走势分析。

## 功能特性

- 实时拉取 A 股和期货行情数据（新浪财经接口）
- 自动判断交易时间，交易时段内每 3 秒刷新
- REST API 接口供外部查询
- 终端实时行情显示（CLI 模式）
- Vue 前端页面展示
- **AI 走势分析**（Claude API，每小时分析一次）
- 内存缓存，高性能响应

## 快速开始

### 方式一: 使用配置文件（推荐）

**1. 配置**

首次使用需要创建配置文件:

```bash
# 复制配置模板
cp config.yaml.example config.yaml

# 编辑配置文件,填入你的 API Token
# 可以使用任何文本编辑器打开 config.yaml
```

**2. 启动服务**

```bash
# Windows - 双击运行
start.bat

# 或命令行启动
stock.exe -config config.yaml

# Linux/Mac
./stock -config config.yaml
```

**3. 访问**

打开浏览器访问: http://localhost:19527

### 方式二: 使用启动脚本

**Windows PowerShell：**
```powershell
# 基本启动（仅行情，无AI分析）
.\start.ps1

# 启用 AI 分析（使用代理）
.\start.ps1 -Token "sk-xxx" -BaseUrl "http://llm.proxy.com" -Model "claude-sonnet-4-5-20250929"

# 终端实时行情模式
.\start.ps1 -Cli
```

**Linux/Mac：**
```bash
# 添加执行权限
chmod +x start.sh

# 基本启动
./start.sh

# 启用 AI 分析（使用代理）
./start.sh -t "sk-xxx" -b "http://llm.proxy.com" -m "claude-sonnet-4-5-20250929"

# 终端实时行情模式
./start.sh -c

# 查看帮助
./start.sh -h
```

### 启动脚本参数说明

| 参数 | PowerShell | Bash | 环境变量 | 说明 |
|------|------------|------|----------|------|
| Token | `-Token` | `-t, --token` | `ANTHROPIC_AUTH_TOKEN` | API Token |
| Base URL | `-BaseUrl` | `-b, --base` | `ANTHROPIC_BASE_URL` | API地址 |
| Model | `-Model` | `-m, --model` | `ANTHROPIC_MODEL` | 模型名称 |
| CLI模式 | `-Cli` | `-c, --cli` | - | 终端显示模式 |

### 手动编译运行

```bash
# 编译
go build -o stock.exe .

# 运行
./stock.exe

# 终端模式
./stock.exe -cli
```

服务启动后访问 http://localhost:19527

### 前端页面

```bash
cd web
npm install
npm run dev
```
访问 http://localhost:5173

## API 接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/stocks` | GET | 查询所有股票行情 |
| `/api/stock/:code` | GET | 查询单只股票（如 `sh600000`） |
| `/api/futures` | GET | 查询所有期货行情 |
| `/api/futures/:code` | GET | 查询单个期货（如 `nf_AU0`） |
| `/api/analysis` | GET | 查询所有 AI 分析结果 |
| `/api/analysis/:code` | GET | 查询单个 AI 分析结果 |
| `/api/status` | GET | 服务状态 |

## AI 分析功能

- 使用 Claude API 分析最近 7 个交易日的 K 线走势
- 交易时间内每小时自动分析一次
- 分析内容包括：近期趋势、成交量变化、短期走势预判
- 需要设置环境变量 `ANTHROPIC_API_KEY`
- 如 API 无法直接访问，可设置 `ANTHROPIC_API_BASE` 使用代理服务

## 配置

编辑 `config/config.go` 修改监控标的：

```go
Stocks: []string{
    "sz002415", // 海康威视
    "sh601611", // 中国核建
    "sh513130", // 恒生科技ETF
},
Futures: []string{
    "nf_I0",  // 铁矿石主连
    "nf_B0",  // 豆二主连
    "nf_MA0", // 甲醇主连
    "nf_UR0", // 尿素主连
    "nf_EB0", // 苯乙烯主连
},
```

### 代码格式

| 市场 | 格式 | 示例 |
|------|------|------|
| 上海股票 | sh + 代码 | sh600000 |
| 深圳股票 | sz + 代码 | sz000001 |
| 商品期货 | nf_ + 品种 + 0 | nf_AU0（黄金主连） |
| 股指期货 | nf_ + 品种 + 0 | nf_IF0（沪深300主连） |

## 项目结构

```
stock/
├── main.go              # 程序入口
├── start.ps1            # Windows 启动脚本
├── start.sh             # Linux/Mac 启动脚本
├── config/              # 配置管理
├── model/               # 数据模型
├── fetcher/             # 数据拉取（股票/期货/K线）
├── analyzer/            # AI 分析模块
├── cache/               # 内存缓存
├── trading/             # 交易时间判断
├── api/                 # REST API
└── web/                 # Vue 前端
```

## 交易时间

- **A股**: 9:30-11:30, 13:00-15:00（周一至周五）
- **期货日盘**: 9:00-10:15, 10:30-11:30, 13:30-15:00
- **期货夜盘**: 21:00-23:00（部分品种至次日凌晨）

## 数据来源

- 实时行情：新浪财经（免费）
- 历史K线：东方财富（股票）、新浪财经（期货）
- AI分析：Anthropic Claude API

## 日线回测（实验）

项目内置一个“收盘确认 -> 次日开盘成交”的日线回测入口，用于验证基于蔡森“破底翻 / 假突破”逻辑的量价结构策略（参数化实现，便于后续接入本地大模型自动生成配置）。

**1. 准备回测配置**

```bash
cp backtest.yaml.example backtest.yaml
```

**2. 运行回测**

```bash
./stock -backtest -bt-config backtest.yaml
```

也可以使用形态策略示例（M头/W底/头肩顶/头肩底/三角形/波段等幅）：

```bash
cp patterns.yaml.example patterns.yaml
./stock -backtest -bt-config patterns.yaml -bt-out report.json
```

**3. 输出到文件**

```bash
./stock -backtest -bt-config backtest.yaml -bt-out report.json
```

## 最新信号扫描（实验）

扫描“最新一根日K收盘是否产生信号”（信号在收盘确认，**下一交易日开盘执行**），同时输出当前持仓状态：

```bash
./stock -scan -bt-config backtest.yaml
```

最近一年（自动覆盖日期窗口）：

```bash
./stock -scan -bt-config backtest.yaml -scan-days 365
```

说明：`-scan-days 365` 按“自然日窗口”计算（含周末/节假日），内部会拉取足够多的日K后再按 `start/end` 过滤，所以实际有效K线条数通常会少于 365。

默认会把 `backtest.yaml` 的标的列表与当前目录的 `config.yaml`（监控 stocks/futures）合并后一起扫描；如需指定配置文件路径，可加 `-config path/to/config.yaml`。

仅输出有信号的标的：

```bash
./stock -scan -bt-config backtest.yaml -scan-only-signal
```

输出趋势上下文图（SVG，包含K线 + 关键画线，如支撑/压力、止损/目标等）：

```bash
./stock -scan -bt-config backtest.yaml -scan-days 365 -scan-chart -scan-chart-bars 220
```

图默认输出到 `scan_charts/`，每个标的一个 `SYMBOL.svg`（可用浏览器直接打开）。

JSON 输出：

```bash
./stock -scan -bt-config backtest.yaml -scan-json
```

## 扫描建议（Ollama）

用本地 Ollama 把扫描结果变成可读的执行清单（Markdown）：

```bash
./stock -llm-scan -bt-config backtest.yaml -llm-url http://localhost:11434 -llm-model qwen2.5-coder:14b > advice.md
```

如果首次加载模型较慢导致超时，可调大超时时间：

```bash
./stock -llm-scan -bt-config backtest.yaml -scan-days 365 -llm-timeout 10m > advice.md
```

建议配合输出 SVG 图（LLM 会在建议里引用 `chart_path`，方便你打开看趋势结构）：

```bash
./stock -llm-scan -bt-config backtest.yaml -scan-days 365 -scan-chart -llm-timeout 10m -llm-out advice.md
```

最近一年（自动覆盖日期窗口）：

```bash
./stock -llm-scan -bt-config backtest.yaml -scan-days 365 > advice.md
```

仅输出有信号的标的：

```bash
./stock -llm-scan -bt-config backtest.yaml -llm-scan-only-signal > advice.md
```

## 本地大模型（Ollama）辅助

项目支持用本地 Ollama + `qwen2.5-coder:14b` 做两件事：

1) 自然语言策略 → 生成并校验 `backtest.yaml`（严格 JSON Schema，避免乱填字段）  
2) `report.json` → 复盘总结（Markdown）

### 1. 自然语言生成 backtest.yaml

```bash
echo "用日线做蔡森破底翻+假突破，回测 nf_I0 和 sh600000，2018-01-01 到 2025-12-31" | \
  ./stock -llm-gen-bt -llm-url http://localhost:11434 -llm-model qwen2.5-coder:14b -llm-out backtest.yaml
```

### 2. 回测报告复盘

先运行回测输出 `report.json`：

```bash
./stock -backtest -bt-config backtest.yaml -bt-out report.json
```

再让 LLM 生成复盘：

```bash
./stock -llm-analyze report.json -llm-bt-config backtest.yaml > review.md
```

## 配置文件说明

配置文件 `config.yaml` 支持以下配置项:

### API 配置
```yaml
api:
  token: ""                              # Anthropic API Token (必填)
  base_url: "https://api.anthropic.com"  # API 地址
  model: "claude-sonnet-4-5-20250929"    # 模型名称
```

### 监控配置
```yaml
monitor:
  stocks:
    - sz002415   # 海康威视
    - sh513130   # 恒生科技ETF

  futures:
    - nf_I0      # 铁矿石主连
    - nf_B0      # 豆二主连
```

### 服务配置
```yaml
server:
  port: 19527           # HTTP 服务端口
  enable_ai: true       # 是否启用 AI 分析
  sync_interval: 5      # 数据同步间隔(秒)
```

**注意**: 配置文件优先级高于环境变量。环境变量仍然支持,可用于覆盖配置文件的设置。

## 打包构建

### 构建发布包

使用构建脚本自动打包 Windows 版本:

```bash
# 执行构建脚本
./build.sh

# 构建产物在 dist/ 目录
# - dist/stock-行情服务-v1.0.0-windows-amd64.zip
# - dist/stock-行情服务-v1.0.0-windows-amd64.tar.gz
```

构建脚本会自动:
1. 构建前端 (npm install && npm run build)
2. 编译 Go 程序 (交叉编译 Windows 版本)
3. 打包配置文件模板、启动脚本和文档
4. 生成 ZIP 和 tar.gz 压缩包

### 自定义版本号

```bash
# 指定版本号
VERSION=2.0.0 ./build.sh
```

### 发布包内容

解压后的目录结构:
```
stock-行情服务-v1.0.0-windows-amd64/
├── stock.exe              # 主程序(包含前端)
├── config.yaml.example    # 配置文件模板
├── start.bat              # Windows 启动脚本
└── README.md              # 使用文档
```

## 常见问题

### Q: 如何获取 API Token?
A: 访问 https://console.anthropic.com/settings/keys 创建 API Key

### Q: 如何修改监控的股票?
A: 编辑 `config.yaml` 中的 `monitor.stocks` 列表

### Q: 如何关闭 AI 分析?
A: 在 `config.yaml` 中设置 `server.enable_ai: false`

### Q: 配置文件和环境变量哪个优先级更高?
A: 环境变量优先级更高,可以用来临时覆盖配置文件的设置

### Q: 前端页面无法访问?
A: 确保使用构建脚本打包,前端资源已嵌入到 stock.exe 中。直接编译需要先运行 `cd web && npm run build`

### Q: 如何在 Linux 上运行?
A: 修改 `build.sh` 中的 `PLATFORMS` 变量为 `linux/amd64`,重新构建即可
