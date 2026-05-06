package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// SwapParams 交换参数
type SwapParams struct {
	TokenIn      string
	TokenOut     string
	AmountIn     *big.Int
	MinAmountOut *big.Int
	To           string
	Deadline     uint64
}

// SwapResult 交换结果
type SwapResult struct {
	TxHash      string
	AmountIn    *big.Int
	AmountOut   *big.Int
	PriceImpact float64
	GasUsed     *big.Int
}

// UniswapV2Router Uniswap V2 Router
type UniswapV2Router struct {
	client  *Client
	address common.Address
}

// Uniswap V2 Router 地址
var RouterAddresses = map[ChainID]string{
	Ethereum: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
	BSC:      "0x10ED43C718714eb63d5aA57B78B54704E256024E", // PancakeSwap
	Polygon:  "0xa5E0829CaCEd8fFDD4De3c43696c57F7D7A678ff", // QuickSwap
}

// NewUniswapV2Router 创建 Uniswap V2 Router
func NewUniswapV2Router(client *Client) *UniswapV2Router {
	address, ok := RouterAddresses[client.ChainID]
	if !ok {
		address = RouterAddresses[Ethereum] // 默认使用 Ethereum
	}
	return &UniswapV2Router{
		client:  client,
		address: common.HexToAddress(address),
	}
}

// GetAmountsOut 获取预期输出金额
func (r *UniswapV2Router) GetAmountsOut(ctx context.Context, amountIn *big.Int, path []string) ([]*big.Int, error) {
	// 调用 router.getAmountsOut
	// method signature: 0xd06ca61f

	pathAddresses := make([]common.Address, len(path))
	for i, p := range path {
		pathAddresses[i] = common.HexToAddress(p)
	}

	data := "0xd06ca61f"
	data += common.LeftPadBytes(amountIn.Bytes(), 32).Hex()[2:]
	data += common.LeftPadBytes(big.NewInt(int64(len(path))).Bytes(), 32).Hex()[2:]
	for _, addr := range pathAddresses {
		data += addr.Hex()[2:]
	}

	// 调用合约
	result, err := r.callContract(ctx, data)
	if err != nil {
		return nil, err
	}

	// 解析结果
	return r.parseAmountsOut(result)
}

// SwapExactTokensForTokens 精确代币换代币
func (r *UniswapV2Router) SwapExactTokensForTokens(ctx context.Context, params SwapParams, path []string) (*SwapResult, error) {
	// 构建交易数据
	// method: swapExactTokensForTokens(uint amountIn, uint amountOutMin, address[] path, address to, uint deadline)

	data := "0x38ed1739"
	data += common.LeftPadBytes(params.AmountIn.Bytes(), 32).Hex()[2:]
	data += common.LeftPadBytes(params.MinAmountOut.Bytes(), 32).Hex()[2:]
	data += common.LeftPadBytes(big.NewInt(int64(len(path))).Bytes(), 32).Hex()[2:]
	for _, p := range path {
		data += common.HexToAddress(p).Hex()[2:]
	}
	data += common.HexToAddress(params.To).Hex()[2:]
	data += common.LeftPadBytes(big.NewInt(int64(params.Deadline)).Bytes(), 32).Hex()[2:]

	// 发送交易
	txHash, err := r.sendTransaction(ctx, data)
	if err != nil {
		return nil, err
	}

	return &SwapResult{
		TxHash: txHash,
	}, nil
}

func (r *UniswapV2Router) callContract(ctx context.Context, data string) (string, error) {
	// 简化实现
	return "", nil
}

func (r *UniswapV2Router) parseAmountsOut(result string) ([]*big.Int, error) {
	// 解析返回数据
	return []*big.Int{big.NewInt(0)}, nil
}

func (r *UniswapV2Router) sendTransaction(ctx context.Context, data string) (string, error) {
	// 发送交易
	return "0x" + hex.EncodeToString([]byte("mock")), nil
}

// UniswapV3Router Uniswap V3 Router
type UniswapV3Router struct {
	client  *Client
	address common.Address
}

// Uniswap V3 Router 地址
var V3RouterAddresses = map[ChainID]string{
	Ethereum: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
	Polygon:  "0xE592427A0AEce92De3Edee1F18E0157C05861564",
	Arbitrum: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
	Optimism: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
}

// NewUniswapV3Router 创建 Uniswap V3 Router
func NewUniswapV3Router(client *Client) *UniswapV3Router {
	address, ok := V3RouterAddresses[client.ChainID]
	if !ok {
		address = V3RouterAddresses[Ethereum]
	}
	return &UniswapV3Router{
		client:  client,
		address: common.HexToAddress(address),
	}
}

// ExactInputSingleParams V3 精确输入参数
type ExactInputSingleParams struct {
	TokenIn       common.Address
	TokenOut      common.Address
	Fee           uint32
	Recipient     common.Address
	Deadline      uint256.Int
	AmountIn      uint256.Int
	AmountOutMinimum uint256.Int
	SqrtPriceLimitX96 uint256.Int
}

// ExactInputSingle V3 精确输入交换
func (r *UniswapV3Router) ExactInputSingle(ctx context.Context, params ExactInputSingleParams) (*SwapResult, error) {
	// method: 0x414bf389 exactInputSingle
	// 实现略

	return &SwapResult{
		TxHash: "0xmock",
	}, nil
}
