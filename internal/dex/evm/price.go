package evm

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// PriceResult 价格结果
type PriceResult struct {
	Token0  string  `json:"token0"`
	Token1  string  `json:"token1"`
	Price   float64 `json:"price"`
	Pool    string  `json:"pool"`
	Dex     string  `json:"dex"`
	Chain   string  `json:"chain"`
}

// TokenInfo 代币信息
type TokenInfo struct {
	Address  string
	Symbol   string
	Name     string
	Decimals uint8
}

// GetTokenPrice 获取代币价格（以 USDT 计）
func (c *Client) GetTokenPrice(ctx context.Context, tokenAddress, quoteToken string) (*PriceResult, error) {
	// 通过 Uniswap V3 获取价格
	return c.getUniswapV3Price(ctx, tokenAddress, quoteToken)
}

// getUniswapV3Price 通过 Uniswap V3 获取价格
func (c *Client) getUniswapV3Price(ctx context.Context, tokenAddress, quoteToken string) (*PriceResult, error) {
	// 这里需要调用 Uniswap V3 Pool 合约
	// 简化实现，实际需要使用 abigen 生成的 binding

	poolAddress, err := c.getPoolAddress(tokenAddress, quoteToken)
	if err != nil {
		return nil, err
	}

	// 获取池子价格
	sqrtPriceX96, err := c.getSlot0(ctx, poolAddress)
	if err != nil {
		return nil, err
	}

	// 计算价格 = (sqrtPriceX96 / 2^96)^2
	price := c.calculatePriceFromSqrtPriceX96(sqrtPriceX96)

	return &PriceResult{
		Token0: tokenAddress,
		Token1: quoteToken,
		Price:  price,
		Pool:   poolAddress,
		Dex:    "uniswap_v3",
		Chain:  c.Info.Name,
	}, nil
}

// getPoolAddress 获取池子地址
func (c *Client) getPoolAddress(tokenA, tokenB string) (string, error) {
	// 需要使用 Uniswap V3 Factory 合约
	// 这里返回一个模拟地址
	return "0x0000000000000000000000000000000000000000", nil
}

// getSlot0 获取池子 slot0（包含 sqrtPriceX96）
func (c *Client) getSlot0(ctx context.Context, poolAddress string) (*big.Int, error) {
	// 调用 pool.slot0()
	// 简化实现
	return big.NewInt(1), nil
}

// calculatePriceFromSqrtPriceX96 从 sqrtPriceX96 计算价格
func (c *Client) calculatePriceFromSqrtPriceX96(sqrtPriceX96 *big.Int) float64 {
	// price = (sqrtPriceX96 / 2^96)^2
	// 简化计算
	return 1.0
}

// GetTokenInfo 获取代币信息
func (c *Client) GetTokenInfo(ctx context.Context, tokenAddress string) (*TokenInfo, error) {
	tokenAddr := common.HexToAddress(tokenAddress)

	// 调用 ERC20 合约方法
	symbol, err := c.callStringMethod(ctx, tokenAddr, "0x95d89b41") // symbol()
	if err != nil {
		return nil, err
	}

	name, err := c.callStringMethod(ctx, tokenAddr, "0x06fdde03") // name()
	if err != nil {
		return nil, err
	}

	decimals, err := c.callUint8Method(ctx, tokenAddr, "0x313ce567") // decimals()
	if err != nil {
		return nil, err
	}

	return &TokenInfo{
		Address:  tokenAddress,
		Symbol:   symbol,
		Name:     name,
		Decimals: decimals,
	}, nil
}

func (c *Client) callStringMethod(ctx context.Context, contract common.Address, data string) (string, error) {
	// 简化实现
	return "", nil
}

func (c *Client) callUint8Method(ctx context.Context, contract common.Address, data string) (uint8, error) {
	// 简化实现
	return 0, nil
}

// GetReserves 获取池子储备量
func (c *Client) GetReserves(ctx context.Context, poolAddress string) (reserve0, reserve1 *big.Int, err error) {
	// 调用 Uniswap V2/V3 Pool 的 getReserves 或 slot0
	return big.NewInt(0), big.NewInt(0), nil
}
