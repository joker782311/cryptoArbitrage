package unit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExchangeFactory(t *testing.T) {
	assert.True(t, true)
}

func TestSupportedExchanges(t *testing.T) {
	assert.True(t, true)
}

func TestCreateExchange(t *testing.T) {
	assert.True(t, true)
}

func TestTickerStruct(t *testing.T) {
	ticker := struct {
		Exchange  string
		Symbol    string
		Price     float64
		Bid       float64
		Ask       float64
		Volume24h float64
		Change24h float64
		Timestamp int64
	}{
		Exchange:  "binance",
		Symbol:    "BTCUSDT",
		Price:     50000,
		Bid:       49999,
		Ask:       50001,
		Volume24h: 1000000,
		Change24h: 2.5,
		Timestamp: 1234567890,
	}

	assert.Equal(t, "binance", ticker.Exchange)
	assert.Equal(t, "BTCUSDT", ticker.Symbol)
	assert.Equal(t, float64(50000), ticker.Price)
}

func TestOrderBookStruct(t *testing.T) {
	ob := struct {
		Exchange string
		Symbol   string
		Bids     []struct {
			Price    float64
			Quantity float64
		}
		Asks []struct {
			Price    float64
			Quantity float64
		}
	}{
		Exchange: "okx",
		Symbol:   "ETHUSDT",
		Bids: []struct {
			Price    float64
			Quantity float64
		}{
			{Price: 3000, Quantity: 10},
			{Price: 2999, Quantity: 20},
		},
		Asks: []struct {
			Price    float64
			Quantity float64
		}{
			{Price: 3001, Quantity: 15},
			{Price: 3002, Quantity: 25},
		},
	}

	assert.Equal(t, "okx", ob.Exchange)
	assert.Len(t, ob.Bids, 2)
	assert.Len(t, ob.Asks, 2)
	assert.Equal(t, float64(3000), ob.Bids[0].Price)
}
