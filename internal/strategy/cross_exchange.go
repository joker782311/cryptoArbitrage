package strategy

import (
	"context"
	"sync"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/exchange"
)

// CrossExchangeStrategy 跨交易所套利策略
type CrossExchangeStrategy struct {
	mu         sync.RWMutex
	exchanges  map[string]exchange.Exchange
	pairs      []string                // 监控的交易对
	minProfit  float64                 // 最小利润率
	maxAmount  float64                 // 单笔最大金额
	lastPrices map[string]map[string]float64 // exchange -> symbol -> price
	opportunityHandler func(*Opportunity)
}

// NewCrossExchangeStrategy 创建跨交易所套利策略
func NewCrossExchangeStrategy(
	exchanges map[string]exchange.Exchange,
	pairs []string,
	minProfit float64,
	maxAmount float64,
) *CrossExchangeStrategy {
	return &CrossExchangeStrategy{
		exchanges:  exchanges,
		pairs:      pairs,
		minProfit:  minProfit,
		maxAmount:  maxAmount,
		lastPrices: make(map[string]map[string]float64),
	}
}

// SetOpportunityHandler 设置机会回调
func (s *CrossExchangeStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdatePrice 更新价格
func (s *CrossExchangeStrategy) UpdatePrice(exchangeName, symbol string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.lastPrices[exchangeName]; !ok {
		s.lastPrices[exchangeName] = make(map[string]float64)
	}
	s.lastPrices[exchangeName][symbol] = price

	// 价格更新后检查套利机会
	if opp := s.CheckOpportunity(symbol); opp != nil && s.opportunityHandler != nil {
		s.opportunityHandler(opp)
	}
}

// CheckOpportunity 检查套利机会
func (s *CrossExchangeStrategy) CheckOpportunity(symbol string) *Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var bestOpp *Opportunity

	// 遍历所有交易所对，寻找套利机会
	for exA, pricesA := range s.lastPrices {
		for exB, pricesB := range s.lastPrices {
			if exA == exB {
				continue
			}

			priceA, okA := pricesA[symbol]
			priceB, okB := pricesB[symbol]

			if !okA || !okB || priceA == 0 || priceB == 0 {
				continue
			}

			// 计算价差 (假设在 A 买入，在 B 卖出)
			spread := (priceB - priceA) / priceA * 100

			if spread >= s.minProfit {
				// 计算预估利润和手续费
				amount := s.maxAmount
				grossProfit := amount * spread / 100

				// 估算手续费 (假设每个交易所 0.1%)
				feeA := amount * 0.001
				feeB := amount * 0.001
				estimatedGas := feeA + feeB

				// 估算滑点 (0.05%)
				slippage := amount * 0.0005

				opp := &Opportunity{
					ID:           generateID("ce", exA, exB, symbol),
					StrategyType: "cross_exchange",
					Timestamp:    time.Now().UnixMilli(),
					ExchangeA:    exA,
					ExchangeB:    exB,
					Symbol:       symbol,
					PriceA:       priceA,
					PriceB:       priceB,
					ProfitRate:   spread,
					ProfitAmount: grossProfit,
					EstimatedGas: estimatedGas,
					Slippage:     slippage,
					Legs: []Leg{
						{ID: 1, Exchange: exA, Symbol: symbol, Side: "buy", Quantity: amount / priceA, Price: priceA},
						{ID: 2, Exchange: exB, Symbol: symbol, Side: "sell", Quantity: amount / priceA, Price: priceB},
					},
				}
				opp.CalculateNetProfit()

				if bestOpp == nil || opp.NetProfit > bestOpp.NetProfit {
					bestOpp = opp
				}
			}
		}
	}

	return bestOpp
}

// Start 启动策略监控
func (s *CrossExchangeStrategy) Start(ctx context.Context) error {
	// 订阅各个交易所的行情
	for exName, ex := range s.exchanges {
		for _, pair := range s.pairs {
			go func(name, symbol string) {
				// 这里应该使用 WebSocket 订阅
				// 简化实现：定期轮询
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						t, err := ex.GetTicker(ctx, symbol)
						if err != nil {
							continue
						}
						s.UpdatePrice(name, symbol, t.Price)
					}
				}
			}(exName, pair)
		}
	}

	return nil
}

// GetConfig 获取策略配置
func (s *CrossExchangeStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":       "cross_exchange",
		"exchanges":  len(s.exchanges),
		"pairs":      s.pairs,
		"min_profit": s.minProfit,
		"max_amount": s.maxAmount,
	}
}
