package risk

import (
	"errors"
	"sync"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/model"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
)

var (
	ErrDailyLossLimit    = errors.New("daily loss limit reached")
	ErrPositionLimit     = errors.New("position limit exceeded")
	ErrStrategyDisabled  = errors.New("strategy is disabled")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// Manager 风险管理器
type Manager struct {
	mu               sync.RWMutex
	maxPosition      float64                // 最大仓位 (USDT)
	dailyStopLoss    float64                // 日止损限额 (USDT)
	strategyLimits   map[string]StrategyLimit
	dailyPnL         float64
	dailyPnLDate     string // 格式：2024-01-15
	currentPositions map[string]*model.Position
	balances         map[string]float64 // asset -> balance
}

// StrategyLimit 策略限额配置
type StrategyLimit struct {
	MaxPosition  float64 // 该策略最大仓位
	MaxDailyLoss float64 // 该策略日最大亏损
	AutoExecute  bool    // 是否允许自动执行
	Enabled      bool    // 策略是否启用
}

// NewManager 创建风险管理器
func NewManager(maxPosition, dailyStopLoss float64) *Manager {
	m := &Manager{
		maxPosition:      maxPosition,
		dailyStopLoss:    dailyStopLoss,
		strategyLimits:   make(map[string]StrategyLimit),
		currentPositions: make(map[string]*model.Position),
		balances:         make(map[string]float64),
	}
	m.initStrategyLimits()
	m.loadDailyPnL()
	return m
}

// initStrategyLimits 初始化策略限额
func (m *Manager) initStrategyLimits() {
	// 默认配置，可以从数据库加载
	m.strategyLimits = map[string]StrategyLimit{
		"cross_exchange": {MaxPosition: 10000, MaxDailyLoss: 500, AutoExecute: true, Enabled: true},
		"funding_rate":   {MaxPosition: 20000, MaxDailyLoss: 1000, AutoExecute: true, Enabled: true},
		"spot_future":    {MaxPosition: 15000, MaxDailyLoss: 750, AutoExecute: false, Enabled: true},
		"triangular":     {MaxPosition: 5000, MaxDailyLoss: 250, AutoExecute: true, Enabled: true},
		"dex_cross_dex":  {MaxPosition: 3000, MaxDailyLoss: 300, AutoExecute: false, Enabled: true},
	}
}

// loadDailyPnL 加载当日盈亏
func (m *Manager) loadDailyPnL() {
	today := time.Now().Format("2006-01-02")
	m.dailyPnLDate = today

	// 从数据库查询当日订单计算盈亏
	var orders []model.Order
	database.DB.Where("DATE(created_at) = ?", today).Find(&orders)

	var pnl float64
	for _, order := range orders {
		if order.Status == "filled" {
			// 简化计算，实际需要更复杂的逻辑
			if order.Side == "sell" {
				pnl += order.Price * order.ExecutedQty
			} else {
				pnl -= order.Price * order.ExecutedQty
			}
		}
	}
	m.dailyPnL = pnl
}

// CanExecute 检查是否可以执行交易
func (m *Manager) CanExecute(opp *strategy.Opportunity) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查日期是否变更
	if time.Now().Format("2006-01-02") != m.dailyPnLDate {
		m.loadDailyPnL()
	}

	// 1. 检查日亏损限制
	if m.dailyPnL < -m.dailyStopLoss {
		return ErrDailyLossLimit
	}

	// 2. 检查策略是否启用
	limit, ok := m.strategyLimits[opp.StrategyType]
	if !ok || !limit.Enabled {
		return ErrStrategyDisabled
	}

	// 3. 检查策略是否允许自动执行
	if !limit.AutoExecute {
		return ErrStrategyDisabled
	}

	// 4. 检查仓位限制
	totalPosition := m.getTotalPosition()
	if totalPosition+opp.ProfitAmount > m.maxPosition {
		return ErrPositionLimit
	}

	// 5. 检查策略仓位限制
	strategyPosition := m.getStrategyPosition(opp.StrategyType)
	if strategyPosition+opp.ProfitAmount > limit.MaxPosition {
		return ErrPositionLimit
	}

	return nil
}

