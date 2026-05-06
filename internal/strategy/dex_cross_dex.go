package strategy

import (
	"sync"
	"time"
)

// DEXCrossDEXStrategy 跨 DEX 套利策略
type DEXCrossDEXStrategy struct {
	mu         sync.RWMutex
	dexes      map[string]interface{} // DEX 实例
	tokens     []string               // 监控的代币
	minProfit  float64
	maxAmount  float64
	prices     map[string]map[string]PriceInfo // dex -> token -> price
	opportunityHandler func(*Opportunity)
}

// PriceInfo DEX 价格信息
type PriceInfo struct {
	Price    float64
	Pool     string
	Reserve0 float64
	Reserve1 float64
}

// NewDEXCrossDEXStrategy 创建跨 DEX 套利策略
func NewDEXCrossDEXStrategy(
	dexes map[string]interface{},
	tokens []string,
	minProfit float64,
	maxAmount float64,
) *DEXCrossDEXStrategy {
	return &DEXCrossDEXStrategy{
		dexes:     dexes,
		tokens:    tokens,
		minProfit: minProfit,
		maxAmount: maxAmount,
		prices:    make(map[string]map[string]PriceInfo),
	}
}

// SetOpportunityHandler 设置机会回调
func (s *DEXCrossDEXStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdatePrice 更新价格
func (s *DEXCrossDEXStrategy) UpdatePrice(dex, token string, price PriceInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.prices[dex]; !ok {
		s.prices[dex] = make(map[string]PriceInfo)
	}
	s.prices[dex][token] = price

	// 检查套利机会
	if opp := s.CheckOpportunity(token); opp != nil && s.opportunityHandler != nil {
		s.opportunityHandler(opp)
	}
}

// CheckOpportunity 检查套利机会
func (s *DEXCrossDEXStrategy) CheckOpportunity(token string) *Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var bestOpp *Opportunity

	// 比较不同 DEX 的价格
	dexList := make([]string, 0, len(s.prices))
	for dex := range s.prices {
		dexList = append(dexList, dex)
	}

	for i := 0; i < len(dexList); i++ {
		for j := i + 1; j < len(dexList); j++ {
			dexA, dexB := dexList[i], dexList[j]

			priceA, okA := s.prices[dexA][token]
			priceB, okB := s.prices[dexB][token]

			if !okA || !okB || priceA.Price == 0 || priceB.Price == 0 {
				continue
			}

			// 计算价差
			spread := (priceB.Price - priceA.Price) / priceA.Price * 100

			if spread >= s.minProfit {
				grossProfit := s.maxAmount * spread / 100

				// DEX Gas 费用 (估算)
				gasFee := 50.0 // 假设 $50 Gas

				// 滑点
				slippage := s.maxAmount * 0.005

				opp := &Opportunity{
					ID:           generateID("dex", dexA, dexB, token),
					StrategyType: "dex_cross_dex",
					Timestamp:    time.Now().UnixMilli(),
					ExchangeA:    dexA,
					ExchangeB:    dexB,
					Symbol:       token,
					PriceA:       priceA.Price,
					PriceB:       priceB.Price,
					PoolA:        priceA.Pool,
					PoolB:        priceB.Pool,
					ProfitRate:   spread,
					ProfitAmount: grossProfit,
					EstimatedGas: gasFee,
					Slippage:     slippage,
					Legs: []Leg{
						{ID: 1, Exchange: dexA, Symbol: token, Side: "buy", Price: priceA.Price},
						{ID: 2, Exchange: dexB, Symbol: token, Side: "sell", Price: priceB.Price},
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

// GetConfig 获取策略配置
func (s *DEXCrossDEXStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":       "dex_cross_dex",
		"dexes":      len(s.dexes),
		"tokens":     s.tokens,
		"min_profit": s.minProfit,
		"max_amount": s.maxAmount,
	}
}
