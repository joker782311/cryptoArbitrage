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
	RPC        *rpc.Client
	Endpoint   string
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

	return &Client{
		RPC:        rpcClient,
		Endpoint:   endpoint,
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
	// 简化实现 - 实际使用需要正确的 API 调用
	return 0, fmt.Errorf("not implemented")
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
		Encoding:                       solana.EncodingBase64,
		Commitment:                     rpc.CommitmentFinalized,
		MaxSupportedTransactionVersion: uint64Ptr(0),
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

	// 轮询检查交易状态
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for transaction")
		default:
			sigStatus, err := c.RPC.GetSignatureStatuses(ctx, true, signature)
			if err != nil {
				return err
			}
			if len(sigStatus.Value) > 0 && sigStatus.Value[0] != nil {
				status := string(sigStatus.Value[0].ConfirmationStatus)
				if status == "finalized" || status == "confirmed" {
					return nil
				}
			}
			time.Sleep(time.Second)
		}
	}
}

// GetEpochInfo 获取 Epoch 信息
func (c *Client) GetEpochInfo(ctx context.Context) (*rpc.GetEpochInfoResult, error) {
	return c.RPC.GetEpochInfo(ctx, rpc.CommitmentFinalized)
}

// GetHealth 获取节点健康状态
func (c *Client) GetHealth(ctx context.Context) (string, error) {
	return c.RPC.GetHealth(ctx)
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}
