<template>
  <div class="app">
    <header class="header">
      <h1>A股/期货实时行情</h1>
      <div class="status">
        <span :class="['status-dot', status.stock_trading ? 'active' : '']"></span>
        股票{{ status.stock_trading ? '交易中' : '休市' }}
        <span :class="['status-dot', status.futures_trading ? 'active' : '']" style="margin-left: 20px"></span>
        期货{{ status.futures_trading ? '交易中' : '休市' }}
        <span :class="['status-dot', status.ai_enabled ? 'ai-active' : '']" style="margin-left: 20px"></span>
        AI分析{{ status.ai_enabled ? '已启用' : '未启用' }}
        <span class="update-time">更新: {{ formatTime(status.last_updated) }}</span>
      </div>
    </header>

    <main class="main">
      <section class="section">
        <h2>股票行情</h2>
        <div class="table-container">
          <table class="quote-table">
            <thead>
              <tr>
                <th>代码</th>
                <th>名称</th>
                <th>最新价</th>
                <th>涨跌幅</th>
                <th>涨跌额</th>
                <th>今开</th>
                <th>最高</th>
                <th>最低</th>
                <th>成交量</th>
                <th>成交额</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in stocks" :key="item.quote.code">
                <td class="code">{{ item.quote.code }}</td>
                <td class="name">{{ item.quote.name }}</td>
                <td :class="getPriceClass(item.change)">{{ formatPrice(item.quote.price) }}</td>
                <td :class="getPriceClass(item.change_percent)">{{ formatPercent(item.change_percent) }}</td>
                <td :class="getPriceClass(item.change)">{{ formatChange(item.change) }}</td>
                <td>{{ formatPrice(item.quote.open) }}</td>
                <td>{{ formatPrice(item.quote.high) }}</td>
                <td>{{ formatPrice(item.quote.low) }}</td>
                <td>{{ formatVolume(item.quote.volume) }}</td>
                <td>{{ formatAmount(item.quote.amount) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="section">
        <h2>期货行情</h2>
        <div class="table-container">
          <table class="quote-table">
            <thead>
              <tr>
                <th>代码</th>
                <th>名称</th>
                <th>最新价</th>
                <th>涨跌幅</th>
                <th>涨跌额</th>
                <th>今开</th>
                <th>最高</th>
                <th>最低</th>
                <th>成交量</th>
                <th>持仓量</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in futures" :key="item.quote.code">
                <td class="code">{{ item.quote.code }}</td>
                <td class="name">{{ item.quote.name }}</td>
                <td :class="getPriceClass(item.change)">{{ formatPrice(item.quote.price) }}</td>
                <td :class="getPriceClass(item.change_percent)">{{ formatPercent(item.change_percent) }}</td>
                <td :class="getPriceClass(item.change)">{{ formatChange(item.change) }}</td>
                <td>{{ formatPrice(item.quote.open) }}</td>
                <td>{{ formatPrice(item.quote.high) }}</td>
                <td>{{ formatPrice(item.quote.low) }}</td>
                <td>{{ formatVolume(item.quote.volume) }}</td>
                <td>{{ formatVolume(item.quote.open_interest) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="section" v-if="analysis.length > 0">
        <h2>AI 走势分析</h2>
        <div class="analysis-grid">
          <div class="analysis-card" v-for="item in analysis" :key="item.code">
            <div class="analysis-header">
              <span class="analysis-name">{{ item.name }}</span>
              <span class="analysis-type">{{ item.type === 'stock' ? '股票' : '期货' }}</span>
            </div>
            <div class="analysis-content">{{ item.analysis }}</div>
            <div class="analysis-time">分析时间: {{ formatDateTime(item.updated_at) }}</div>
          </div>
        </div>
      </section>
    </main>

    <footer class="footer">
      数据来源: 新浪财经 | 行情刷新: 3秒 | AI分析: 每小时
    </footer>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const API_BASE = 'http://localhost:19527/api'

const stocks = ref([])
const futures = ref([])
const analysis = ref([])
const status = ref({
  stock_trading: false,
  futures_trading: false,
  ai_enabled: false,
  last_updated: null
})

let timer = null
let analysisTimer = null

// 获取股票数据
async function fetchStocks() {
  try {
    const res = await fetch(`${API_BASE}/stocks`)
    const data = await res.json()
    if (data.code === 0) {
      stocks.value = data.data || []
    }
  } catch (e) {
    console.error('获取股票数据失败:', e)
  }
}

// 获取期货数据
async function fetchFutures() {
  try {
    const res = await fetch(`${API_BASE}/futures`)
    const data = await res.json()
    if (data.code === 0) {
      futures.value = data.data || []
    }
  } catch (e) {
    console.error('获取期货数据失败:', e)
  }
}

// 获取AI分析
async function fetchAnalysis() {
  try {
    const res = await fetch(`${API_BASE}/analysis`)
    const data = await res.json()
    if (data.code === 0) {
      analysis.value = data.data || []
    }
  } catch (e) {
    console.error('获取AI分析失败:', e)
  }
}

// 获取状态
async function fetchStatus() {
  try {
    const res = await fetch(`${API_BASE}/status`)
    const data = await res.json()
    if (data.code === 0) {
      status.value = data.data
    }
  } catch (e) {
    console.error('获取状态失败:', e)
  }
}

// 刷新所有数据
async function refreshData() {
  await Promise.all([fetchStocks(), fetchFutures(), fetchStatus()])
}

// 格式化价格
function formatPrice(price) {
  if (!price) return '-'
  return price.toFixed(2)
}

// 格式化涨跌幅
function formatPercent(percent) {
  if (percent === undefined || percent === null) return '-'
  const sign = percent >= 0 ? '+' : ''
  return `${sign}${percent.toFixed(2)}%`
}

// 格式化涨跌额
function formatChange(change) {
  if (change === undefined || change === null) return '-'
  const sign = change >= 0 ? '+' : ''
  return `${sign}${change.toFixed(2)}`
}

// 格式化成交量
function formatVolume(vol) {
  if (!vol) return '-'
  if (vol >= 100000000) return (vol / 100000000).toFixed(2) + '亿'
  if (vol >= 10000) return (vol / 10000).toFixed(2) + '万'
  return vol.toString()
}

// 格式化成交额
function formatAmount(amount) {
  if (!amount) return '-'
  if (amount >= 100000000) return (amount / 100000000).toFixed(2) + '亿'
  if (amount >= 10000) return (amount / 10000).toFixed(2) + '万'
  return amount.toFixed(2)
}

// 格式化时间
function formatTime(timeStr) {
  if (!timeStr) return '-'
  const date = new Date(timeStr)
  return date.toLocaleTimeString('zh-CN')
}

// 格式化日期时间
function formatDateTime(timeStr) {
  if (!timeStr) return '-'
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN')
}

// 获取价格样式类
function getPriceClass(value) {
  if (value > 0) return 'up'
  if (value < 0) return 'down'
  return ''
}

onMounted(() => {
  refreshData()
  fetchAnalysis()
  timer = setInterval(refreshData, 3000)
  analysisTimer = setInterval(fetchAnalysis, 30000) // 30秒刷新一次分析
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (analysisTimer) clearInterval(analysisTimer)
})
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background: #1a1a2e;
  color: #eee;
}

.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.header {
  background: #16213e;
  padding: 20px;
  text-align: center;
  border-bottom: 1px solid #0f3460;
}

.header h1 {
  font-size: 24px;
  margin-bottom: 10px;
  color: #e94560;
}

.status {
  font-size: 14px;
  color: #888;
}

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #666;
  margin-right: 5px;
}

.status-dot.active {
  background: #00ff88;
  box-shadow: 0 0 10px #00ff88;
}

.status-dot.ai-active {
  background: #a855f7;
  box-shadow: 0 0 10px #a855f7;
}

.update-time {
  margin-left: 20px;
  color: #666;
}

.main {
  flex: 1;
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
  width: 100%;
}

.section {
  margin-bottom: 30px;
}

.section h2 {
  font-size: 18px;
  margin-bottom: 15px;
  color: #e94560;
  padding-left: 10px;
  border-left: 3px solid #e94560;
}

.table-container {
  overflow-x: auto;
}

.quote-table {
  width: 100%;
  border-collapse: collapse;
  background: #16213e;
  border-radius: 8px;
  overflow: hidden;
}

.quote-table th,
.quote-table td {
  padding: 12px 15px;
  text-align: right;
  border-bottom: 1px solid #0f3460;
}

.quote-table th {
  background: #0f3460;
  color: #888;
  font-weight: 500;
  font-size: 13px;
}

.quote-table td {
  font-size: 14px;
}

.quote-table td.code,
.quote-table td.name {
  text-align: left;
}

.quote-table td.code {
  color: #888;
  font-family: monospace;
}

.quote-table td.name {
  color: #fff;
  font-weight: 500;
}

.quote-table tr:hover {
  background: #1f2b4d;
}

.up {
  color: #ff4757 !important;
}

.down {
  color: #2ed573 !important;
}

/* AI分析样式 */
.analysis-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 20px;
}

.analysis-card {
  background: #16213e;
  border-radius: 8px;
  padding: 20px;
  border: 1px solid #0f3460;
}

.analysis-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
  padding-bottom: 10px;
  border-bottom: 1px solid #0f3460;
}

.analysis-name {
  font-size: 16px;
  font-weight: 600;
  color: #fff;
}

.analysis-type {
  font-size: 12px;
  padding: 2px 8px;
  background: #a855f7;
  color: #fff;
  border-radius: 4px;
}

.analysis-content {
  font-size: 14px;
  line-height: 1.8;
  color: #ccc;
  white-space: pre-wrap;
}

.analysis-time {
  margin-top: 15px;
  font-size: 12px;
  color: #666;
  text-align: right;
}

.footer {
  text-align: center;
  padding: 15px;
  color: #666;
  font-size: 12px;
  border-top: 1px solid #0f3460;
}
</style>
