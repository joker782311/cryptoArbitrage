package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// PriceResult 价格结果
type PriceResult struct {
	TokenIn   string  `json:"token_in"`
	TokenOut  string  `json:"token_out"`
	Price     float64 `json:"price"`
	Pool      string  `json:"pool"`
	Dex       string  `json:"dex"`
	Slippage  float64 `json:"slippage"`
}

// JupiterQuoteRequest Jupiter 报价请求
type JupiterQuoteRequest struct {
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	Amount     uint64 `json:"amount"`
	SlippageBps uint16 `json:"slippageBps"`
}

// JupiterQuoteResponse Jupiter 报价响应
type JupiterQuoteResponse struct {
	InputMint      string `json:"inputMint"`
	InAmount       string `json:"inAmount"`
	OutputMint     string `json:"outputMint"`
	OutAmount      string `json:"outAmount"`
	PriceImpactPct string `json:"priceImpactPct"`
	SlippageBps    uint16 `json:"slippageBps"`
	RoutePlan      []RoutePlan `json:"routePlan"`
}

// RoutePlan 路由计划
type RoutePlan struct {
	SwapInfo SwapInfo `json:"swapInfo"`
	Percent  int      `json:"percent"`
}

// SwapInfo 交换信息
type SwapInfo struct {
	AmmKey       string `json:"ammKey"`
	Label        string `json:"label"`
	InputMint    string `json:"inputMint"`
	OutputMint   string `json:"outputMint"`
	InAmount     string `json:"inAmount"`
	OutAmount    string `json:"outAmount"`
	FeeAmount    string `json:"feeAmount"`
	FeeMint      string `json:"feeMint"`
}

// JupiterClient Jupiter 聚合器客户端
type JupiterClient struct {
	HTTPClient *http.Client
	BaseURL    string
}

// NewJupiterClient 创建 Jupiter 客户端
func NewJupiterClient() *JupiterClient {
	return &JupiterClient{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    "https://quote-api.jup.ag/v6",
	}
}

// GetQuote 获取交换报价
func (j *JupiterClient) GetQuote(ctx context.Context, inputMint, outputMint string, amount uint64, slippageBps uint16) (*JupiterQuoteResponse, error) {
	url := fmt.Sprintf("%s/quote?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		j.BaseURL, inputMint, outputMint, amount, slippageBps)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := j.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var quote JupiterQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, err
	}

	return &quote, nil
}

// GetSwap 获取交换指令
func (j *JupiterClient) GetSwap(ctx context.Context, quote *JupiterQuoteResponse, userPublicKey string) (string, error) {
	body := map[string]interface{}{
		"quoteResponse": quote,
		"userPublicKey": userPublicKey,
		"wrapAndUnwrapSol": true,
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/swap", j.BaseURL),
		bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	swapTransaction, ok := result["swapTransaction"].(string)
	if !ok {
		return "", fmt.Errorf("no swap transaction in response")
	}

	return swapTransaction, nil
}

// CalculatePrice 计算价格
func (j *JupiterClient) CalculatePrice(inAmount, outAmount uint64, inDecimals, outDecimals uint8) float64 {
	// 考虑小数位
	inAmountFloat := float64(inAmount) / float64(big.NewInt(10).Exp(big.NewInt(10), big.NewInt(int64(inDecimals)), nil).Uint64())
	outAmountFloat := float64(outAmount) / float64(big.NewInt(10).Exp(big.NewInt(10), big.NewInt(int64(outDecimals)), nil).Uint64())

	return outAmountFloat / inAmountFloat
}

// RaydiumClient Raydium 客户端
type RaydiumClient struct {
	solanaClient *Client
}

// NewRaydiumClient 创建 Raydium 客户端
func NewRaydiumClient(solanaClient *Client) *RaydiumClient {
	return &RaydiumClient{
		solanaClient: solanaClient,
	}
}

// GetPoolInfo 获取池子信息
func (r *RaydiumClient) GetPoolInfo(ctx context.Context, poolID solana.PublicKey) (map[string]interface{}, error) {
	// 获取池子账户数据
	accountInfo, err := r.solanaClient.GetAccountInfo(ctx, poolID)
	if err != nil {
		return nil, err
	}

	if accountInfo.Value == nil {
		return nil, fmt.Errorf("pool account not found")
	}

	// 解析池子数据（需要知道 Raydium AMM 结构）
	return map[string]interface{}{
		"pool": poolID.String(),
	}, nil
}

// GetPrice 获取池子价格
func (r *RaydiumClient) GetPrice(ctx context.Context, poolID solana.PublicKey) (*PriceResult, error) {
	poolInfo, err := r.GetPoolInfo(ctx, poolID)
	if err != nil {
		return nil, err
	}

	// 计算价格
	return &PriceResult{
		Pool: poolID.String(),
		Dex:  "raydium",
	}, nil
}

// GetSwapQuote 获取交换报价
func (r *RaydiumClient) GetSwapQuote(ctx context.Context, inputMint, outputMint solana.PublicKey, amount uint64) (*PriceResult, error) {
	// 查找对应的池子
	// 计算输出金额
	return &PriceResult{
		TokenIn:  inputMint.String(),
		TokenOut: outputMint.String(),
		Dex:      "raydium",
	}, nil
}

// OrcaClient Orca 客户端
type OrcaClient struct {
	solanaClient *Client
}

// NewOrcaClient 创建 Orca 客户端
func NewOrcaClient(solanaClient *Client) *OrcaClient {
	return &OrcaClient{
		solanaClient: solanaClient,
	}
}

// GetWhirlpoolInfo 获取 Whirlpool 池子信息
func (o *OrcaClient) GetWhirlpoolInfo(ctx context.Context, poolID solana.PublicKey) (map[string]interface{}, error) {
	accountInfo, err := o.solanaClient.GetAccountInfo(ctx, poolID)
	if err != nil {
		return nil, err
	}

	if accountInfo.Value == nil {
		return nil, fmt.Errorf("whirlpool account not found")
	}

	return map[string]interface{}{
		"pool": poolID.String(),
	}, nil
}

// GetPrice 获取 Whirlpool 价格
func (o *OrcaClient) GetPrice(ctx context.Context, poolID solana.PublicKey) (*PriceResult, error) {
	poolInfo, err := o.GetWhirlpoolInfo(ctx, poolID)
	if err != nil {
		return nil, err
	}

	return &PriceResult{
		Pool: poolID.String(),
		Dex:  "orca",
	}, nil
}
