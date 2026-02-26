package stockctl

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func writeAnalysisHTML(outDir string, report analysisReport) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}

	// Use a static HTML with embedded JSON to avoid file:// fetch / CORS issues.
	var b bytes.Buffer
	b.WriteString("<!doctype html>\n")
	b.WriteString("<html lang=\"zh-CN\">\n<head>\n")
	b.WriteString("  <meta charset=\"utf-8\" />\n")
	b.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n")
	b.WriteString("  <title>一年量价分析报告</title>\n")
	b.WriteString("  <style>\n")
	b.WriteString(cssAnalysisReport())
	b.WriteString("  </style>\n")
	b.WriteString("</head>\n<body>\n")

	b.WriteString("  <header class=\"hdr\">\n")
	b.WriteString("    <div class=\"title\">一年量价分析报告</div>\n")
	b.WriteString("    <div class=\"meta\" id=\"meta\"></div>\n")
	b.WriteString("  </header>\n")

	b.WriteString("  <section class=\"controls\">\n")
	b.WriteString("    <input id=\"q\" class=\"q\" placeholder=\"搜索：代码/名称 (例如 sh600000 / 海康)\" />\n")
	b.WriteString("    <select id=\"instrument\" class=\"sel\">\n")
	b.WriteString("      <option value=\"\">全部市场</option>\n")
	b.WriteString("      <option value=\"stock\">股票</option>\n")
	b.WriteString("      <option value=\"futures\">期货</option>\n")
	b.WriteString("    </select>\n")
	b.WriteString("    <select id=\"signal\" class=\"sel\">\n")
	b.WriteString("      <option value=\"\">全部信号</option>\n")
	b.WriteString("      <option value=\"buy\">BUY</option>\n")
	b.WriteString("      <option value=\"sell\">SELL</option>\n")
	b.WriteString("      <option value=\"short\">SHORT</option>\n")
	b.WriteString("      <option value=\"cover\">COVER</option>\n")
	b.WriteString("      <option value=\"none\">无信号</option>\n")
	b.WriteString("      <option value=\"error\">有错误</option>\n")
	b.WriteString("    </select>\n")
	b.WriteString("    <button id=\"reset\" class=\"btn\" type=\"button\">重置</button>\n")
	b.WriteString("  </section>\n")

	b.WriteString("  <main class=\"grid\">\n")
	b.WriteString("    <section class=\"card\">\n")
	b.WriteString("      <div class=\"card-hd\">摘要</div>\n")
	b.WriteString("      <div class=\"table-wrap\">\n")
	b.WriteString("        <table class=\"tbl\" id=\"tbl\">\n")
	b.WriteString("          <thead><tr>\n")
	b.WriteString("            <th>代码</th><th>名称</th><th>市场</th><th>窗口</th>\n")
	b.WriteString("            <th>信号</th><th>持仓</th><th>止损</th><th>目标</th>\n")
	b.WriteString("            <th>量比</th><th>胜率</th><th>回撤</th><th>交易数</th>\n")
	b.WriteString("            <th>图</th><th>错误</th>\n")
	b.WriteString("          </tr></thead>\n")
	b.WriteString("          <tbody id=\"tbody\"></tbody>\n")
	b.WriteString("        </table>\n")
	b.WriteString("      </div>\n")
	b.WriteString("    </section>\n")

	b.WriteString("    <section class=\"card\" id=\"detail\">\n")
	b.WriteString("      <div class=\"card-hd\">详情</div>\n")
	b.WriteString("      <div class=\"detail-empty\">点击左侧某行查看详情</div>\n")
	b.WriteString("    </section>\n")
	b.WriteString("  </main>\n")

	b.WriteString("  <script type=\"application/json\" id=\"analysis-data\">")
	// Prevent </script> from terminating the tag (keep JSON otherwise raw so JSON.parse works).
	b.WriteString(strings.ReplaceAll(string(raw), "</", "<\\/"))
	b.WriteString("</script>\n")
	b.WriteString("  <script>\n")
	b.WriteString(jsAnalysisReport())
	b.WriteString("  </script>\n")

	b.WriteString("</body>\n</html>\n")

	path := filepath.Join(outDir, "index.html")
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func cssAnalysisReport() string {
	return `
:root {
  --bg: #0b1220;
  --panel: rgba(255,255,255,0.06);
  --panel2: rgba(255,255,255,0.08);
  --txt: rgba(255,255,255,0.88);
  --muted: rgba(255,255,255,0.62);
  --grid: rgba(255,255,255,0.10);
  --good: #22c55e;
  --bad: #ef4444;
  --warn: #f59e0b;
  --accent: #38bdf8;
  --mono: ui-monospace, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  --sans: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, "Noto Sans", "Helvetica Neue", Arial, "Apple Color Emoji", "Segoe UI Emoji";
}
* { box-sizing: border-box; }
body { margin:0; background: var(--bg); color: var(--txt); font-family: var(--sans); }
.hdr { padding: 18px 18px 10px 18px; border-bottom: 1px solid var(--grid); display:flex; align-items: baseline; gap: 12px; }
.title { font-size: 18px; font-weight: 700; letter-spacing: .2px; }
.meta { font-family: var(--mono); color: var(--muted); font-size: 12px; }
.controls { padding: 10px 18px; display:flex; gap: 10px; align-items: center; border-bottom: 1px solid var(--grid); flex-wrap: wrap; }
.q { width: 320px; max-width: 100%; padding: 9px 10px; border-radius: 10px; border: 1px solid var(--grid); background: rgba(255,255,255,0.04); color: var(--txt); outline: none; }
.sel { padding: 9px 10px; border-radius: 10px; border: 1px solid var(--grid); background: rgba(255,255,255,0.04); color: var(--txt); outline: none; }
.btn { padding: 9px 12px; border-radius: 10px; border: 1px solid var(--grid); background: rgba(255,255,255,0.06); color: var(--txt); cursor: pointer; }
.btn:hover { background: rgba(255,255,255,0.10); }
.grid { padding: 14px 18px 20px 18px; display:grid; grid-template-columns: 1.25fr 0.75fr; gap: 14px; }
@media (max-width: 1050px) { .grid { grid-template-columns: 1fr; } }
.card { background: var(--panel); border: 1px solid var(--grid); border-radius: 14px; overflow: hidden; }
.card-hd { padding: 10px 12px; font-size: 13px; font-weight: 700; color: var(--muted); border-bottom: 1px solid var(--grid); }
.table-wrap { overflow:auto; max-height: calc(100vh - 220px); }
.tbl { width: 100%; border-collapse: collapse; font-family: var(--mono); font-size: 12px; }
.tbl thead th { position: sticky; top: 0; background: rgba(10,16,30,0.92); border-bottom: 1px solid var(--grid); color: var(--muted); text-align: left; padding: 8px 10px; white-space: nowrap; }
.tbl tbody td { border-bottom: 1px solid rgba(255,255,255,0.06); padding: 7px 10px; white-space: nowrap; }
.tbl tbody tr { cursor: pointer; }
.tbl tbody tr:hover { background: rgba(255,255,255,0.05); }
.pill { display:inline-block; padding: 2px 8px; border-radius: 999px; border: 1px solid rgba(255,255,255,0.12); font-size: 11px; }
.pill.good { color: var(--good); border-color: rgba(34,197,94,0.35); background: rgba(34,197,94,0.08); }
.pill.bad { color: var(--bad); border-color: rgba(239,68,68,0.35); background: rgba(239,68,68,0.08); }
.pill.warn { color: var(--warn); border-color: rgba(245,158,11,0.35); background: rgba(245,158,11,0.08); }
.pill.muted { color: var(--muted); border-color: rgba(255,255,255,0.10); background: rgba(255,255,255,0.04); }
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.detail-empty { padding: 14px 12px; color: var(--muted); font-family: var(--mono); font-size: 12px; }
.detail { padding: 12px; display:flex; flex-direction: column; gap: 10px; }
.kv { display:grid; grid-template-columns: 120px 1fr; gap: 6px 10px; font-family: var(--mono); font-size: 12px; }
.k { color: var(--muted); }
.v { color: var(--txt); overflow-wrap: anywhere; }
.img { width: 100%; border: 1px solid rgba(255,255,255,0.10); border-radius: 12px; background: rgba(0,0,0,0.25); }
.err { color: var(--bad); font-family: var(--mono); font-size: 12px; white-space: pre-wrap; }
`
}

