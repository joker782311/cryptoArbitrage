package order

import (
	"context"
	"fmt"
	"sync"

	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/exchange"
	"github.com/joker782311/cryptoArbitrage/internal/model"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
)

// Manager 订单管理器
type Manager struct {
	mu        sync.RWMutex
	exchanges map[string]exchange.Exchange
	orderMap  map[string]*model.Order  // orderID -> Order
	pending   map[string]*strategy.Leg // 待执行的交易腿
}

// NewManager 创建订单管理器
func NewManager(exchanges map[string]exchange.Exchange) *Manager {
	return &Manager{
		exchanges: exchanges,
		orderMap:  make(map[string]*model.Order),
		pending:   make(map[string]*strategy.Leg),
	}
}

// ExecuteOrder 执行订单
func (m *Manager) ExecuteOrder(ctx context.Context, leg *strategy.Leg) (*model.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ex, ok := m.exchanges[leg.Exchange]
	if !ok {
		return nil, fmt.Errorf("unknown exchange: %s", leg.Exchange)
	}

	// 调用交易所下单
	order, err := ex.PlaceOrder(ctx, leg.Symbol, leg.Side, "market", leg.Quantity, leg.Price)
	if err != nil {
		return nil, err
	}

	// 保存到数据库
	dbOrder := &model.Order{
		StrategyID:  0, // TODO: 从策略 ID 传入
		Exchange:    leg.Exchange,
		Symbol:      leg.Symbol,
		Side:        leg.Side,
		Type:        "market",
		Price:       order.Price,
		Quantity:    order.Quantity,
		ExecutedQty: order.ExecutedQty,
		Status:      order.Status,
		OrderID:     order.OrderID,
	}

	if err := database.DB.Create(dbOrder).Error; err != nil {
		return nil, err
	}

	// 缓存订单
	m.orderMap[dbOrder.OrderID] = dbOrder
	leg.OrderID = dbOrder.OrderID
	leg.Status = order.Status

	return dbOrder, nil
}

// ExecuteArbitrage 执行套利订单（多条腿同时执行）
func (m *Manager) ExecuteArbitrage(ctx context.Context, opp *strategy.Opportunity) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(opp.Legs))
	results := make([]*model.Order, len(opp.Legs))

	for i, leg := range opp.Legs {
		wg.Add(1)
		go func(idx int, l strategy.Leg) {
			defer wg.Done()
			order, err := m.ExecuteOrder(ctx, &l)
			if err != nil {
				errChan <- err
				return
			}
			results[idx] = order
		}(i, leg)
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	if err := <-errChan; err != nil {
		// TODO: 执行回滚逻辑
		return fmt.Errorf("arbitrage execution failed: %w", err)
	}

	return nil
}

// CancelOrder 取消订单
func (m *Manager) CancelOrder(ctx context.Context, exchangeName, orderID string) error {
	m.mu.RLock()
	ex, ok := m.exchanges[exchangeName]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown exchange: %s", exchangeName)
	}

	return ex.CancelOrder(ctx, "", orderID)
}

// GetOrder 获取订单
func (m *Manager) GetOrder(ctx context.Context, exchangeName, orderID string) (*model.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	order, ok := m.orderMap[orderID]
	if !ok {
		// 从数据库查询
		var dbOrder model.Order
		if err := database.DB.Where("order_id = ? AND exchange = ?", orderID, exchangeName).First(&dbOrder).Error; err != nil {
			return nil, err
		}
		return &dbOrder, nil
	}

	return order, nil
}

// GetPendingOrders 获取待执行订单
func (m *Manager) GetPendingOrders() []*strategy.Leg {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*strategy.Leg, 0, len(m.pending))
	for _, leg := range m.pending {
		result = append(result, leg)
	}
	return result
}

// UpdateOrderStatus 更新订单状态
func (m *Manager) UpdateOrderStatus(orderID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if order, ok := m.orderMap[orderID]; ok {
		order.Status = status
		return database.DB.Model(order).Update("status", status).Error
	}
	return fmt.Errorf("order not found: %s", orderID)
}

// GetOrderHistory 获取订单历史
func (m *Manager) GetOrderHistory(limit int) ([]model.Order, error) {
	var orders []model.Order
	err := database.DB.Order("created_at DESC").Limit(limit).Find(&orders).Error
	return orders, err
}

// GetOrdersByStrategy 获取策略相关订单
func (m *Manager) GetOrdersByStrategy(strategyID uint) ([]model.Order, error) {
	var orders []model.Order
	err := database.DB.Where("strategy_id = ?", strategyID).Find(&orders).Error
	return orders, err
}

// SyncOrderStatus 同步订单状态
func (m *Manager) SyncOrderStatus(ctx context.Context, exchangeName, orderID string) (*model.Order, error) {
	ex, ok := m.exchanges[exchangeName]
	if !ok {
		return nil, fmt.Errorf("unknown exchange: %s", exchangeName)
	}

	remoteOrder, err := ex.GetOrder(ctx, "", orderID)
	if err != nil {
		return nil, err
	}

	// 更新本地状态
	if err := m.UpdateOrderStatus(orderID, remoteOrder.Status); err != nil {
		return nil, err
	}

	return m.GetOrder(ctx, exchangeName, orderID)
}

// Stats 订单统计
type Stats struct {
	TotalOrders   int64   `json:"total_orders"`
	PendingOrders int64   `json:"pending_orders"`
	FilledOrders  int64   `json:"filled_orders"`
	TotalVolume   float64 `json:"total_volume"`
	TotalProfit   float64 `json:"total_profit"`
}

// GetStats 获取统计信息
func (m *Manager) GetStats() (*Stats, error) {
	var totalOrders, pendingOrders, filledOrders int64
	var totalVolume float64

	database.DB.Model(&model.Order{}).Count(&totalOrders)
	database.DB.Model(&model.Order{}).Where("status = ?", "pending").Count(&pendingOrders)
	database.DB.Model(&model.Order{}).Where("status = ?", "filled").Count(&filledOrders)

	// 计算总成交量
	rows, err := database.DB.Model(&model.Order{}).Where("status = ?", "filled").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var order model.Order
		database.DB.ScanRows(rows, &order)
		totalVolume += order.Price * order.ExecutedQty
	}

	return &Stats{
		TotalOrders:   totalOrders,
		PendingOrders: pendingOrders,
		FilledOrders:  filledOrders,
		TotalVolume:   totalVolume,
	}, nil
}
