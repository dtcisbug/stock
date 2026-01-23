package api

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"stock/analyzer"
	"stock/cache"
)

// Server HTTP服务器
type Server struct {
	engine    *gin.Engine
	server    *http.Server
	cache     *cache.Cache
	analyzer  *analyzer.ClaudeAnalyzer
	staticFS  fs.FS
}

// NewServer 创建服务器
func NewServer(c *cache.Cache, port int, a *analyzer.ClaudeAnalyzer, staticFS fs.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())
	engine.Use(loggerMiddleware())

	s := &Server{
		engine:   engine,
		cache:    c,
		analyzer: a,
		staticFS: staticFS,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: engine,
		},
	}

	s.setupRoutes()
	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	handler := NewHandler(s.cache, s.analyzer)

	api := s.engine.Group("/api")
	{
		// 股票相关
		api.GET("/stock/:code", handler.GetStock)
		api.GET("/stocks", handler.GetAllStocks)

		// 期货相关
		api.GET("/futures/:code", handler.GetFutures)
		api.GET("/futures", handler.GetAllFutures)

		// AI分析
		api.GET("/analysis", handler.GetAllAnalysis)
		api.GET("/analysis/:code", handler.GetAnalysis)

		// 服务状态
		api.GET("/status", handler.GetStatus)
	}

	// 健康检查
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 静态文件服务 (嵌入的前端)
	if s.staticFS != nil {
		s.engine.StaticFS("/static", http.FS(s.staticFS))
		// 首页路由
		s.engine.GET("/", func(c *gin.Context) {
			data, err := fs.ReadFile(s.staticFS, "index.html")
			if err != nil {
				c.String(http.StatusNotFound, "index.html not found")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		})
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	log.Printf("[API] 服务启动在 http://localhost%s\n", s.server.Addr)
	log.Println("[API] 可用接口:")
	log.Println("  GET /api/stock/:code   - 查询单只股票")
	log.Println("  GET /api/stocks        - 查询所有股票")
	log.Println("  GET /api/futures/:code - 查询单个期货")
	log.Println("  GET /api/futures       - 查询所有期货")
	log.Println("  GET /api/analysis      - 查询所有AI分析")
	log.Println("  GET /api/analysis/:code - 查询单个AI分析")
	log.Println("  GET /api/status        - 服务状态")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// loggerMiddleware 日志中间件
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Printf("[API] %s %s %d %v\n", c.Request.Method, path, status, latency)
	}
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
