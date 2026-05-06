package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"
)

const (
	BaseURL       = "https://api.binance.com"
	FuturesURL    = "https://fapi.binance.com"
	SpotWSURL     = "wss://stream.binance.com:9443/ws"
	FuturesWSURL  = "wss://fstream.binance.com/ws"
)

// Client 币安客户端
type Client struct {
	APIKey    string
	SecretKey string
	Client    *http.Client
}

// NewClient 创建币安客户端
func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		APIKey:    apiKey,
		SecretKey: secretKey,
		Client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// generateSignature 生成签名
func (c *Client) generateSignature(query string) string {
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(query))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

// do 执行 HTTP 请求
func (c *Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binance API error: status=%d", resp.StatusCode)
	}

	return decodeJSON(resp.Body, result)
}
