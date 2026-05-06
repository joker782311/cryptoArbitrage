package strategy

import (
	"sync"
	"time"
)

// SpotFutureStrategy 期现套利策略
type SpotFutureStrategy struct {
	mu        sync.RWMutex
	exchange  string // 交易所名称
	pairs     []string
	minBasis  float64                // 最小基差
	maxAmount float64
	prices    map[string]PricePair // symbol -> {spot, future}
	opportunityHandler func(*Opportunity)
}

// PricePair 现货/期货价格对
type PricePair struct {
	Spot   float64
	Future float64
}

// NewSpotFutureStrategy 创建期现套利策略
func NewSpotFutureStrategy(
	exchange string,
	pairs []string,
	minBasis float64,
	maxAmount float64,
) *SpotFutureStrategy {
	return &SpotFutureStrategy{
		exchange:  exchange,
		pairs:     pairs,
		minBasis:  minBasis,
		maxAmount: maxAmount,
		prices:    make(map[string]PricePair),
	}
}

// SetOpportunityHandler 设置机会回调
func (s *SpotFutureStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdatePrice 更新价格
func (s *SpotFutureStrategy) UpdatePrice(symbol string, spotPrice, futurePrice float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prices[symbol] = PricePair{
		Spot:   spotPrice,
		Future: futurePrice,
	}

	// 检查套利机会
	if opp := s.CheckOpportunity(symbol); opp != nil && s.opportunityHandler != nil {
		s.opportunityHandler(opp)
	}
}

// CheckOpportunity 检查套利机会
func (s *SpotFutureStrategy) CheckOpportunity(symbol string) *Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pair, ok := s.prices[symbol]
	if !ok || pair.Spot == 0 || pair.Future == 0 {
		return nil
	}

	// 基差 = (期货 - 现货) / 现货
	basis := (pair.Future - pair.Spot) / pair.Spot * 100

	if basis >= s.minBasis {
		// 买入现货，卖出期货
		quantity := s.maxAmount / pair.Spot

		// 毛利润
		grossProfit := (pair.Future - pair.Spot) * quantity

		// 手续费 (现货 + 期货)
		feeSpot := s.maxAmount * 0.001
		feeFuture := s.maxAmount * 0.001

		// 资金成本 (假设持有 1 天)
		capitalCost := s.maxAmount * 0.0001 // 万分之一的日资金成本

		opp := &Opportunity{
			ID:           generateID("sf", s.exchange, symbol),
			StrategyType: "spot_future",
			Timestamp:    time.Now().UnixMilli(),
			ExchangeA:    s.exchange,
			Symbol:       symbol,
			PriceA:       pair.Spot,
			PriceB:       pair.Future,
			ProfitRate:   basis,
			ProfitAmount: grossProfit,
			EstimatedGas: feeSpot + feeFuture + capitalCost,
			Slippage:     s.maxAmount * 0.0005,
			Legs: []Leg{
				{ID: 1, Exchange: s.exchange, Symbol: symbol, Side: "buy", Quantity: quantity, Price: pair.Spot},
				{ID: 2, Exchange: s.exchange + "_futures", Symbol: symbol, Side: "sell", Quantity: quantity, Price: pair.Future},
			},
		}
		opp.CalculateNetProfit()

		return opp
	}

	return nil
}

// GetConfig 获取策略配置
func (s *SpotFutureStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":       "spot_future",
		"exchange":   s.exchange,
		"pairs":      s.pairs,
		"min_basis":  s.minBasis,
		"max_amount": s.maxAmount,
	}
}
