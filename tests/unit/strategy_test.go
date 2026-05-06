package strategy_test

import (
	"testing"

	"github.com/joker782311/cryptoArbitrage/internal/strategy"
	"github.com/stretchr/testify/assert"
)

func TestCrossExchangeStrategy(t *testing.T) {
	exchanges := make(map[string]interface{})
	pairs := []string{"BTCUSDT", "ETHUSDT"}

	s := strategy.NewCrossExchangeStrategy(exchanges, pairs, 0.5, 10000)

	assert.NotNil(t, s)
	assert.Equal(t, 0.5, s.GetConfig()["min_profit"])
}

func TestOpportunity(t *testing.T) {
	opp := &strategy.Opportunity{
		StrategyType: "cross_exchange",
		ProfitRate:   1.5,
		ProfitAmount: 150,
		EstimatedGas: 20,
		Slippage:     5,
	}

	netProfit := opp.CalculateNetProfit()
	assert.Equal(t, 125.0, netProfit)
	assert.True(t, opp.IsValid(1.0))
	assert.False(t, opp.IsValid(2.0))
}

func TestFundingRateStrategy(t *testing.T) {
	exchanges := make(map[string]interface{})
	pairs := []string{"BTCUSDT"}

	s := strategy.NewFundingRateStrategy(exchanges, pairs, 0.01, 20000)
	assert.NotNil(t, s)

	s.UpdateRate("binance", "BTCUSDT", 0.0005)
	s.UpdateRate("okx", "BTCUSDT", 0.0001)

	// 检查是否检测到机会
	// opp := s.CheckOpportunity("BTCUSDT")
	// assert.NotNil(t, opp)
}

func TestSpotFutureStrategy(t *testing.T) {
	pairs := []string{"BTCUSDT"}
	s := strategy.NewSpotFutureStrategy("binance", pairs, 0.3, 15000)
	assert.NotNil(t, s)

	// 期现价差 0.5%
	opp := s.CheckOpportunity("BTCUSDT", 50000, 50250)
	assert.NotNil(t, opp)
	assert.Equal(t, 0.5, opp.ProfitRate)
}

func TestTriangularStrategy(t *testing.T) {
	pairs := []strategy.TriPair{
		{Base: "BTC", Quote: "USDT"},
		{Base: "ETH", Quote: "USDT"},
	}
	s := strategy.NewTriangularStrategy("binance", pairs, 0.2, 5000)
	assert.NotNil(t, s)
}
