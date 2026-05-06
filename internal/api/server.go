package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joker782311/cryptoArbitrage/internal/api/handlers"
)

// Server API 服务器
type Server struct {
	router *gin.Engine
}

// NewServer 创建 API 服务器
func NewServer() *Server {
	gin.SetMode(gin.DebugMode)
	router := gin.Default()

	// CORS 中间件
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	return &Server{router: router}
}

// SetupRoutes 设置路由
func (s *Server) SetupRoutes() {
	// 健康检查
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1
	v1 := s.router.Group("/api/v1")
	{
		// 行情
		v1.GET("/tickers", handlers.GetTickers)
		v1.GET("/tickers/:exchange/:symbol", handlers.GetTicker)

		// 策略
		v1.GET("/strategies", handlers.ListStrategies)
		v1.GET("/strategies/:name", handlers.GetStrategy)
		v1.PUT("/strategies/:name", handlers.UpdateStrategy)
		v1.POST("/strategies/:name/enable", handlers.EnableStrategy)
		v1.POST("/strategies/:name/disable", handlers.DisableStrategy)
		v1.PUT("/strategies/:name/auto-execute", handlers.SetAutoExecute)

		// 订单
		v1.GET("/orders", handlers.ListOrders)
		v1.GET("/orders/:id", handlers.GetOrder)
		v1.POST("/orders", handlers.PlaceOrder)
		v1.DELETE("/orders/:exchange/:id", handlers.CancelOrder)
		v1.GET("/orders/stats", handlers.GetOrderStats)

		// 仓位
		v1.GET("/positions", handlers.GetPositions)
		v1.GET("/positions/stats", handlers.GetPositionStats)

		// 告警
		v1.GET("/alerts", handlers.ListAlerts)
		v1.GET("/alerts/unread", handlers.GetUnreadAlerts)
		v1.PUT("/alerts/read", handlers.MarkAlertsRead)
		v1.GET("/alerts/stats", handlers.GetAlertStats)

		// 配置
		v1.GET("/config/strategies", handlers.GetStrategyConfigs)
		v1.GET("/config/api-keys", handlers.GetAPIKeys)
		v1.POST("/config/api-keys", handlers.SaveAPIKey)
		v1.DELETE("/config/api-keys/:exchange", handlers.DeleteAPIKey)
		v1.GET("/config/alerts", handlers.GetAlertConfigs)
		v1.POST("/config/alerts", handlers.SaveAlertConfig)
		v1.DELETE("/config/alerts/:id", handlers.DeleteAlertConfig)

		// 风控
		v1.GET("/risk/stats", handlers.GetRiskStats)
		v1.GET("/risk/limits", handlers.GetStrategyLimits)
		v1.PUT("/risk/limits/:name", handlers.UpdateStrategyLimit)
	}

	// WebSocket
	v1.GET("/ws", handlers.WebSocketHandler)
}

// Run 启动服务器
func (s *Server) Run(port string) error {
	return s.router.Run(":" + port)
}
