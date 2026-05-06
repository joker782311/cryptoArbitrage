package evm

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// CurvePool Curve 池子
type CurvePool struct {
	client     *Client
	address    common.Address
	coins      []string
	coinCount  int
}

// Curve Factory 地址
var CurveFactoryAddresses = map[ChainID]string{
	Ethereum: "0xB9fC157394Af804a3578134A6585C0dc9cc990d4",
	Polygon:  "0x722272D36ef0Da72FF51c5A65Db7b870E2e8D4ee",
	Arbitrum: "0xb17b674D9c5CB2e441F8e196a2f048A81355d031",
}

// NewCurvePool 创建 Curve 池子
func NewCurvePool(client *Client, poolAddress string) *CurvePool {
	return &CurvePool{
		client:  client,
		address: common.HexToAddress(poolAddress),
	}
}

// GetPoolInfo 获取池子信息
func (p *CurvePool) GetPoolInfo(ctx context.Context) (coins []string, err error) {
	// 调用 pool.coins() 或从合约读取
	// 简化实现
	return []string{}, nil
}

// GetBalance 获取池子中某个代币的余额
func (p *CurvePool) GetBalance(ctx context.Context, coinIndex int) (*big.Int, error) {
	// 调用 pool.balances(uint256)
	return big.NewInt(0), nil
}

// GetVirtualPrice 获取虚拟价格
func (p *CurvePool) GetVirtualPrice(ctx context.Context) (*big.Int, error) {
	// 调用 pool.get_virtual_price()
	return big.NewInt(0), nil
}

// GetDy 计算交换输出
func (p *CurvePool) GetDy(ctx context.Context, i, j int, dx *big.Int) (*big.Int, error) {
	// 调用 pool.get_dy(int128 i, int128 j, uint256 dx)
	return big.NewInt(0), nil
}

// Exchange 执行交换
func (p *CurvePool) Exchange(ctx context.Context, i, j int, dx, minDy *big.Int) (*SwapResult, error) {
	// 调用 pool.exchange(int128 i, int128 j, uint256 dx, uint256 min_dy)
	return &SwapResult{
		TxHash: "0xmock",
	}, nil
}