// CanExecuteManual 检查是否可以手动执行（只检查基本限制，不检查 AutoExecute）
func (m *Manager) CanExecuteManual(opp *strategy.Opportunity) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if time.Now().Format("2006-01-02") != m.dailyPnLDate {
		m.loadDailyPnL()
	}

	if m.dailyPnL < -m.dailyStopLoss {
		return ErrDailyLossLimit
	}

	limit, ok := m.strategyLimits[opp.StrategyType]
	if !ok || !limit.Enabled {
		return ErrStrategyDisabled
	}

	totalPosition := m.getTotalPosition()
	if totalPosition+opp.ProfitAmount > m.maxPosition {
		return ErrPositionLimit
	}

	return nil
}

// UpdatePosition 更新仓位
func (m *Manager) UpdatePosition(exchange, symbol string, position *model.Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := exchange + ":" + symbol
	m.currentPositions[key] = position
}

// UpdateBalance 更新余额
func (m *Manager) UpdateBalance(asset string, balance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[asset] = balance
}

// GetBalance 获取余额
func (m *Manager) GetBalance(asset string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.balances[asset]
}

// HasSufficientBalance 检查是否有足够余额
func (m *Manager) HasSufficientBalance(asset string, required float64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.balances[asset] >= required
}

// getTotalPosition 获取总仓位
func (m *Manager) getTotalPosition() float64 {
	var total float64
	for _, pos := range m.currentPositions {
		total += pos.EntryPrice * pos.Quantity
	}
	return total
}

// getStrategyPosition 获取策略仓位
func (m *Manager) getStrategyPosition(strategyType string) float64 {
	// 简化实现，实际需要根据订单关联策略
	var total float64
	for _, pos := range m.currentPositions {
		total += pos.EntryPrice * pos.Quantity
	}
	return total
}

// UpdateDailyPnL 更新日盈亏
func (m *Manager) UpdateDailyPnL(pnl float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dailyPnL += pnl
}

// GetDailyPnL 获取日盈亏
func (m *Manager) GetDailyPnL() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dailyPnL
}

// GetStats 获取风控统计
type Stats struct {
	TotalPosition    float64 `json:"total_position"`
	DailyPnL         float64 `json:"daily_pnl"`
	DailyPnLPercent  float64 `json:"daily_pnl_percent"`
	PositionUtilization float64 `json:"position_utilization"` // 仓位使用率
	RemainingLimit   float64 `json:"remaining_limit"`
}

func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalPosition := m.getTotalPosition()
	utilization := totalPosition / m.maxPosition * 100
	remainingLimit := m.maxPosition - totalPosition

	return &Stats{
		TotalPosition:     totalPosition,
		DailyPnL:          m.dailyPnL,
		PositionUtilization: utilization,
		RemainingLimit:    remainingLimit,
	}
}

// SetStrategyEnabled 设置策略启用状态
func (m *Manager) SetStrategyEnabled(name string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit, ok := m.strategyLimits[name]; ok {
		limit.Enabled = enabled
		m.strategyLimits[name] = limit
	}
}

// SetStrategyAutoExecute 设置策略自动执行
func (m *Manager) SetStrategyAutoExecute(name string, auto bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit, ok := m.strategyLimits[name]; ok {
		limit.AutoExecute = auto
		m.strategyLimits[name] = limit
	}
}

// GetStrategyLimit 获取策略限额配置
func (m *Manager) GetStrategyLimit(name string) *StrategyLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit, ok := m.strategyLimits[name]; ok {
		return &limit
	}
	return nil
}

// GetAllStrategyLimits 获取所有策略限额
func (m *Manager) GetAllStrategyLimits() map[string]StrategyLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]StrategyLimit)
	for k, v := range m.strategyLimits {
		result[k] = v
	}
	return result
}
