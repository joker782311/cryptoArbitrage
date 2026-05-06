package model

import "time"

type Alert struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Type      string    `gorm:"size:20" json:"type"` // price, opportunity, system, order
	Level     string    `gorm:"size:10" json:"level"` // info, warning, error
	Title     string    `gorm:"size:100" json:"title"`
	Message   string    `gorm:"type:text" json:"message"`
	IsRead    bool      `gorm:"default:false" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type AlertConfig struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	Channel      string `gorm:"size:20" json:"channel"` // telegram, slack, email, sms, webhook
	WebhookURL   string `gorm:"size:255" json:"webhook_url"`
	Email        string `gorm:"size:100" json:"email"`
	Phone        string `gorm:"size:20" json:"phone"`
	ChatID       string `gorm:"size:50" json:"chat_id"` // Telegram
	IsEnabled    bool   `gorm:"default:true" json:"is_enabled"`
}
