package bitget

import (
	"context"
	"fmt"
	"net/http"
)

// Account 账户信息
type Account struct {
	TotalUSDT string `json:"totalUSDT"`
	Assets    []Asset `json:"assetList"`
}

// Asset 币种资产
type Asset struct {
	CoinName  string `json:"coinName"`
	Available string `json:"available"`
	Frozen    string `json:"frozen"`
	Lock      string `json:"lock"`
}

// MixPosition 合约仓位
type MixPosition struct {
	Symbol      string `json:"symbol"`
	MarginCoin  string `json:"marginCoin"`
	HoldPos     string `json:"holdPos"`
	OpenAvgPrice string `json:"openAvgPrice"`
	CloseAvgPrice string `json:"closeAvgPrice"`
	Leverage    string `json:"leverage"`
	UnrPnl      string `json:"unrPnl"`
}

// GetAccount 获取账户信息（现货）
func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	url := fmt.Sprintf("%s/api/v2/spot/account/info", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v2/spot/account/info", "")

	var result struct {
		AssetList []Asset `json:"assetList"`
	}

	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	account := &Account{
		Assets: result.AssetList,
	}
	return account, nil
}

// GetBalance 获取余额
func (c *Client) GetBalance(ctx context.Context, coinName string) (*Asset, error) {
	url := fmt.Sprintf("%s/api/v2/spot/account/assets", TradeURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v2/spot/account/assets", "")

	var assets []Asset
	if err := c.do(req, &assets); err != nil {
		return nil, err
	}

	for _, asset := range assets {
		if asset.CoinName == coinName {
			return &asset, nil
		}
	}

	return nil, fmt.Errorf("asset not found: %s", coinName)
}

// GetMixAccount 获取合约账户
func (c *Client) GetMixAccount(ctx context.Context, symbol, marginCoin string) error {
	url := fmt.Sprintf("%s/api/v2/mix/account/account?symbol=%s&marginCoin=%s", TradeURL, symbol, marginCoin)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	c.setHeaders(req, "GET", "/api/v2/mix/account/account", "")

	var result interface{}
	return c.do(req, &result)
}

// GetPositions 获取合约仓位
func (c *Client) GetPositions(ctx context.Context, symbol, marginCoin string) ([]MixPosition, error) {
	url := fmt.Sprintf("%s/api/v2/mix/position/current?symbol=%s&marginCoin=%s", TradeURL, symbol, marginCoin)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, "GET", "/api/v2/mix/position/current", "")

	var result struct {
		PositionList []MixPosition `json:"positionList"`
	}

	if err := c.do(req, &result); err != nil {
		return nil, err
	}

	return result.PositionList, nil
}
