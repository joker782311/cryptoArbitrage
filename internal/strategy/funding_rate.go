package strategy

import (
	"context"
	"sync"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/exchange"
)

// FundingRateStrategy 资金费率套利策略
type FundingRateStrategy struct {
	mu                 sync.RWMutex
	exchanges          map[string]exchange.Exchange
	pairs              []string
	minRateDiff        float64 // 最小费率差
	maxAmount          float64
	lastRates          map[string]map[string]float64 // exchange -> symbol -> rate
	opportunityHandler func(*Opportunity)
}

// NewFundingRateStrategy 创建资金费率套利策略
func NewFundingRateStrategy(
	exchanges map[string]exchange.Exchange,
	pairs []string,
	minRateDiff float64,
	maxAmount float64,
) *FundingRateStrategy {
	return &FundingRateStrategy{
		exchanges:   exchanges,
		pairs:       pairs,
		minRateDiff: minRateDiff,
		maxAmount:   maxAmount,
		lastRates:   make(map[string]map[string]float64),
	}
}

// SetOpportunityHandler 设置机会回调
func (s *FundingRateStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdateRate 更新资金费率
func (s *FundingRateStrategy) UpdateRate(exchangeName, symbol string, rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.lastRates[exchangeName]; !ok {
		s.lastRates[exchangeName] = make(map[string]float64)
	}
	s.lastRates[exchangeName][symbol] = rate

	// 检查套利机会
	if opp := s.CheckOpportunity(symbol); opp != nil && s.opportunityHandler != nil {
		s.opportunityHandler(opp)
	}
}

// CheckOpportunity 检查套利机会
func (s *FundingRateStrategy) CheckOpportunity(symbol string) *Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var bestOpp *Opportunity

	// 寻找资金费率差异
	// 策略：做空高费率币种，做多低费率币种
	for exA, ratesA := range s.lastRates {
		for exB, ratesB := range s.lastRates {
			if exA == exB {
				continue
			}

			rateA, okA := ratesA[symbol]
			rateB, okB := ratesB[symbol]

			if !okA || !okB {
				continue
			}

			// 费率差 (年化)
			rateDiff := rateA - rateB

			if rateDiff >= s.minRateDiff {
				// 资金费率套利收益 = 费率差 * 持仓金额
				// 假设每天结算 3 次 (8 小时一次)
				dailyRate := rateDiff * 3
				dailyProfit := s.maxAmount * dailyRate

				// 手续费成本 (开仓 + 平仓)
				feeCost := s.maxAmount * 0.002 // 0.1% * 2

				opp := &Opportunity{
					ID:           generateID("fr", exA, exB, symbol),
					StrategyType: "funding_rate",
					Timestamp:    time.Now().UnixMilli(),
					ExchangeA:    exA,
					ExchangeB:    exB,
					Symbol:       symbol,
					ProfitRate:   rateDiff * 100,
					ProfitAmount: dailyProfit,
					EstimatedGas: feeCost,
					Slippage:     s.maxAmount * 0.0005,
					Legs: []Leg{
						{ID: 1, Exchange: exA, Symbol: symbol, Side: "sell", Quantity: s.maxAmount, Price: 1},
						{ID: 2, Exchange: exB, Symbol: symbol, Side: "buy", Quantity: s.maxAmount, Price: 1},
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

func (s *FundingRateStrategy) Start(_ context.Context) error { return nil }

// GetConfig 获取策略配置
func (s *FundingRateStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":          "funding_rate",
		"exchanges":     len(s.exchanges),
		"pairs":         s.pairs,
		"min_rate_diff": s.minRateDiff,
		"max_amount":    s.maxAmount,
	}
}
