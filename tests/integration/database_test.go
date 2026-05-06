package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/config"
	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestMain 测试主函数，用于 setup 和 teardown
func TestMain(m *testing.M) {
	// Setup: 初始化数据库连接
	// 使用测试数据库或内存数据库
	code := m.Run()
	os.Exit(code)
}

func TestDatabaseConnection(t *testing.T) {
	// 跳过需要实际数据库的测试
	t.Skip("Skipping database test - requires MySQL connection")

	cfg := config.Load()
	dsn := cfg.Database.User + ":" + cfg.Database.Password + "@tcp(" + cfg.Database.Host + ":" + cfg.Database.Port + ")/" + cfg.Database.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"

	err := database.InitMySQL(dsn)
	assert.NoError(t, err)

	err = database.Migrate()
	assert.NoError(t, err)
}

func TestAPIKeyCRUD(t *testing.T) {
	t.Skip("Skipping database test")

	// 测试 API Key 的增删改查
	apiKey := &model.APIKey{
		Exchange:   "binance",
		Name:       "test_key",
		APIKey:     "test_api_key",
		APISecret:  "test_secret",
		IsEnabled:  true,
	}

	// Create
	database.DB.Create(apiKey)
	assert.NotZero(t, apiKey.ID)

	// Read
	var result model.APIKey
	database.DB.First(&result, apiKey.ID)
	assert.Equal(t, "binance", result.Exchange)

	// Update
	database.DB.Model(&result).Update("is_enabled", false)
	assert.False(t, result.IsEnabled)

	// Delete
	database.DB.Delete(&result)
}

func TestStrategyCRUD(t *testing.T) {
	t.Skip("Skipping database test")

	strategy := &model.Strategy{
		Name:          "cross_exchange",
		IsEnabled:     true,
		AutoExecute:   true,
		MinProfitRate: 0.5,
		MaxPosition:   10000,
		StopLossRate:  2.0,
		Config:        `{"exchanges":["binance","okx"]}`,
	}

	database.DB.Create(strategy)
	assert.NotZero(t, strategy.ID)

	var result model.Strategy
	database.DB.First(&result, strategy.ID)
	assert.Equal(t, "cross_exchange", result.Name)
}

func TestAlertCRUD(t *testing.T) {
	t.Skip("Skipping database test")

	alert := &model.Alert{
		Type:    "opportunity",
		Level:   "info",
		Title:   "Test Alert",
		Message: "This is a test alert",
		IsRead:  false,
	}

	database.DB.Create(alert)
	assert.NotZero(t, alert.ID)

	var result model.Alert
	database.DB.First(&result, alert.ID)
	assert.Equal(t, "Test Alert", result.Title)
}

func TestTickerPersistence(t *testing.T) {
	t.Skip("Skipping database test")

	ticker := &model.Ticker{
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		Price:     50000,
		Bid:       49999,
		Ask:       50001,
		Timestamp: time.Now(),
	}

	database.DB.Create(ticker)
	assert.NotZero(t, ticker.ID)
}
