package binance

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Account 账户信息
type Account struct {
	MakerCommission int       `json:"makerCommission"`
	TakerCommission int       `json:"takerCommission"`
	Balances        []Balance `json:"balances"`
}

// Balance 余额
type Balance struct {
	Asset  string  `json:"asset"`
	Free   float64 `json:"free,string"`
	Locked float64 `json:"locked,string"`
}

// FuturesPosition 期货仓位
type FuturesPosition struct {
	Symbol        string  `json:"symbol"`
	PositionAmt   float64 `json:"positionAmt,string"`
	EntryPrice    float64 `json:"entryPrice,string"`
	MarkPrice     float64 `json:"markPrice,string"`
	UnRealizedPnL float64 `json:"unRealizedPnL,string"`
	LiquidationPrice string `json:"liquidationPrice,string"`
	PositionSide  string  `json:"positionSide"`
}

// GetAccount 获取账户信息
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/api/v3/account?%s&signature=%s", BaseURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var account Account
	if err := c.do(req, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// GetBalance 获取余额
func (c *Client) GetBalance(ctx context.Context, asset string) (*Balance, error) {
	account, err := c.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	for _, balance := range account.Balances {
		if balance.Asset == asset {
			return &balance, nil
		}
	}

	return &Balance{Asset: asset}, nil
}

// GetFuturesPositions 获取期货仓位
func (c *Client) GetFuturesPositions(ctx context.Context) ([]FuturesPosition, error) {
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	params.Set("recvWindow", "5000")

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/fapi/v2/positionRisk?%s&signature=%s", FuturesURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	var positions []FuturesPosition
	if err := c.do(req, &positions); err != nil {
		return nil, err
	}

	return positions, nil
}

// GetFuturesAccount 获取期货账户信息
func (c *Client) GetFuturesAccount(ctx context.Context) error {
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	params.Set("recvWindow", "5000")

	signature := c.generateSignature(params.Encode())
	reqURL := fmt.Sprintf("%s/fapi/v2/account?%s&signature=%s", FuturesURL, params.Encode(), signature)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	return c.do(req, &result)
}
