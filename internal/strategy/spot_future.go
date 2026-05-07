package strategy

import (
	"context"
	"sync"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/exchange"
)

// SpotFutureStrategy 期现套利策略
type SpotFutureStrategy struct {
	mu                 sync.RWMutex
	exchangeName       string
	spotExchange       exchange.Exchange
	futuresPricer      exchange.FuturesPricer
	pairs              []string
	minBasis           float64
	maxAmount          float64
	prices             map[string]PricePair
	opportunityHandler func(*Opportunity)
}

// PricePair 现货/期货价格对
type PricePair struct {
	Spot   float64
	Future float64
}

// NewSpotFutureStrategy 创建期现套利策略
func NewSpotFutureStrategy(
	exchangeName string,
	spotExchange exchange.Exchange,
	futuresPricer exchange.FuturesPricer,
	pairs []string,
	minBasis float64,
	maxAmount float64,
) *SpotFutureStrategy {
	return &SpotFutureStrategy{
		exchangeName:  exchangeName,
		spotExchange:  spotExchange,
		futuresPricer: futuresPricer,
		pairs:         pairs,
		minBasis:      minBasis,
		maxAmount:     maxAmount,
		prices:        make(map[string]PricePair),
	}
}

// SetOpportunityHandler 设置机会回调
func (s *SpotFutureStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdatePrice 更新价格并检查套利机会
func (s *SpotFutureStrategy) UpdatePrice(symbol string, spotPrice, futurePrice float64) {
	s.mu.Lock()
	s.prices[symbol] = PricePair{Spot: spotPrice, Future: futurePrice}
	s.mu.Unlock()

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
	if basis < s.minBasis {
		return nil
	}

	quantity := s.maxAmount / pair.Spot
	grossProfit := (pair.Future - pair.Spot) * quantity
	feeSpot := s.maxAmount * 0.001
	feeFuture := s.maxAmount * 0.001
	capitalCost := s.maxAmount * 0.0001

	opp := &Opportunity{
		ID:           generateID("sf", s.exchangeName, symbol),
		StrategyType: "spot_future",
		Timestamp:    time.Now().UnixMilli(),
		ExchangeA:    s.exchangeName,
		Symbol:       symbol,
		PriceA:       pair.Spot,
		PriceB:       pair.Future,
		ProfitRate:   basis,
		ProfitAmount: grossProfit,
		EstimatedGas: feeSpot + feeFuture + capitalCost,
		Slippage:     s.maxAmount * 0.0005,
		Legs: []Leg{
			{ID: 1, Exchange: s.exchangeName, Symbol: symbol, Side: "buy", Quantity: quantity, Price: pair.Spot},
			{ID: 2, Exchange: s.exchangeName + "_futures", Symbol: symbol, Side: "sell", Quantity: quantity, Price: pair.Future},
		},
	}
	opp.CalculateNetProfit()
	return opp
}

// Start 启动价格轮询
func (s *SpotFutureStrategy) Start(ctx context.Context) error {
	for _, pair := range s.pairs {
		go func(symbol string) {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					spotT, err := s.spotExchange.GetTicker(ctx, symbol)
					if err != nil {
						continue
					}
					futuresT, err := s.futuresPricer.GetFuturesTicker(ctx, symbol)
					if err != nil {
						continue
					}
					s.UpdatePrice(symbol, spotT.Price, futuresT.Price)
				}
			}
		}(pair)
	}
	return nil
}

// GetConfig 获取策略配置
func (s *SpotFutureStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":       "spot_future",
		"exchange":   s.exchangeName,
		"pairs":      s.pairs,
		"min_basis":  s.minBasis,
		"max_amount": s.maxAmount,
	}
}
