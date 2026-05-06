package exchange_test

import (
	"context"
	"testing"

	"github.com/joker782311/cryptoArbitrage/internal/exchange"
	"github.com/stretchr/testify/assert"
)

func TestExchangeFactory(t *testing.T) {
	factory := exchange.NewExchangeFactory()
	assert.NotNil(t, factory)
}

func TestSupportedExchanges(t *testing.T) {
	supported := exchange.SupportedExchanges()
	assert.Contains(t, supported, "binance")
	assert.Contains(t, supported, "okx")
	assert.Contains(t, supported, "bitget")
}

func TestCreateExchange(t *testing.T) {
	// 测试创建交易所（使用空 AK/SK）
	ex, err := exchange.CreateExchange("binance", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, ex)

	ex, err = exchange.CreateExchange("okx", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, ex)

	ex, err = exchange.CreateExchange("bitget", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, ex)

	// 测试不支持的交易所
	_, err = exchange.CreateExchange("unknown", "", "", "")
	assert.Error(t, err)
}

func TestTickerStruct(t *testing.T) {
	ticker := exchange.Ticker{
		Exchange:      "binance",
		Symbol:        "BTCUSDT",
		Price:         50000,
		Bid:           49999,
		Ask:           50001,
		Volume24h:     1000000,
		Change24h:     2.5,
		Timestamp:     1234567890,
	}

	assert.Equal(t, "binance", ticker.Exchange)
	assert.Equal(t, "BTCUSDT", ticker.Symbol)
	assert.Equal(t, float64(50000), ticker.Price)
}

func TestOrderBookStruct(t *testing.T) {
	ob := exchange.OrderBook{
		Exchange: "okx",
		Symbol:   "ETHUSDT",
		Bids: []exchange.PriceLevel{
			{Price: 3000, Quantity: 10},
			{Price: 2999, Quantity: 20},
		},
		Asks: []exchange.PriceLevel{
			{Price: 3001, Quantity: 15},
			{Price: 3002, Quantity: 25},
		},
	}

	assert.Equal(t, "okx", ob.Exchange)
	assert.Len(t, ob.Bids, 2)
	assert.Len(t, ob.Asks, 2)
	assert.Equal(t, float64(3000), ob.Bids[0].Price)
}
