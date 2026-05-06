package strategy

import (
	"sync"
	"time"
)

// TriangularStrategy 三角套利策略
type TriangularStrategy struct {
	mu         sync.RWMutex
	exchange   string
	exchangeInstance interface{} // 交易所实例
	pairs      []TriPair
	minProfit  float64
	maxAmount  float64
	prices     map[string]float64 // symbol -> price
	opportunityHandler func(*Opportunity)
}

// TriPair 交易对
type TriPair struct {
	Base  string
	Quote string
}

// NewTriangularStrategy 创建三角套利策略
func NewTriangularStrategy(
	exchange string,
	pairs []TriPair,
	minProfit float64,
	maxAmount float64,
) *TriangularStrategy {
	return &TriangularStrategy{
		exchange:  exchange,
		pairs:     pairs,
		minProfit: minProfit,
		maxAmount: maxAmount,
		prices:    make(map[string]float64),
	}
}

// SetOpportunityHandler 设置机会回调
func (s *TriangularStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdatePrice 更新价格
func (s *TriangularStrategy) UpdatePrice(symbol string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prices[symbol] = price

	// 检查所有可能的三角路径
	s.checkTriangularPaths()
}

// checkTriangularPaths 检查三角套利路径
func (s *TriangularStrategy) checkTriangularPaths() {
	// 典型三角路径：USDT -> BTC -> ETH -> USDT
	// 需要三个交易对：BTC/USDT, ETH/BTC, ETH/USDT

	triPaths := [][]string{
		{"USDT", "BTC", "ETH", "USDT"},
		{"USDT", "ETH", "BTC", "USDT"},
		{"USDT", "BTC", "SOL", "USDT"},
		{"USDT", "ETH", "SOL", "USDT"},
	}

	for _, path := range triPaths {
		if opp := s.checkPath(path); opp != nil && s.opportunityHandler != nil {
			s.opportunityHandler(opp)
		}
	}
}

// checkPath 检查特定路径
func (s *TriangularStrategy) checkPath(path []string) *Opportunity {
	// 构建交易对
	pairs := make([]string, len(path)-1)
	for i := 0; i < len(path)-1; i++ {
		pairs[i] = path[i+1] + path[i] // 如 BTCUSDT
	}

	// 检查价格是否存在
	prices := make([]float64, len(pairs))
	for i, pair := range pairs {
		price, ok := s.prices[pair]
		if !ok || price == 0 {
			return nil
		}
		prices[i] = price
	}

	// 计算最终金额
	startAmount := s.maxAmount
	current := startAmount

	for i, price := range prices {
		if i == 0 {
			// USDT -> BTC: 用 USDT 买 BTC
			current = current / price
		} else if i == len(prices)-1 {
			// ETH -> USDT: 卖出 ETH 得到 USDT
			current = current * price
		} else {
			// BTC -> ETH: 用 BTC 买 ETH
			current = current / price
		}
	}

	profitRate := (current - startAmount) / startAmount * 100

	if profitRate >= s.minProfit {
		opp := &Opportunity{
			ID:           generateID("tri", s.exchange, path[0]),
			StrategyType: "triangular",
			Timestamp:    time.Now().UnixMilli(),
			ExchangeA:    s.exchange,
			Symbol:       path[0],
			ProfitRate:   profitRate,
			ProfitAmount: current - startAmount,
			EstimatedGas: s.maxAmount * 0.003, // 三次交易手续费
			Slippage:     s.maxAmount * 0.001,
			Path:         path,
		}
		opp.CalculateNetProfit()
		return opp
	}

	return nil
}

// GetConfig 获取策略配置
func (s *TriangularStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":       "triangular",
		"exchange":   s.exchange,
		"min_profit": s.minProfit,
		"max_amount": s.maxAmount,
	}
}
