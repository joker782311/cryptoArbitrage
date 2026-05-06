package model

import "time"

type Order struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	StrategyID  uint      `gorm:"index" json:"strategy_id"`
	Exchange    string    `gorm:"size:20;index" json:"exchange"`
	Symbol      string    `gorm:"size:20" json:"symbol"`
	Side        string    `gorm:"size:10" json:"side"` // buy, sell
	Type        string    `gorm:"size:10" json:"type"` // market, limit
	Price       float64   `gorm:"type:decimal(20,8)" json:"price"`
	Quantity    float64   `gorm:"type:decimal(20,8)" json:"quantity"`
	ExecutedQty float64   `gorm:"type:decimal(20,8)" json:"executed_qty"`
	Status      string    `gorm:"size:20" json:"status"` // pending, filled, cancelled, failed
	OrderID     string    `gorm:"size:50" json:"order_id"` // 交易所订单 ID
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
