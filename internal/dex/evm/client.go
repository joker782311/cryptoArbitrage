package evm

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ChainID 链 ID
type ChainID uint64

const (
	Ethereum ChainID = 1
	BSC      ChainID = 56
	Polygon  ChainID = 137
	Arbitrum ChainID = 42161
	Optimism ChainID = 10
	Base     ChainID = 8453
)

// ChainInfo 链信息
type ChainInfo struct {
	ChainID   ChainID
	Name      string
	RPCURL    string
	Explorer  string
	NativeToken string
}

var Chains = map[ChainID]ChainInfo{
	Ethereum: {ChainID: Ethereum, Name: "Ethereum", RPCURL: "https://eth.llamarpc.com", Explorer: "https://etherscan.io", NativeToken: "ETH"},
	BSC:      {ChainID: BSC, Name: "BSC", RPCURL: "https://bsc-dataseed.binance.org", Explorer: "https://bscscan.com", NativeToken: "BNB"},
	Polygon:  {ChainID: Polygon, Name: "Polygon", RPCURL: "https://polygon-rpc.com", Explorer: "https://polygonscan.com", NativeToken: "MATIC"},
	Arbitrum: {ChainID: Arbitrum, Name: "Arbitrum", RPCURL: "https://arb1.arbitrum.io/rpc", Explorer: "https://arbiscan.io", NativeToken: "ETH"},
	Optimism: {ChainID: Optimism, Name: "Optimism", RPCURL: "https://mainnet.optimism.io", Explorer: "https://optimistic.etherscan.io", NativeToken: "ETH"},
	Base:     {ChainID: Base, Name: "Base", RPCURL: "https://mainnet.base.org", Explorer: "https://basescan.org", NativeToken: "ETH"},
}

// Client EVM 客户端
type Client struct {
	ChainID ChainID
	Info    ChainInfo
	Client  *ethclient.Client
}

// NewClient 创建 EVM 客户端
func NewClient(chainID ChainID, rpcURL ...string) (*Client, error) {
	info, ok := Chains[chainID]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %d", chainID)
	}

	url := info.RPCURL
	if len(rpcURL) > 0 && rpcURL[0] != "" {
		url = rpcURL[0]
	}

	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}

	return &Client{
		ChainID: chainID,
		Info:    info,
		Client:  client,
	}, nil
}

// GetBlockNumber 获取最新区块号
func (c *Client) GetBlockNumber(ctx context.Context) (uint64, error) {
	return c.Client.BlockNumber(ctx)
}

// GetBalance 获取账户余额
func (c *Client) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	addr := common.HexToAddress(address)
	return c.Client.BalanceAt(ctx, addr, nil)
}

// GetGasPrice 获取 Gas 价格
func (c *Client) GetGasPrice(ctx context.Context) (*big.Int, error) {
	return c.Client.SuggestGasPrice(ctx)
}

// GetTokenBalance 获取 ERC20 代币余额
func (c *Client) GetTokenBalance(ctx context.Context, tokenAddress, holderAddress string) (*big.Int, error) {
	tokenAddr := common.HexToAddress(tokenAddress)
	holderAddr := common.HexToAddress(holderAddress)

	// ERC20 balanceOf 方法签名
	data := common.Hex2Bytes("70a08231")
	data = append(data, holderAddr.Bytes()...)

	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}

	result, err := c.Client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(result), nil
}

// WaitForTransaction 等待交易确认
func (c *Client) WaitForTransaction(ctx context.Context, txHash string, timeout time.Duration) (*types.Receipt, error) {
	hash := common.HexToHash(txHash)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			receipt, err := c.Client.TransactionReceipt(ctx, hash)
			if err == nil {
				return receipt, nil
			}
		}
	}
}

// GetChainID 获取链 ID
func (c *Client) GetChainID(ctx context.Context) (*big.Int, error) {
	return c.Client.ChainID(ctx)
}

// GetNonce 获取账户 Nonce
func (c *Client) GetNonce(ctx context.Context, address string) (uint64, error) {
	addr := common.HexToAddress(address)
	return c.Client.PendingNonceAt(ctx, addr)
}
