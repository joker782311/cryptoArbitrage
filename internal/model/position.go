package model

import "time"

type Position struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Exchange     string    `gorm:"size:20;index" json:"exchange"`
	Symbol       string    `gorm:"size:20" json:"symbol"`
	Side         string    `gorm:"size:10" json:"side"` // long, short
	Quantity     float64   `gorm:"type:decimal(20,8)" json:"quantity"`
	EntryPrice   float64   `gorm:"type:decimal(20,8)" json:"entry_price"`
	CurrentPrice float64   `gorm:"type:decimal(20,8)" json:"current_price"`
	PNL          float64   `gorm:"type:decimal(20,8)" json:"pnl"`
	PNLPercent   float64   `gorm:"type:decimal(10,4)" json:"pnl_percent"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
