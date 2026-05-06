package model

import "time"

type Strategy struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Name          string    `gorm:"size:50;uniqueIndex" json:"name"` // cross_exchange, funding_rate, spot_future, triangular, dex_triangular, dex_cross_dex
	IsEnabled     bool      `gorm:"default:false" json:"is_enabled"`
	AutoExecute   bool      `gorm:"default:false" json:"auto_execute"`
	MinProfitRate float64   `gorm:"type:decimal(10,8)" json:"min_profit_rate"` // 最小利润率
	MaxPosition   float64   `gorm:"type:decimal(20,8)" json:"max_position"`    // 最大仓位
	StopLossRate  float64   `gorm:"type:decimal(10,8)" json:"stop_loss_rate"`  // 止损率
	Config        string    `gorm:"type:text" json:"config"`                   // JSON 配置
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
