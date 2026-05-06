package strategy

import (
	"context"
	"sync"

	"github.com/joker782311/cryptoArbitrage/internal/exchange"
)

// Engine 策略引擎
type Engine struct {
	mu               sync.RWMutex
	strategies       []Strategy
	opportunityChan  chan *Opportunity
	exchanges        map[string]exchange.Exchange
	isRunning        bool
	ctx              context.Context
	cancel           context.CancelFunc
}

// Strategy 策略接口
type Strategy interface {
	GetConfig() map[string]interface{}
}

// OpportunityHandler 机会处理函数
type OpportunityHandler func(*Opportunity)

// NewEngine 创建策略引擎
func NewEngine(exchanges map[string]exchange.Exchange) *Engine {
	return &Engine{
		strategies:      make([]Strategy, 0),
		opportunityChan: make(chan *Opportunity, 100),
		exchanges:       exchanges,
	}
}

// AddStrategy 添加策略
func (e *Engine) AddStrategy(s Strategy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.strategies = append(e.strategies, s)
}

// GetStrategies 获取所有策略配置
func (e *Engine) GetStrategies() []map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	configs := make([]map[string]interface{}, len(e.strategies))
	for i, s := range e.strategies {
		configs[i] = s.GetConfig()
	}
	return configs
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	if e.isRunning {
		e.mu.Unlock()
		return nil
	}

	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.isRunning = true
	e.mu.Unlock()

	// 启动机会处理协程
	go e.processOpportunities()

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return nil
	}

	e.cancel()
	e.isRunning = false

	return nil
}

// processOpportunities 处理套利机会
func (e *Engine) processOpportunities() {
	for {
		select {
		case <-e.ctx.Done():
			return
		case opp := <-e.opportunityChan:
			// 这里可以将机会发送到执行引擎
			// 或者存储到数据库
			handleOpportunity(opp)
		}
	}
}

// handleOpportunity 处理单个机会
func handleOpportunity(opp *Opportunity) {
	// 可以将机会记录到日志或数据库
	// 或者发送到前端
}

// EmitOpportunity 发送机会到通道
func (e *Engine) EmitOpportunity(opp *Opportunity) {
	select {
	case e.opportunityChan <- opp:
	default:
		// 通道已满，丢弃
	}
}
