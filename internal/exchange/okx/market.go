package okx

import (
	"context"
	"fmt"
	"net/http"
)

// Ticker OKX 行情
type Ticker struct {
	InstID           string `json:"instId"`
	Last             string `json:"last"`
	LastPx           string `json:"lastPx"`
	BidPx            string `json:"bidPx"`
	AskPx            string `json:"askPx"`
	BidSz            string `json:"bidSz"`
	AskSz            string `json:"askSz"`
	Vol24h           string `json:"vol24h"`
	VolCcy24h        string `json:"volCcy24h"`
	Open24h          string `json:"open24h"`
	High24h          string `json:"high24h"`
	Low24h           string `json:"low24h"`
	ChangePercent24h string `json:"chgPct24h"`
}

// OrderBook OKX 订单簿
type OrderBook struct {
	Bids [][]string `json:"bids"` // [price, size]
	Asks [][]string `json:"asks"`
}

// FundingRate OKX 资金费率
type FundingRate struct {
	InstID      string `json:"instId"`
	FundingRate string `json:"fundingRate"`
	NextFundingTime string `json:"nextFundingTime"`
}

// GetTicker 获取行情
func (c *Client) GetTicker(ctx context.Context, instID string) (*Ticker, error) {
	url := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", MarketURL, instID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var ticker Ticker
	if err := c.doRequest(req, &ticker); err != nil {
		return nil, err
	}

	return &ticker, nil
}

// GetTickers 获取所有行情
func (c *Client) GetTickers(ctx context.Context, instType string) ([]Ticker, error) {
	url := fmt.Sprintf("%s/api/v5/market/tickers?instType=%s", MarketURL, instType)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var tickers []Ticker
	if err := c.doRequest(req, &tickers); err != nil {
		return nil, err
	}

	return tickers, nil
}

// GetOrderBook 获取订单簿
func (c *Client) GetOrderBook(ctx context.Context, instID string, size int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=%d", MarketURL, instID, size)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var ob OrderBook
	if err := c.doRequest(req, &ob); err != nil {
		return nil, err
	}

	return &ob, nil
}

// GetFundingRate 获取资金费率
func (c *Client) GetFundingRate(ctx context.Context, instID string) (*FundingRate, error) {
	url := fmt.Sprintf("%s/api/v5/public/funding-rate?instId=%s", MarketURL, instID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var rates []FundingRate
	if err := c.doRequest(req, &rates); err != nil {
		return nil, err
	}

	if len(rates) == 0 {
		return nil, fmt.Errorf("no funding rate found for %s", instID)
	}

	return &rates[0], nil
}
