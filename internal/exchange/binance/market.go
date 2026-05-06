package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Ticker 行情信息
type Ticker struct {
	Symbol             string  `json:"symbol"`
	LastPrice          float64 `json:"lastPrice,string"`
	BidPrice           float64 `json:"bidPrice,string"`
	AskPrice           float64 `json:"askPrice,string"`
	Volume             float64 `json:"volume,string"`
	QuoteVolume        float64 `json:"quoteVolume,string"`
	PriceChangePercent float64 `json:"priceChangePercent,string"`
	High24h            float64 `json:"high24h,string"`
	Low24h             float64 `json:"low24h,string"`
}

// OrderBook 订单簿
type OrderBook struct {
	LastUpdateID int64    `json:"lastUpdateId"`
	Bids         []Level  `json:"bids"`
	Asks         []Level  `json:"asks"`
}

// Level 价格层次
type Level struct {
	Price    float64
	Quantity float64
}

// FundingRate 资金费率
type FundingRate struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
}

// GetTicker 获取 24 小时行情
func (c *Client) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/24hr?symbol=%s", BaseURL, symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var ticker Ticker
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := decodeJSON(resp.Body, &ticker); err != nil {
		return nil, err
	}

	return &ticker, nil
}

// GetOrderBook 获取订单簿
func (c *Client) GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=%d", BaseURL, symbol, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		LastUpdateID int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}

	if err := doRequest(c.Client, req, &resp); err != nil {
		return nil, err
	}

	// 转换数据格式
	orderBook := &OrderBook{
		LastUpdateID: resp.LastUpdateID,
		Bids:         make([]Level, len(resp.Bids)),
		Asks:         make([]Level, len(resp.Asks)),
	}

	for i, bid := range resp.Bids {
		orderBook.Bids[i].Price = parseFloat(bid[0])
		orderBook.Bids[i].Quantity = parseFloat(bid[1])
	}

	for i, ask := range resp.Asks {
		orderBook.Asks[i].Price = parseFloat(ask[0])
		orderBook.Asks[i].Quantity = parseFloat(ask[1])
	}

	return orderBook, nil
}

// GetFundingRate 获取资金费率
func (c *Client) GetFundingRate(ctx context.Context, symbol string) (*FundingRate, error) {
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", FuturesURL, symbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var rate FundingRate
	if err := doRequest(c.Client, req, &rate); err != nil {
		return nil, err
	}

	return &rate, nil
}

// GetTickers 获取所有币种行情
func (c *Client) GetTickers(ctx context.Context) ([]Ticker, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/24hr", BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var tickers []Ticker
	if err := doRequest(c.Client, req, &tickers); err != nil {
		return nil, err
	}

	return tickers, nil
}

// 辅助函数
func doRequest(client *http.Client, req *http.Request, result interface{}) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return decodeJSON(resp.Body, result)
}

func decodeJSON(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}
