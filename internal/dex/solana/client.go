package solana

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Client Solana 客户端
type Client struct {
	RPC    *rpc.Client
	WS     *rpc.Streamer
	Endpoint string
	WSEndpoint string
}

// NewClient 创建 Solana 客户端
func NewClient(endpoint, wsEndpoint string) (*Client, error) {
	if endpoint == "" {
		endpoint = rpc.MainNetBeta_RPC
	}
	if wsEndpoint == "" {
		wsEndpoint = "wss://api.mainnet-beta.solana.com"
	}

	rpcClient := rpc.New(endpoint)
	wsClient, err := rpc.NewStreamer(wsEndpoint, rpc.StreamerOpts{})
	if err != nil {
		return nil, err
	}

	return &Client{
		RPC:      rpcClient,
		WS:       wsClient,
		Endpoint: endpoint,
		WSEndpoint: wsEndpoint,
	}, nil
}

// GetSlot 获取当前 Slot
func (c *Client) GetSlot(ctx context.Context) (uint64, error) {
	slot, err := c.RPC.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return slot, nil
}

// GetBalance 获取账户余额（SOL）
func (c *Client) GetBalance(ctx context.Context, account solana.PublicKey) (uint64, error) {
	balance, err := c.RPC.GetBalance(ctx, account, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}
	return balance.Value, nil
}

// GetTokenBalance 获取 SPL Token 余额
func (c *Client) GetTokenBalance(ctx context.Context, account, mint solana.PublicKey) (uint64, error) {
	// 获取账户的 Token Account
	tokenAccounts, err := c.RPC.GetTokenAccountsByOwner(ctx, account, rpc.TokenFilter{Mint: mint}, nil)
	if err != nil {
		return 0, err
	}

	if len(tokenAccounts) == 0 {
		return 0, fmt.Errorf("no token account found")
	}

	// 获取第一个 Token Account 的余额
	tokenAccount := tokenAccounts[0].Pubkey
	balance, err := c.RPC.GetTokenAccountBalance(ctx, tokenAccount, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}

	amount, err := balance.Value.GetAmount()
	if err != nil {
		return 0, err
	}

	return amount, nil
}

// GetRecentBlockhash 获取最新区块哈希
func (c *Client) GetRecentBlockhash(ctx context.Context) (solana.Hash, error) {
	recent, err := c.RPC.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Hash{}, err
	}
	return recent.Value.Blockhash, nil
}

// GetAccountInfo 获取账户信息
func (c *Client) GetAccountInfo(ctx context.Context, account solana.PublicKey) (*rpc.GetAccountInfoResult, error) {
	info, err := c.RPC.GetAccountInfo(ctx, account)
	if err != nil {
		return nil, err
	}
	return info, nil
}

// GetTransaction 获取交易详情
func (c *Client) GetTransaction(ctx context.Context, signature solana.Signature) (*rpc.GetTransactionResult, error) {
	tx, err := c.RPC.GetTransaction(ctx, signature, &rpc.GetTransactionOpts{
		Encoding:               solana.EncodingBase64,
		Commitment:             rpc.CommitmentFinalized,
		MaxSupportedTransactionVersion: solana.Uint64Ptr(0),
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// SendTransaction 发送交易
func (c *Client) SendTransaction(ctx context.Context, tx *solana.Transaction) (solana.Signature, error) {
	_, err := c.RPC.SendTransaction(ctx, tx)
	if err != nil {
		return solana.Signature{}, err
	}
	return tx.Signatures[0], nil
}

// ConfirmTransaction 确认交易
func (c *Client) ConfirmTransaction(ctx context.Context, signature solana.Signature, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sigSub, err := c.WS.SignatureSubscribe(
		signature,
		rpc.SignatureSubscribeOpts{
			Commitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return err
	}
	defer sigSub.Unsubscribe()

	result, err := sigSub.Recv(ctx)
	if err != nil {
		return err
	}

	if result.Value.Err != nil {
		return fmt.Errorf("transaction failed: %v", result.Value.Err)
	}

	return nil
}

// GetEpochInfo 获取 Epoch 信息
func (c *Client) GetEpochInfo(ctx context.Context) (*rpc.EpochInfo, error) {
	return c.RPC.GetEpochInfo(ctx, rpc.CommitmentFinalized)
}

// GetHealth 获取节点健康状态
func (c *Client) GetHealth(ctx context.Context) error {
	return c.RPC.GetHealth(ctx)
}
