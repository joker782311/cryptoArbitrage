package okx

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

// Order OKX 订单
type Order struct {
	InstID    string `json:"instId"`
	Side      string `json:"side"`
	OrdType   string `json:"ordType"`
	Px        string `json:"px,omitempty"`
	Sz        string `json:"sz"`
	OrdID     string `json:"ordId"`
	State     string `json:"state"`
	FillPx    string `json:"fillPx"`
	FillSz    string `json:"fillSz"`
}

// PlaceOrder 下单
func (c *Client) PlaceOrder(ctx context.Context, instID string, side OrderSide, ordType OrderType, sz string, px ...string) (*Order, error) {
	body := map[string]interface{}{
		"instId":  instID,
		"side":    side,
		"ordType": ordType,
		"sz":      sz,
	}

	if len(px) > 0 && ordType == OrderTypeLimit {
		body["px"] = px[0]
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v5/trade/order", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "POST", "/api/v5/trade/order", string(jsonBody))

	var order Order
	if err := c.do(req, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// CancelOrder 撤单
func (c *Client) CancelOrder(ctx context.Context, instID, ordID string) error {
	body := map[string]string{
		"instId": instID,
		"ordId":  ordID,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v5/trade/cancel-order", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	c.setHeaders(req, "POST", "/api/v5/trade/cancel-order", string(jsonBody))

	var result interface{}
	return c.do(req, &result)
}

// GetOrder 查询订单
func (c *Client) GetOrder(ctx context.Context, instID, ordID string) (*Order, error) {
	url := fmt.Sprintf("%s/api/v5/trade/order?instId=%s&ordId=%s", TradeURL, instID, ordID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v5/trade/order", "")

	var orders []Order
	if err := c.do(req, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("order not found: %s", ordID)
	}

	return &orders[0], nil
}

// PlaceBatchOrders 批量下单
func (c *Client) PlaceBatchOrders(ctx context.Context, orders []map[string]interface{}) ([]Order, error) {
	body := map[string]interface{}{
		"orders": orders,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v5/trade/batch-orders", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "POST", "/api/v5/trade/batch-orders", string(jsonBody))

	var result []Order
	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	return result, nil
}
