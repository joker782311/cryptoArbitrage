package service_test

import (
	"testing"

	"github.com/joker782311/cryptoArbitrage/internal/service/risk"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
	"github.com/stretchr/testify/assert"
)

func TestRiskManager(t *testing.T) {
	rm := risk.NewManager(100000, 5000)
	assert.NotNil(t, rm)
}

func TestRiskManager_CanExecute(t *testing.T) {
	rm := risk.NewManager(100000, 5000)

	opp := &strategy.Opportunity{
		StrategyType: "cross_exchange",
		ProfitAmount: 1000,
	}

	// 应该允许执行
	err := rm.CanExecute(opp)
	assert.NoError(t, err)
}

func TestRiskManager_DailyLossLimit(t *testing.T) {
	rm := risk.NewManager(100000, 1000)

	// 模拟日亏损超过限制
	rm.UpdateDailyPnL(-1500)

	// 获取内部状态来测试
	assert.Equal(t, float64(-1500), rm.GetDailyPnL())
}

func TestStrategyLimits(t *testing.T) {
	rm := risk.NewManager(100000, 5000)

	limits := rm.GetAllStrategyLimits()
	assert.NotEmpty(t, limits)

	// 检查默认配置
	limit := rm.GetStrategyLimit("cross_exchange")
	assert.NotNil(t, limit)
	assert.True(t, limit.Enabled)
	assert.True(t, limit.AutoExecute)
}

func TestPositionStats(t *testing.T) {
	rm := risk.NewManager(100000, 5000)

	stats := rm.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, float64(0), stats.TotalPosition)
	assert.Equal(t, float64(0), stats.DailyPnL)
}