func jsAnalysisReport() string {
	return `
function esc(s) {
  return String(s === undefined || s === null ? "" : s)
    .replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;");
}
function n2(x) {
  const v = Number(x);
  if (!Number.isFinite(v)) return "";
  return v.toFixed(2);
}
function relChart(absPath) {
  const p = String(absPath || "");
  const idx = p.lastIndexOf("/charts/");
  if (idx >= 0) return "charts/" + p.substring(idx + "/charts/".length);
  const idx2 = p.lastIndexOf("\\\\charts\\\\");
  if (idx2 >= 0) return "charts/" + p.substring(idx2 + "\\\\charts\\\\".length);
  const base = p.split(/[\\\\/]/).pop();
  return base ? ("charts/" + base) : "";
}
function pillSpan(sig, hasErr) {
  const span = document.createElement("span");
  span.className = "pill muted";
  if (hasErr) { span.className = "pill bad"; span.textContent = "ERROR"; return span; }
  if (!sig) { span.textContent = "-"; return span; }
  const s = String(sig).toLowerCase();
  if (s === "buy") { span.className = "pill good"; span.textContent = "BUY"; return span; }
  if (s === "sell") { span.className = "pill warn"; span.textContent = "SELL"; return span; }
  if (s === "short") { span.className = "pill bad"; span.textContent = "SHORT"; return span; }
  if (s === "cover") { span.className = "pill warn"; span.textContent = "COVER"; return span; }
  span.textContent = String(sig);
  return span;
}

const dataEl = document.getElementById("analysis-data");
let blob = dataEl.textContent || "{}";
// Backward compatibility: older generated reports might have HTML-escaped JSON (e.g. &#34;).
if (blob.includes("&quot;") || blob.includes("&#34;") || blob.includes("&amp;") || blob.includes("&lt;") || blob.includes("&gt;")) {
  const tmp = document.createElement("textarea");
  tmp.innerHTML = blob;
  blob = tmp.value || blob;
}
const rep = JSON.parse(blob || "{}");
const all = Array.isArray(rep.results) ? rep.results : [];

const meta = document.getElementById("meta");
const w = rep.window || {};
let win = "";
if (w.mode === "bars") {
  win = "窗口: 最近 " + (w.bars || "") + " 根日K";
} else {
  win = "窗口: " + (w.start_date || "") + " ~ " + (w.end_date || "") + " (最近 " + (w.days || "") + " 天)";
}
meta.textContent = "生成时间: " + (rep.generated_at || "") + "  |  " + win + "  |  标的数: " + all.length;

const qEl = document.getElementById("q");
const instEl = document.getElementById("instrument");
const sigEl = document.getElementById("signal");
const resetEl = document.getElementById("reset");
const tbody = document.getElementById("tbody");
const detail = document.getElementById("detail");

function getSignalKey(r) {
  if (Array.isArray(r.errors) && r.errors.length) return "error";
  const latest = r.latest || {};
  const s = String(latest.next_action || "").toLowerCase();
  return s || "none";
}

function filterRows() {
  const q = (qEl.value || "").trim().toLowerCase();
  const inst = (instEl.value || "").trim();
  const sig = (sigEl.value || "").trim();

  return all.filter(function(r) {
    if (inst && r.instrument !== inst) return false;
    const sk = getSignalKey(r);
    if (sig) {
      if (sig === "none" && sk !== "none") return false;
      if (sig === "error" && sk !== "error") return false;
      if (sig !== "none" && sig !== "error" && sk !== sig) return false;
    }
    if (!q) return true;
    const hay = (String(r.symbol || "") + " " + String(r.name || "")).toLowerCase();
    return hay.includes(q);
  });
}

function clear(el) { while (el.firstChild) el.removeChild(el.firstChild); }
function tdText(tr, text) { const td = document.createElement("td"); td.textContent = text || ""; tr.appendChild(td); return td; }

function renderTable(rows) {
  clear(tbody);
  rows.forEach(function(r) {
    const latest = r.latest || {};
    const hasErr = Array.isArray(r.errors) && r.errors.length;
    const tr = document.createElement("tr");
    tr.setAttribute("data-symbol", String(r.symbol || ""));
    tdText(tr, r.symbol);
    tdText(tr, r.name || "");
    tdText(tr, r.instrument || "");
    const winTxt = (r.start_date && r.end_date) ? (r.start_date + "~" + r.end_date) : "";
    tdText(tr, winTxt);

    const tdSig = document.createElement("td");
    tdSig.appendChild(pillSpan(latest.next_action || "", hasErr));
    tr.appendChild(tdSig);

    tdText(tr, latest.position_side || "");
    tdText(tr, latest.suggested_stop ? n2(latest.suggested_stop) : "");
    tdText(tr, latest.suggested_target ? n2(latest.suggested_target) : "");
    tdText(tr, r.last_volume_ratio ? n2(r.last_volume_ratio) : "");

    const yr = r.year_stats || null;
    tdText(tr, yr ? n2(yr.win_rate_pct) : "");
    tdText(tr, yr ? n2(yr.max_dd_pct) : "");
    tdText(tr, yr ? String(yr.total_trades || "") : "");

    const tdChart = document.createElement("td");
    if (r.chart_path) {
      const a = document.createElement("a");
      a.href = relChart(r.chart_path);
      a.target = "_blank";
      a.textContent = "SVG";
      tdChart.appendChild(a);
    }
    tr.appendChild(tdChart);

    const tdErr = document.createElement("td");
    if (hasErr) {
      tdErr.title = r.errors.join(" | ");
      tdErr.textContent = "ERR";
    }
    tr.appendChild(tdErr);

    tbody.appendChild(tr);
  });
}

function renderDetail(r) {
  const latest = r.latest || {};
  const hasErr = Array.isArray(r.errors) && r.errors.length;
  const yr = r.year_stats || null;
  const chart = r.chart_path ? relChart(r.chart_path) : "";

  detail.innerHTML = '<div class="card-hd">详情</div>';
  const wrap = document.createElement("div");
  wrap.className = "detail";

  const kv = document.createElement("div");
  kv.className = "kv";
  function kvRow(k, v) {
    const kk = document.createElement("div"); kk.className = "k"; kk.textContent = k;
    const vv = document.createElement("div"); vv.className = "v"; vv.textContent = v;
    kv.appendChild(kk); kv.appendChild(vv);
  }
  kvRow("名称/代码", (r.name || "") + " " + (r.symbol || ""));
  kvRow("市场", r.instrument || "");
  kvRow("窗口", (r.start_date || "") + " ~ " + (r.end_date || "") + " (" + String(r.bars_count || "") + " bars)");
  kvRow("支撑/压力", n2(r.support) + " / " + n2(r.resistance));
  kvRow("最新信号", (hasErr ? "ERROR" : String(latest.next_action || "-")) + " " + String(latest.reason || ""));
  kvRow("持仓", String(latest.position_side || ""));
  kvRow("入场/止损/目标", (latest.entry_price ? n2(latest.entry_price) : "") + " / " + (latest.suggested_stop ? n2(latest.suggested_stop) : "") + " / " + (latest.suggested_target ? n2(latest.suggested_target) : ""));
  kvRow("量能", "MA" + String(r.volume_ma_n || "") + "=" + n2(r.last_volume_ma) + "  last=" + String(r.last_volume || "") + "  ratio=" + n2(r.last_volume_ratio));
  kvRow("一年统计", yr ? ("win=" + n2(yr.win_rate_pct) + "%  dd=" + n2(yr.max_dd_pct) + "%  trades=" + (yr.total_trades || 0) + "  equity=" + n2(yr.final_equity)) : "-");

  wrap.appendChild(kv);

  if (chart) {
    const a = document.createElement("a");
    a.href = chart; a.target = "_blank"; a.textContent = "打开图表";
    wrap.appendChild(a);
    const img = document.createElement("img");
    img.className = "img";
    img.src = chart;
    img.alt = "chart";
    wrap.appendChild(img);
  } else {
    const empty = document.createElement("div");
    empty.className = "detail-empty";
    empty.textContent = "没有图表（可能数据不足或生成失败）";
    wrap.appendChild(empty);
  }

  if (hasErr) {
    const err = document.createElement("div");
    err.className = "err";
    err.textContent = r.errors.join("\\n");
    wrap.appendChild(err);
  }

  detail.appendChild(wrap);
}

function resetDetail() {
  detail.innerHTML = '<div class="card-hd">详情</div><div class="detail-empty">点击左侧某行查看详情</div>';
}

function wire() {
  renderTable(filterRows());

  qEl.addEventListener("input", function() { renderTable(filterRows()); });
  instEl.addEventListener("change", function() { renderTable(filterRows()); });
  sigEl.addEventListener("change", function() { renderTable(filterRows()); });
  resetEl.addEventListener("click", function() {
    qEl.value = "";
    instEl.value = "";
    sigEl.value = "";
    renderTable(filterRows());
    resetDetail();
  });

  tbody.addEventListener("click", function(e) {
    const tr = e.target && e.target.closest ? e.target.closest("tr") : null;
    if (!tr) return;
    const sym = tr.getAttribute("data-symbol");
    const r = all.find(function(x) { return x.symbol === sym; });
    if (r) renderDetail(r);
  });
}

resetDetail();
wire();
`
}
