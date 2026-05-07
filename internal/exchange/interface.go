package exchange

import (
	"context"
)

// Ticker 统一行情结构
type Ticker struct {
	Exchange       string  `json:"exchange"`
	Symbol         string  `json:"symbol"`
	Price          float64 `json:"price"`
	Bid            float64 `json:"bid"`
	Ask            float64 `json:"ask"`
	Volume24h      float64 `json:"volume_24h"`
	Change24h      float64 `json:"change_24h"`
	High24h        float64 `json:"high_24h"`
	Low24h         float64 `json:"low_24h"`
	QuoteVolume24h float64 `json:"quote_volume_24h"`
	Timestamp      int64   `json:"timestamp"`
}

// OrderBook 统一订单簿结构
type OrderBook struct {
	Exchange string       `json:"exchange"`
	Symbol   string       `json:"symbol"`
	Bids     []PriceLevel `json:"bids"`
	Asks     []PriceLevel `json:"asks"`
}

// PriceLevel 价格层次
type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// Order 统一订单结构
type Order struct {
	Exchange    string  `json:"exchange"`
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`
	Type        string  `json:"type"`
	Price       float64 `json:"price"`
	Quantity    float64 `json:"quantity"`
	ExecutedQty float64 `json:"executed_qty"`
	Status      string  `json:"status"`
	OrderID     string  `json:"order_id"`
}

// Balance 统一余额结构
type Balance struct {
	Exchange string  `json:"exchange"`
	Asset    string  `json:"asset"`
	Free     float64 `json:"free"`
	Locked   float64 `json:"locked"`
}

// Position 统一仓位结构
type Position struct {
	Exchange     string  `json:"exchange"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	Quantity     float64 `json:"quantity"`
	EntryPrice   float64 `json:"entry_price"`
	CurrentPrice float64 `json:"current_price"`
	PNL          float64 `json:"pnl"`
	PNLPercent   float64 `json:"pnl_percent"`
}

// FundingRate 统一资金费率结构
type FundingRate struct {
	Exchange    string  `json:"exchange"`
	Symbol      string  `json:"symbol"`
	FundingRate float64 `json:"funding_rate"`
	NextFunding int64   `json:"next_funding"`
}

// FuturesPricer 期货价格接口（仅 Binance 实现）
type FuturesPricer interface {
	GetFuturesTicker(ctx context.Context, symbol string) (*Ticker, error)
}

// Exchange 交易所接口
type Exchange interface {
	// 行情
	GetTicker(ctx context.Context, symbol string) (*Ticker, error)
	GetTickers(ctx context.Context) ([]Ticker, error)
	GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error)
	GetFundingRate(ctx context.Context, symbol string) (*FundingRate, error)

	// 交易
	PlaceOrder(ctx context.Context, symbol, side, orderType string, quantity, price float64) (*Order, error)
	CancelOrder(ctx context.Context, symbol, orderID string) error
	GetOrder(ctx context.Context, symbol, orderID string) (*Order, error)

	// 账户
	GetBalance(ctx context.Context, asset string) (*Balance, error)
	GetPositions(ctx context.Context) ([]Position, error)

	// WebSocket
	SubscribeTicker(ctx context.Context, symbols []string, handler func(*Ticker)) error
	SubscribeOrderBook(ctx context.Context, symbol string, limit int, handler func(*OrderBook)) error
}
