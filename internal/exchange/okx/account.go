package okx

import (
	"context"
	"fmt"
	"net/http"
)

// Account 账户信息
type Account struct {
	TotalEq     string     `json:"totEq"`
	Details     []Detail   `json:"details"`
}

// Detail 币种详情
type Detail struct {
	Ccy         string `json:"ccy"`
	Eq          string `json:"eq"`
	AvailEq     string `json:"availEq"`
	CashBal     string `json:"cashBal"`
	UavailEq    string `json:"uavailEq"`
}

// Position 仓位
type Position struct {
	InstID      string `json:"instId"`
	Pos         string `json:"pos"`
	PosSide     string `json:"posSide"`
	AvgPx       string `json:"avgPx"`
	Margin      string `json:"margin"`
	Pnl         string `json:"pnl"`
	PnlRatio    string `json:"pnlRatio"`
	LiqPx       string `json:"liqPx"`
}

// GetAccount 获取账户信息
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	url := fmt.Sprintf("%s/api/v5/account", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v5/account", "")

	var accounts []Account
	if err := c.do(req, &accounts); err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no account data")
	}

	return &accounts[0], nil
}

// GetBalance 获取余额
func (c *Client) GetBalance(ctx context.Context, ccy ...string) ([]Detail, error) {
	url := fmt.Sprintf("%s/api/v5/account/balance", TradeURL)
	if len(ccy) > 0 {
		url += fmt.Sprintf("?ccy=%s", ccy[0])
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v5/account/balance", "")

	var result struct {
		Details []Detail `json:"details"`
	}

	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	return result.Details, nil
}

// GetPositions 获取仓位
func (c *Client) GetPositions(ctx context.Context, instType, instID string) ([]Position, error) {
	url := fmt.Sprintf("%s/api/v5/account/positions?instType=%s", TradeURL, instType)
	if instID != "" {
		url += fmt.Sprintf("&instId=%s", instID)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v5/account/positions", "")

	var positions []Position
	if err := c.do(req, &positions); err != nil {
		return nil, err
	}

	return positions, nil
}
