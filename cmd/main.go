package main

import (
	"context"
	"fmt"
	_ "log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joker782311/cryptoArbitrage/internal/api"
	"github.com/joker782311/cryptoArbitrage/internal/config"
	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/exchange"
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

	// 从配置加载交易所 API Key
	_ = exchangeFactory // Placeholder for future use
	for _, exName := range exchange.SupportedExchanges() {
		_ = exName // Placeholder
	}

	exchanges := exchangeFactory.GetAll()

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
