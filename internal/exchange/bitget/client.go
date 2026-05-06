package bitget

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL   = "https://api.bitget.com"
	TradeURL  = "https://api.bitget.com"
	MarketURL = "https://api.bitget.com"
)

// Client Bitget 客户端
type Client struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	Client     *http.Client
}

// NewClient 创建 Bitget 客户端
func NewClient(apiKey, secretKey, passphrase string) *Client {
	return &Client{
		APIKey:     apiKey,
		SecretKey:  secretKey,
		Passphrase: passphrase,
		Client:     &http.Client{Timeout: 30 * time.Second},
	}
}

// sign 生成签名
func (c *Client) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// setHeaders 设置请求头
func (c *Client) setHeaders(req *http.Request, method, path, body string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("ACCESS-KEY", c.APIKey)
	req.Header.Set("ACCESS-SIGN", c.sign(timestamp, method, path, body))
	req.Header.Set("ACCESS-PASSPHRASE", c.Passphrase)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("Content-Type", "application/json")
}

// do 执行请求
func (c *Client) do(req *http.Request, result interface{}) error {
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Bitget V2 响应格式
	var respData struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		return err
	}

	if respData.Code != 0 {
		return fmt.Errorf("Bitget error: code=%d, msg=%s", respData.Code, respData.Msg)
	}

	return json.Unmarshal(respData.Data, result)
}

// doRequest 简单请求（不带签名）
func (c *Client) doRequest(req *http.Request, result interface{}) error {
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var respData struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		return err
	}

	if respData.Code != 0 {
		return fmt.Errorf("Bitget error: code=%d, msg=%s", respData.Code, respData.Msg)
	}

	return json.Unmarshal(respData.Data, result)
}
