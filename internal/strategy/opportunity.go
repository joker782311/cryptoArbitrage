package strategy

// Opportunity 套利机会
type Opportunity struct {
	ID                    string  `json:"id"`
	StrategyType          string  `json:"strategy_type"`       // cross_exchange, funding_rate, spot_future, triangular, dex_triangular, dex_cross_dex
	Direction             string  `json:"direction,omitempty"` // spot_long_perp_short, spot_short_inventory_perp_long
	Timestamp             int64   `json:"timestamp"`
	Notional              float64 `json:"notional,omitempty"`                // 本次机会使用的名义本金 (USDT)
	ProfitRate            float64 `json:"profit_rate"`                       // 利润率 (%)
	ProfitAmount          float64 `json:"profit_amount"`                     // 预估利润金额 (USDT)
	BasisAmount           float64 `json:"basis_amount,omitempty"`            // 基差贡献 (USDT)
	FundingAmount         float64 `json:"funding_amount,omitempty"`          // 预计资金费率贡献 (USDT)
	FeeCost               float64 `json:"fee_cost,omitempty"`                // 交易手续费 (USDT)
	SafetyBuffer          float64 `json:"safety_buffer,omitempty"`           // 安全缓冲 (USDT)
	CarryFundingAmount    float64 `json:"carry_funding_amount,omitempty"`    // 多期持仓资金费率贡献 (USDT)
	CarryNetProfit        float64 `json:"carry_net_profit,omitempty"`        // 多期持仓预期净收益 (USDT)
	CarryProfitRate       float64 `json:"carry_profit_rate,omitempty"`       // 多期持仓预期收益率 (%)
	CarryFundingIntervals float64 `json:"carry_funding_intervals,omitempty"` // 多期持仓估算使用的资金费率期数

	// 跨交易所字段
	ExchangeA    string  `json:"exchange_a,omitempty"`
	ExchangeB    string  `json:"exchange_b,omitempty"`
	SpotExchange string  `json:"spot_exchange,omitempty"`
	PerpExchange string  `json:"perp_exchange,omitempty"`
	Symbol       string  `json:"symbol,omitempty"`
	PriceA       float64 `json:"price_a,omitempty"`
	PriceB       float64 `json:"price_b,omitempty"`

	// DEX 字段
	PoolA string   `json:"pool_a,omitempty"`
	PoolB string   `json:"pool_b,omitempty"`
	Path  []string `json:"path,omitempty"` // 三角套利路径

	// 执行信息
	Legs         []Leg   `json:"legs"`
	EstimatedGas float64 `json:"estimated_gas"` // 预估手续费
	Slippage     float64 `json:"slippage"`      // 预估滑点
	NetProfit    float64 `json:"net_profit"`    // 净利润
}

// Leg 交易腿
type Leg struct {
	ID       int     `json:"id"`
	Exchange string  `json:"exchange"`
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"` // buy, sell
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
	OrderID  string  `json:"order_id,omitempty"`
	Status   string  `json:"status"` // pending, filled, failed
}

// NetProfit 计算净利润
func (o *Opportunity) CalculateNetProfit() float64 {
	o.NetProfit = o.ProfitAmount - o.EstimatedGas - o.Slippage
	return o.NetProfit
}

// IsValid 检查机会是否有效
func (o *Opportunity) IsValid(minProfitRate float64) bool {
	return o.ProfitRate >= minProfitRate && o.NetProfit > 0
}
