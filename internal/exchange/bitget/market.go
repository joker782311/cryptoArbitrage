package bitget

import (
	"context"
	"fmt"
	"net/http"
)

// Ticker Bitget 行情
type Ticker struct {
	Symbol    string `json:"symbol"`
	LastPr    string `json:"lastPr"`
	BidPr     string `json:"bidPr"`
	AskPr     string `json:"askPr"`
	BidSz     string `json:"bidSz"`
	AskSz     string `json:"askSz"`
	High24h   string `json:"high24h"`
	Low24h    string `json:"low24h"`
	Change24h string `json:"ch24h"`
	ChangePct string `json:"pct24h"`
	Vol24h    string `json:"vol24h"`
	QuoteVol  string `json:"usdtVol"`
}

// OrderBook Bitget 订单簿
type OrderBook struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

// FundingRate Bitget 资金费率
type FundingRate struct {
	Symbol        string `json:"symbol"`
	FundingRate   string `json:"fundingRate"`
	FeeRate       string `json:"feeRate"`
	NextSettleTime string `json:"nextSettleTime"`
}

// GetTicker 获取行情
func (c *Client) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	url := fmt.Sprintf("%s/api/v2/spot/market/tickers?symbol=%s", MarketURL, symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tickers []Ticker `json:"data"`
	}

	if err := c.doRequest(req, &result); err != nil {
		return nil, err
	}

	if len(result.Tickers) == 0 {
		return nil, fmt.Errorf("no ticker found for %s", symbol)
	}

	return &result.Tickers[0], nil
}

// GetTickers 获取所有行情
func (c *Client) GetTickers(ctx context.Context) ([]Ticker, error) {
	url := fmt.Sprintf("%s/api/v2/spot/market/tickers", MarketURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tickers []Ticker `json:"data"`
	}

	if err := c.doRequest(req, &result); err != nil {
		return nil, err
	}

	return result.Tickers, nil
}

// GetOrderBook 获取订单簿
func (c *Client) GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/api/v2/spot/market/orderbook?symbol=%s&limit=%d", MarketURL, symbol, limit)
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

// GetFundingRate 获取资金费率（合约）
func (c *Client) GetFundingRate(ctx context.Context, symbol string) (*FundingRate, error) {
	url := fmt.Sprintf("%s/api/v2/mix/market/funding-rate?symbol=%s", MarketURL, symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Rates []FundingRate `json:"data"`
	}

	if err := c.doRequest(req, &result); err != nil {
		return nil, err
	}

	if len(result.Rates) == 0 {
		return nil, fmt.Errorf("no funding rate found for %s", symbol)
	}

	return &result.Rates[0], nil
}
