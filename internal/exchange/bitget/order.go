package bitget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OrderSide 订单方向
type OrderSide string

const (
	SideBuy  OrderSide = "buy"
	SideSell OrderSide = "sell"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

// Order Bitget 订单
type Order struct {
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	OrderType string `json:"orderType"`
	Price     string `json:"price,omitempty"`
	Size      string `json:"size"`
	OrderID   string `json:"orderId"`
	Status    string `json:"status"`
	FillPrice string `json:"fillPrice"`
	FillSize  string `json:"fillSize"`
}

// PlaceOrder 下单（现货）
func (c *Client) PlaceOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, size string, price ...string) (*Order, error) {
	body := map[string]string{
		"symbol":    symbol,
		"side":      string(side),
		"orderType": string(orderType),
		"size":      size,
	}

	if len(price) > 0 && orderType == OrderTypeLimit {
		body["price"] = price[0]
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v2/spot/trade/order", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "POST", "/api/v2/spot/trade/order", string(jsonBody))

	var result Order
	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// PlaceMixOrder 下单（合约）
func (c *Client) PlaceMixOrder(ctx context.Context, symbol, marginCoin, side, orderType, size string, price ...string) (*Order, error) {
	body := map[string]string{
		"symbol":     symbol,
		"marginCoin": marginCoin,
		"side":       side,
		"orderType":  orderType,
		"size":       size,
	}

	if len(price) > 0 && orderType == "limit" {
		body["price"] = price[0]
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v2/mix/trade/order", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "POST", "/api/v2/mix/trade/order", string(jsonBody))

	var result Order
	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CancelOrder 撤单
func (c *Client) CancelOrder(ctx context.Context, symbol, orderID string) error {
	body := map[string]string{
		"symbol":  symbol,
		"orderId": orderID,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v2/spot/trade/cancel-order", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	c.setHeaders(req, "POST", "/api/v2/spot/trade/cancel-order", string(jsonBody))

	var result interface{}
	return c.do(req, &result)
}

// GetOrder 查询订单
func (c *Client) GetOrder(ctx context.Context, symbol, orderID string) (*Order, error) {
	url := fmt.Sprintf("%s/api/v2/spot/trade/orderInfo?symbol=%s&orderId=%s", TradeURL, symbol, orderID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v2/spot/trade/orderInfo", "")

	var result Order
	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
