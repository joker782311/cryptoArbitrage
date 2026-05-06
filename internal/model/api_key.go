package model

import "time"

type APIKey struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	Exchange   string    `gorm:"size:20;index" json:"exchange"` // binance, okx, bitget
	Name       string    `gorm:"size:50" json:"name"`
	APIKey     string    `gorm:"size:255" json:"api_key"`      // 加密存储
	APISecret  string    `gorm:"size:255" json:"api_secret"`   // 加密存储
	Passphrase string    `gorm:"size:255" json:"passphrase"`   // OKX/Bitget 需要
	IsEnabled  bool      `gorm:"default:true" json:"is_enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
