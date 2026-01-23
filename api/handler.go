package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"stock/analyzer"
	"stock/cache"
	"stock/trading"
)

// Handler API处理器
type Handler struct {
	cache    *cache.Cache
	analyzer *analyzer.ClaudeAnalyzer
}

// NewHandler 创建处理器
func NewHandler(c *cache.Cache, a *analyzer.ClaudeAnalyzer) *Handler {
	return &Handler{cache: c, analyzer: a}
}

// GetStock 获取单只股票行情
func (h *Handler) GetStock(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "股票代码不能为空",
		})
		return
	}

	quote := h.cache.GetStock(code)
	if quote == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "未找到该股票数据",
			"code":  code,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"quote":          quote,
			"change":         quote.Change(),
			"change_percent": quote.ChangePercent(),
		},
	})
}

// GetAllStocks 获取所有股票行情
func (h *Handler) GetAllStocks(c *gin.Context) {
	quotes := h.cache.GetAllStocks()

	result := make([]gin.H, 0, len(quotes))
	for _, q := range quotes {
		result = append(result, gin.H{
			"quote":          q,
			"change":         q.Change(),
			"change_percent": q.ChangePercent(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"count": len(result),
		"data":  result,
	})
}

// GetFutures 获取单个期货行情
func (h *Handler) GetFutures(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "期货代码不能为空",
		})
		return
	}

	quote := h.cache.GetFutures(code)
	if quote == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "未找到该期货数据",
			"code":  code,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"quote":          quote,
			"change":         quote.Change(),
			"change_percent": quote.ChangePercent(),
		},
	})
}

// GetAllFutures 获取所有期货行情
func (h *Handler) GetAllFutures(c *gin.Context) {
	quotes := h.cache.GetAllFutures()

	result := make([]gin.H, 0, len(quotes))
	for _, q := range quotes {
		result = append(result, gin.H{
			"quote":          q,
			"change":         q.Change(),
			"change_percent": q.ChangePercent(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"count": len(result),
		"data":  result,
	})
}

// GetAnalysis 获取单个分析结果
func (h *Handler) GetAnalysis(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "代码不能为空",
		})
		return
	}

	if h.analyzer == nil || !h.analyzer.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI分析功能未启用",
		})
		return
	}

	analysis := h.analyzer.GetAnalysis(code)
	if analysis == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "未找到该标的的分析结果",
			"code":  code,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": analysis,
	})
}

// GetAllAnalysis 获取所有分析结果
func (h *Handler) GetAllAnalysis(c *gin.Context) {
	if h.analyzer == nil || !h.analyzer.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI分析功能未启用",
		})
		return
	}

	results := h.analyzer.GetAllAnalysis()

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"count": len(results),
		"data":  results,
	})
}

// GetStatus 获取服务状态
func (h *Handler) GetStatus(c *gin.Context) {
	isStockTrading := trading.IsStockTradingTime()
	isFuturesTrading := trading.IsFuturesTradingTime()

	aiEnabled := h.analyzer != nil && h.analyzer.IsEnabled()

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"stock_trading":   isStockTrading,
			"futures_trading": isFuturesTrading,
			"ai_enabled":      aiEnabled,
			"last_updated":    h.cache.LastUpdated(),
			"stock_count":     h.cache.StockCount(),
			"futures_count":   h.cache.FuturesCount(),
		},
	})
}
