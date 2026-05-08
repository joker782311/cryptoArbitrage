package model

import "time"

type CEXSpotPerpAutomationSetting struct {
	Name                string    `gorm:"primaryKey;size:50" json:"name"`
	Enabled             bool      `gorm:"default:false" json:"enabled"`
	AutoOpen            bool      `gorm:"default:true" json:"auto_open"`
	AutoClose           bool      `gorm:"default:true" json:"auto_close"`
	OpenMinProfitRate   float64   `gorm:"type:decimal(10,6)" json:"open_min_profit_rate"`
	CloseMinProfitRate  float64   `gorm:"type:decimal(10,6)" json:"close_min_profit_rate"`
	MaxHoldSeconds      int64     `json:"max_hold_seconds"`
	MaxOpenPositions    int       `json:"max_open_positions"`
	CheckIntervalMillis int64     `json:"check_interval_millis"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CEXSpotPerpOpportunityLog struct {
	Key              string    `gorm:"primaryKey;size:160" json:"key"`
	OpportunityID    string    `gorm:"size:220;index" json:"opportunity_id"`
	Symbol           string    `gorm:"size:30;index" json:"symbol"`
	Direction        string    `gorm:"size:50;index" json:"direction"`
	SpotExchange     string    `gorm:"size:20;index" json:"spot_exchange"`
	PerpExchange     string    `gorm:"size:20;index" json:"perp_exchange"`
	FirstSeenAt      int64     `gorm:"index" json:"first_seen_at"`
	LastSeenAt       int64     `gorm:"index" json:"last_seen_at"`
	SeenCount        int       `json:"seen_count"`
	BestProfit       float64   `gorm:"type:decimal(20,8)" json:"best_profit"`
	BestProfitRate   float64   `gorm:"type:decimal(10,6)" json:"best_profit_rate"`
	LastProfit       float64   `gorm:"type:decimal(20,8)" json:"last_profit"`
	LastProfitRate   float64   `gorm:"type:decimal(10,6)" json:"last_profit_rate"`
	LastStatus       string    `gorm:"size:20" json:"last_status"`
	LastBlockReason  string    `gorm:"type:text" json:"last_block_reason"`
	AutoOpenedCount  int       `json:"auto_opened_count"`
	AutoRejectedNote string    `gorm:"type:text" json:"auto_rejected_note"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CEXSpotPerpAutoTrade struct {
	ID           string    `gorm:"primaryKey;size:220" json:"id"`
	PositionID   string    `gorm:"size:120;index" json:"position_id"`
	Opportunity  string    `gorm:"size:220;index" json:"opportunity"`
	Symbol       string    `gorm:"size:30;index" json:"symbol"`
	Direction    string    `gorm:"size:50;index" json:"direction"`
	SpotExchange string    `gorm:"size:20;index" json:"spot_exchange"`
	PerpExchange string    `gorm:"size:20;index" json:"perp_exchange"`
	Action       string    `gorm:"size:20;index" json:"action"`
	Reason       string    `gorm:"type:text" json:"reason"`
	Quantity     float64   `gorm:"type:decimal(20,8)" json:"quantity"`
	Notional     float64   `gorm:"type:decimal(20,8)" json:"notional"`
	Margin       float64   `gorm:"type:decimal(20,8)" json:"margin"`
	SpotValue    float64   `gorm:"type:decimal(20,8)" json:"spot_value"`
	CapitalUsed  float64   `gorm:"type:decimal(20,8)" json:"capital_used"`
	Profit       float64   `gorm:"type:decimal(20,8)" json:"profit"`
	ProfitRate   float64   `gorm:"type:decimal(10,6)" json:"profit_rate"`
	CreatedAtMs  int64     `gorm:"index" json:"created_at_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type CEXSpotPerpSimAccount struct {
	Exchange      string    `gorm:"primaryKey;size:20" json:"exchange"`
	USDT          float64   `gorm:"type:decimal(20,8)" json:"usdt"`
	PerpUSDT      float64   `gorm:"type:decimal(20,8)" json:"perp_usdt"`
	FrozenUSDT    float64   `gorm:"type:decimal(20,8)" json:"frozen_usdt"`
	SpotBalances  string    `gorm:"type:json" json:"spot_balances"`
	PerpPositions string    `gorm:"type:json" json:"perp_positions"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CEXSpotPerpSimPosition struct {
	ID              string    `gorm:"primaryKey;size:220" json:"id"`
	OpportunityID   string    `gorm:"size:220;index" json:"opportunity_id"`
	Symbol          string    `gorm:"size:30;index" json:"symbol"`
	Direction       string    `gorm:"size:50;index" json:"direction"`
	SpotExchange    string    `gorm:"size:20;index" json:"spot_exchange"`
	PerpExchange    string    `gorm:"size:20;index" json:"perp_exchange"`
	Status          string    `gorm:"size:20;index" json:"status"`
	OpenedAt        int64     `gorm:"index" json:"opened_at"`
	ClosedAt        int64     `gorm:"index" json:"closed_at"`
	Notional        float64   `gorm:"type:decimal(20,8)" json:"notional"`
	Margin          float64   `gorm:"type:decimal(20,8)" json:"margin"`
	RealizedPnL     float64   `gorm:"type:decimal(20,8)" json:"realized_pnl"`
	OpportunityJSON string    `gorm:"type:json" json:"opportunity_json"`
	TradesJSON      string    `gorm:"type:json" json:"trades_json"`
	UpdatedAt       time.Time `json:"updated_at"`
}
