package binance

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// OrderSide 订单方向
type OrderSide string

const (
	SideBuy  OrderSide = "BUY"
	SideSell OrderSide = "SELL"
)

// OrderType 订单类型
type OrderType string

const (
	TypeMarket OrderType = "MARKET"
	TypeLimit  OrderType = "LIMIT"
)

// Order 订单
type Order struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	Type         string  `json:"type"`
	Price        float64 `json:"price,string"`
	Quantity     float64 `json:"origQty,string"`
	ExecutedQty  float64 `json:"executedQty,string"`
	Status       string  `json:"status"`
	OrderID      int64   `json:"orderId"`
	TransactTime int64   `json:"transactTime"`
}

// PlaceOrder 下单
func (c *Client) PlaceOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price ...float64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", string(side))
	params.Set("type", string(orderType))
	params.Set("quantity", fmt.Sprintf("%f", quantity))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	if len(price) > 0 && orderType == TypeLimit {
		params.Set("price", fmt.Sprintf("%f", price[0]))
	}

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/api/v3/order?%s&signature=%s", BaseURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var order Order
	if err := c.do(req, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// CancelOrder 撤单
func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", fmt.Sprintf("%d", orderID))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/api/v3/order?%s&signature=%s", BaseURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	return c.do(req, &result)
}

// GetOrder 查询订单
func (c *Client) GetOrder(ctx context.Context, symbol string, orderID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", fmt.Sprintf("%d", orderID))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/api/v3/order?%s&signature=%s", BaseURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var order Order
	if err := c.do(req, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// PlaceFuturesOrder 期货下单
func (c *Client) PlaceFuturesOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price ...float64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", string(side))
	params.Set("type", string(orderType))
	params.Set("quantity", fmt.Sprintf("%f", quantity))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	params.Set("recvWindow", "5000")

	if len(price) > 0 && orderType == TypeLimit {
		params.Set("price", fmt.Sprintf("%f", price[0]))
	}

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/fapi/v1/order?%s&signature=%s", FuturesURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var order Order
	if err := c.do(req, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// CancelFuturesOrder 期货撤单
func (c *Client) CancelFuturesOrder(ctx context.Context, symbol string, orderID int64) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", fmt.Sprintf("%d", orderID))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	params.Set("recvWindow", "5000")

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/fapi/v1/order?%s&signature=%s", FuturesURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	return c.do(req, &result)
}
