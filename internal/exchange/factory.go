package exchange

import (
	"fmt"
	"github.com/joker782311/cryptoArbitrage/internal/exchange/binance"
	"github.com/joker782311/cryptoArbitrage/internal/exchange/bitget"
	"github.com/joker782311/cryptoArbitrage/internal/exchange/okx"
)

// ExchangeFactory 交易所工厂
type ExchangeFactory struct {
	exchanges map[string]Exchange
}

// NewExchangeFactory 创建交易所工厂
func NewExchangeFactory() *ExchangeFactory {
	return &ExchangeFactory{
		exchanges: make(map[string]Exchange),
	}
}

// Register 注册交易所
func (f *ExchangeFactory) Register(name string, ex Exchange) {
	f.exchanges[name] = ex
}

// Get 获取交易所
func (f *ExchangeFactory) Get(name string) (Exchange, error) {
	ex, ok := f.exchanges[name]
	if !ok {
		return nil, fmt.Errorf("exchange not found: %s", name)
	}
	return ex, nil
}

// GetAll 获取所有交易所
func (f *ExchangeFactory) GetAll() map[string]Exchange {
	return f.exchanges
}

// CreateExchange 创建交易所实例
func CreateExchange(name, apiKey, secretKey, passphrase string) (Exchange, error) {
	switch name {
	case "binance", "binance_futures":
		return binance.NewClient(apiKey, secretKey), nil
	case "okx":
		return okx.NewClient(apiKey, secretKey, passphrase), nil
	case "bitget":
		return bitget.NewClient(apiKey, secretKey, passphrase), nil
	default:
		return nil, fmt.Errorf("unsupported exchange: %s", name)
	}
}

// SupportedExchanges 支持的交易所列表
func SupportedExchanges() []string {
	return []string{"binance", "okx", "bitget"}
}
