package model

import "time"

type Ticker struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Exchange  string    `gorm:"size:20;index" json:"exchange"`
	Symbol    string    `gorm:"size:20;index" json:"symbol"`
	Price     float64   `gorm:"type:decimal(20,8)" json:"price"`
	Bid       float64   `gorm:"type:decimal(20,8)" json:"bid"`
	Ask       float64   `gorm:"type:decimal(20,8)" json:"ask"`
	Volume24h float64   `gorm:"type:decimal(20,8)" json:"volume_24h"`
	Change24h float64   `gorm:"type:decimal(10,4)" json:"change_24h"`
	Timestamp time.Time `json:"timestamp"`
}

type FundingRate struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Exchange    string    `gorm:"size:20;index" json:"exchange"`
	Symbol      string    `gorm:"size:20;index" json:"symbol"`
	FundingRate float64   `gorm:"type:decimal(10,8)" json:"funding_rate"`
	NextFunding time.Time `json:"next_funding"`
	Timestamp   time.Time `json:"timestamp"`
}
