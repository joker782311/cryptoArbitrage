package main

import (
	"context"
	"fmt"
	_ "log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/api"
	"github.com/joker782311/cryptoArbitrage/internal/api/handlers"
	"github.com/joker782311/cryptoArbitrage/internal/config"
	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/exchange"
	"github.com/joker782311/cryptoArbitrage/internal/model"
	"github.com/joker782311/cryptoArbitrage/internal/service/alert"
	"github.com/joker782311/cryptoArbitrage/internal/service/order"
	"github.com/joker782311/cryptoArbitrage/internal/service/risk"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
	"github.com/joker782311/cryptoArbitrage/pkg/logger"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志
	if err := logger.Init(); err != nil {
		panic(err)
	}

	// 初始化数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName)

	if err := database.InitMySQL(dsn); err != nil {
		logger.Log.Fatalf("Failed to init MySQL: %v", err)
	}

	if err := database.InitRedis(cfg.Redis.Host+":"+cfg.Redis.Port, cfg.Redis.Password); err != nil {
		logger.Log.Fatalf("Failed to init Redis: %v", err)
	}

	if err := database.Migrate(); err != nil {
		logger.Log.Fatalf("Failed to migrate database: %v", err)
	}

	logger.Log.Info("Database initialized")

	// 初始化交易所
	exchangeFactory := exchange.NewExchangeFactory()

	// 创建交易所实例（不需要 API Key 也可以获取行情）
	exchanges := make(map[string]exchange.Exchange)
	for _, exName := range exchange.SupportedExchanges() {
		ex, err := exchange.CreateExchange(exName, "", "", "")
		if err != nil {
			logger.Log.Warnf("Failed to create exchange %s: %v", exName, err)
			continue
		}
		exchanges[exName] = ex
		exchangeFactory.Register(exName, ex)
	}

	// 初始化 handlers（传入 exchangeFactory）
	handlers.InitHandlers(exchangeFactory)

	// 初始化策略引擎
	strategyEngine := strategy.NewEngine(exchanges)

	// 跨交易所策略
	crossExchangeStrategy := strategy.NewCrossExchangeStrategy(
		exchanges,
		[]string{"BTCUSDT", "ETHUSDT"},
		0.5,   // 最小利润率 0.5%
		10000, // 最大金额 10000 USDT
	)
	strategyEngine.AddStrategy(crossExchangeStrategy)

	// 资金费率策略
	fundingRateStrategy := strategy.NewFundingRateStrategy(
		exchanges,
		[]string{"BTCUSDT", "ETHUSDT"},
		0.01,  // 最小费率差 1%
		20000, // 最大金额 20000 USDT
	)
	strategyEngine.AddStrategy(fundingRateStrategy)

	// 期现策略
	spotFutureStrategy := strategy.NewSpotFutureStrategy(
		"binance",
		[]string{"BTCUSDT", "ETHUSDT"},
		0.3,   // 最小基差 0.3%
		15000, // 最大金额 15000 USDT
	)
	strategyEngine.AddStrategy(spotFutureStrategy)

	// 三角套利策略
	triangularStrategy := strategy.NewTriangularStrategy(
		"binance",
		[]strategy.TriPair{
			{Base: "BTC", Quote: "USDT"},
			{Base: "ETH", Quote: "USDT"},
		},
		0.2,  // 最小利润率 0.2%
		5000, // 最大金额 5000 USDT
	)
	strategyEngine.AddStrategy(triangularStrategy)

	logger.Log.Info("Strategies initialized")

	// 插入测试数据（如果数据库为空）
	initSeedData()

	// 初始化订单管理器
	_ = order.NewManager(exchanges)

	// 初始化风险管理器
	_ = risk.NewManager(100000, 5000) // 最大仓位 100000 USDT, 日止损 5000 USDT

	// 初始化告警引擎
	_ = alert.NewEngine()

	// 启动策略引擎
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := strategyEngine.Start(); err != nil {
		logger.Log.Fatalf("Failed to start strategy engine: %v", err)
	}

	// 启动行情监控
	for exName, ex := range exchanges {
		for _, symbol := range []string{"BTCUSDT", "ETHUSDT"} {
			go func(name, sym string) {
				ticker, err := ex.GetTicker(ctx, sym)
				if err != nil {
					return
				}
				crossExchangeStrategy.UpdatePrice(name, sym, ticker.Price)
			}(exName, symbol)
		}
	}

	logger.Log.Info("Strategy engine started")

	// 初始化 API 服务器
	apiServer := api.NewServer()

	// 启动 API 服务器
	go func() {
		logger.Log.Infof("Starting API server on port %s", cfg.Server.Port)
		if err := apiServer.Run(cfg.Server.Port); err != nil {
			logger.Log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("Shutting down...")
	cancel()

	// 停止策略引擎
	strategyEngine.Stop()

	logger.Log.Info("Server stopped")
}

// initSeedData 插入测试数据
func initSeedData() {
	now := time.Now()

	// 插入策略数据
	strategies := []model.Strategy{
		{
			Name:          "cross_exchange",
			IsEnabled:     true,
			AutoExecute:   true,
			MinProfitRate: 0.5,
			MaxPosition:   10000,
			Config:        `{"exchanges": ["binance", "okx"], "symbols": ["BTCUSDT", "ETHUSDT"]}`,
		},
		{
			Name:          "funding_rate",
			IsEnabled:     true,
			AutoExecute:   false,
			MinProfitRate: 1.0,
			MaxPosition:   20000,
			Config:        `{"exchanges": ["binance", "okx"], "symbols": ["BTCUSDT", "ETHUSDT"]}`,
		},
		{
			Name:          "spot_future",
			IsEnabled:     true,
			AutoExecute:   true,
			MinProfitRate: 0.3,
			MaxPosition:   15000,
			Config:        `{"exchange": "binance", "symbols": ["BTCUSDT", "ETHUSDT"]}`,
		},
		{
			Name:          "triangular",
			IsEnabled:     false,
			AutoExecute:   false,
			MinProfitRate: 0.2,
			MaxPosition:   5000,
			Config:        `{"exchange": "binance", "pairs": [{"base": "BTC", "quote": "USDT"}, {"base": "ETH", "quote": "USDT"}]}`,
		},
	}

	for _, s := range strategies {
		var existing model.Strategy
		if err := database.DB.Where("name = ?", s.Name).First(&existing).Error; err != nil {
			database.DB.Create(&s)
			logger.Log.Infof("Inserted strategy: %s", s.Name)
		}
	}

	// 插入订单数据
	orders := []model.Order{
		{
			StrategyID:  1,
			Exchange:    "binance",
			Symbol:      "BTCUSDT",
			Side:        "buy",
			Type:        "market",
			Price:       67500,
			Quantity:    0.1,
			ExecutedQty: 0.1,
			Status:      "filled",
			OrderID:     "BNB001",
			CreatedAt:   now.Add(-2 * time.Hour),
		},
		{
			StrategyID:  1,
			Exchange:    "okx",
			Symbol:      "BTCUSDT",
			Side:        "sell",
			Type:        "market",
			Price:       67520,
			Quantity:    0.1,
			ExecutedQty: 0.1,
			Status:      "filled",
			OrderID:     "OKX001",
			CreatedAt:   now.Add(-2*time.Hour + time.Minute),
		},
		{
			StrategyID:  2,
			Exchange:    "binance",
			Symbol:      "ETHUSDT",
			Side:        "buy",
			Type:        "market",
			Price:       3450,
			Quantity:    1.0,
			ExecutedQty: 1.0,
			Status:      "filled",
			OrderID:     "BNB002",
			CreatedAt:   now.Add(-1 * time.Hour),
		},
		{
			StrategyID:  2,
			Exchange:    "okx",
			Symbol:      "ETHUSDT",
			Side:        "sell",
			Type:        "market",
			Price:       3452,
			Quantity:    1.0,
			ExecutedQty: 1.0,
			Status:      "filled",
			OrderID:     "OKX002",
			CreatedAt:   now.Add(-1*time.Hour + time.Minute*30),
		},
		{
			StrategyID:  3,
			Exchange:    "binance",
			Symbol:      "BTCUSDT",
			Side:        "buy",
			Type:        "limit",
			Price:       67000,
			Quantity:    0.05,
			ExecutedQty: 0,
			Status:      "pending",
			OrderID:     "BNB003",
			CreatedAt:   now.Add(-30 * time.Minute),
		},
	}

	for _, o := range orders {
		var existing model.Order
		if err := database.DB.Where("order_id = ?", o.OrderID).First(&existing).Error; err != nil {
			database.DB.Create(&o)
			logger.Log.Infof("Inserted order: %s - %s %s", o.OrderID, o.Exchange, o.Symbol)
		}
	}
}
