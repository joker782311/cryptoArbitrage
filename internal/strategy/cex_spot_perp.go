package strategy

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	StrategyTypeCEXSpotPerp = "cex_spot_perp"

	// DirectionSpotLongPerpShort 是第一阶段主策略：买入现货，同时做空 U 本位永续。
	DirectionSpotLongPerpShort = "spot_long_perp_short"
	// DirectionSpotShortInventoryPerpLong 是第二阶段策略：只允许卖出已有现货库存，同时做多 U 本位永续。
	DirectionSpotShortInventoryPerpLong = "spot_short_inventory_perp_long"

	MarketTypeSpot = "spot"
	MarketTypePerp = "perp"
)

var (
	ErrSimAccountNotFound          = errors.New("sim account not found")
	ErrSimInsufficientUSDT         = errors.New("insufficient simulated USDT")
	ErrSimInsufficientInventory    = errors.New("insufficient simulated spot inventory")
	ErrSimInsufficientMargin       = errors.New("insufficient simulated perp margin")
	ErrSimCircuitBreakerActive     = errors.New("sim circuit breaker is active")
	ErrSimUnsupportedDirection     = errors.New("unsupported spot-perp direction")
	ErrSimOpportunityNotProfitable = errors.New("opportunity is not profitable after costs")
	ErrSimPositionNotFound         = errors.New("sim position not found")
	ErrSimPositionNotOpen          = errors.New("sim position is not open")
)

// CEXSpotPerpQuote 保存现货或永续的盘口快照。模拟盘用 ask/bid 估算成交价；
// 未来切到真实盘时，应改用交易所订单回报里的成交均价。
type CEXSpotPerpQuote struct {
	Exchange    string
	Symbol      string
	MarketType  string
	Bid         float64
	Ask         float64
	Last        float64
	FundingRate float64 // 单次资金费率，正数表示空永续收钱、多永续付钱
	Timestamp   int64
}

// CEXSpotPerpConfig 是跨所期现策略的核心参数。第一版用固定滑点和费率，
// 后续可以把费用模型替换成交易所/VIP 等级维度的动态配置。
type CEXSpotPerpConfig struct {
	Symbols                []string
	Exchanges              []string
	NotionalUSDT           float64
	MinNetProfitRate       float64
	FundingIntervals       float64
	SpotTakerFeeRate       float64
	PerpTakerFeeRate       float64
	SlippageRate           float64
	SafetyBufferRate       float64
	DefaultLeverage        float64
	EnableInventoryReverse bool
}

// DefaultCEXSpotPerpConfig 给出保守的模拟盘默认值，避免没有配置时过度高估收益。
func DefaultCEXSpotPerpConfig() CEXSpotPerpConfig {
	return CEXSpotPerpConfig{
		Symbols:                []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
		Exchanges:              []string{"binance", "okx", "bitget"},
		NotionalUSDT:           1000,
		MinNetProfitRate:       0.2,
		FundingIntervals:       1,
		SpotTakerFeeRate:       0.001,
		PerpTakerFeeRate:       0.0005,
		SlippageRate:           0.0005,
		SafetyBufferRate:       0.0002,
		DefaultLeverage:        3,
		EnableInventoryReverse: true,
	}
}

// CEXSpotPerpStrategy 只负责发现机会和计算预期收益，不直接真实下单。
// 自动化系统必须把机会交给统一执行器，由模拟盘或真实盘执行器决定如何成交。
type CEXSpotPerpStrategy struct {
	mu                 sync.RWMutex
	config             CEXSpotPerpConfig
	spotQuotes         map[string]map[string]CEXSpotPerpQuote
	perpQuotes         map[string]map[string]CEXSpotPerpQuote
	opportunityHandler func(*Opportunity)
}

func NewCEXSpotPerpStrategy(config CEXSpotPerpConfig) *CEXSpotPerpStrategy {
	if config.NotionalUSDT == 0 {
		config = DefaultCEXSpotPerpConfig()
	}
	if config.DefaultLeverage == 0 {
		config.DefaultLeverage = 3
	}
	return &CEXSpotPerpStrategy{
		config:     config,
		spotQuotes: make(map[string]map[string]CEXSpotPerpQuote),
		perpQuotes: make(map[string]map[string]CEXSpotPerpQuote),
	}
}

func (s *CEXSpotPerpStrategy) SetOpportunityHandler(handler func(*Opportunity)) {
	s.opportunityHandler = handler
}

// UpdateQuote 更新单个盘口快照。收到任一侧价格后立即扫描该 symbol，
// 这样后续接 WebSocket 时可以做到事件驱动，而不是固定轮询。
func (s *CEXSpotPerpStrategy) UpdateQuote(q CEXSpotPerpQuote) {
	s.mu.Lock()
	target := s.spotQuotes
	if q.MarketType == MarketTypePerp {
		target = s.perpQuotes
	}
	if _, ok := target[q.Exchange]; !ok {
		target[q.Exchange] = make(map[string]CEXSpotPerpQuote)
	}
	target[q.Exchange][q.Symbol] = q
	s.mu.Unlock()

	if s.opportunityHandler == nil {
		return
	}
	for _, opp := range s.ScanSymbol(q.Symbol) {
		s.opportunityHandler(opp)
	}
}

// ScanSymbol 按“任意现货所 x 任意合约所”全组合扫描机会，包括同所期现。
func (s *CEXSpotPerpStrategy) ScanSymbol(symbol string) []*Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var opportunities []*Opportunity
	for _, spotEx := range s.config.Exchanges {
		spotQuote, ok := s.spotQuotes[spotEx][symbol]
		if !ok || spotQuote.Ask <= 0 || spotQuote.Bid <= 0 {
			continue
		}
		for _, perpEx := range s.config.Exchanges {
			perpQuote, ok := s.perpQuotes[perpEx][symbol]
			if !ok || perpQuote.Ask <= 0 || perpQuote.Bid <= 0 {
				continue
			}
			if opp := s.buildOpportunity(spotQuote, perpQuote, DirectionSpotLongPerpShort); opp != nil {
				opportunities = append(opportunities, opp)
			}
			if s.config.EnableInventoryReverse {
				if opp := s.buildOpportunity(spotQuote, perpQuote, DirectionSpotShortInventoryPerpLong); opp != nil {
					opportunities = append(opportunities, opp)
				}
			}
		}
	}
	return opportunities
}

func (s *CEXSpotPerpStrategy) buildOpportunity(spot, perp CEXSpotPerpQuote, direction string) *Opportunity {
	notional := s.config.NotionalUSDT
	spotFee := notional * s.config.SpotTakerFeeRate
	perpFee := notional * s.config.PerpTakerFeeRate
	feeCost := spotFee + perpFee

	// 滑点不是手续费。这里单独计入安全成本，代表盘口深度不足或成交价漂移。
	slippageCost := notional * s.config.SlippageRate * 2
	safetyBuffer := notional * s.config.SafetyBufferRate

	var quantity, basisAmount, fundingAmount, spotPrice, perpPrice float64
	switch direction {
	case DirectionSpotLongPerpShort:
		spotPrice = spot.Ask
		perpPrice = perp.Bid
		quantity = notional / spotPrice
		basisAmount = (perpPrice - spotPrice) * quantity
		fundingAmount = notional * perp.FundingRate * s.config.FundingIntervals
	case DirectionSpotShortInventoryPerpLong:
		spotPrice = spot.Bid
		perpPrice = perp.Ask
		quantity = notional / spotPrice
		basisAmount = (spotPrice - perpPrice) * quantity
		// 负资金费率时，多永续收钱；正资金费率会成为成本。
		fundingAmount = -notional * perp.FundingRate * s.config.FundingIntervals
	default:
		return nil
	}

	grossProfit := basisAmount + fundingAmount
	netProfit := grossProfit - feeCost - slippageCost - safetyBuffer
	profitRate := netProfit / notional * 100
	if profitRate < s.config.MinNetProfitRate {
		return nil
	}

	opp := &Opportunity{
		ID:            generateID("csp", direction, spot.Exchange, perp.Exchange, spot.Symbol),
		StrategyType:  StrategyTypeCEXSpotPerp,
		Direction:     direction,
		Timestamp:     time.Now().UnixMilli(),
		Notional:      notional,
		ProfitRate:    profitRate,
		ProfitAmount:  grossProfit,
		BasisAmount:   basisAmount,
		FundingAmount: fundingAmount,
		FeeCost:       feeCost,
		EstimatedGas:  feeCost,
		Slippage:      slippageCost,
		SafetyBuffer:  safetyBuffer,
		NetProfit:     netProfit,
		ExchangeA:     spot.Exchange,
		ExchangeB:     perp.Exchange,
		SpotExchange:  spot.Exchange,
		PerpExchange:  perp.Exchange,
		Symbol:        spot.Symbol,
		PriceA:        spotPrice,
		PriceB:        perpPrice,
		Legs:          buildSpotPerpLegs(direction, spot.Exchange, perp.Exchange, spot.Symbol, quantity, spotPrice, perpPrice),
	}
	return opp
}

func buildSpotPerpLegs(direction, spotExchange, perpExchange, symbol string, quantity, spotPrice, perpPrice float64) []Leg {
	switch direction {
	case DirectionSpotLongPerpShort:
		return []Leg{
			{ID: 1, Exchange: spotExchange, Symbol: symbol, Side: "buy", Quantity: quantity, Price: spotPrice, Status: "pending"},
			{ID: 2, Exchange: perpExchange, Symbol: symbol, Side: "sell", Quantity: quantity, Price: perpPrice, Status: "pending"},
		}
	case DirectionSpotShortInventoryPerpLong:
		return []Leg{
			{ID: 1, Exchange: spotExchange, Symbol: symbol, Side: "sell", Quantity: quantity, Price: spotPrice, Status: "pending"},
			{ID: 2, Exchange: perpExchange, Symbol: symbol, Side: "buy", Quantity: quantity, Price: perpPrice, Status: "pending"},
		}
	default:
		return nil
	}
}

func (s *CEXSpotPerpStrategy) Start(_ context.Context) error { return nil }

func (s *CEXSpotPerpStrategy) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":                     StrategyTypeCEXSpotPerp,
		"symbols":                  s.config.Symbols,
		"exchanges":                s.config.Exchanges,
		"notional_usdt":            s.config.NotionalUSDT,
		"min_net_profit_rate":      s.config.MinNetProfitRate,
		"enable_inventory_reverse": s.config.EnableInventoryReverse,
	}
}

// SimAccount 是每个交易所独立的模拟账户。跨所套利不能把资金当作全局池，
// 因为真实交易中 Binance 的 USDT 不能瞬间挪到 OKX 或 Bitget。
type SimAccount struct {
	Exchange      string
	USDT          float64
	PerpUSDT      float64
	FrozenUSDT    float64
	SpotBalances  map[string]float64
	PerpPositions map[string]float64
}

type SimTrade struct {
	Exchange string
	Market   string
	Symbol   string
	Side     string
	Quantity float64
	Price    float64
	Fee      float64
}

type SimArbitragePosition struct {
	ID          string
	Opportunity *Opportunity
	Status      string
	OpenedAt    int64
	ClosedAt    int64
	Trades      []SimTrade
	Notional    float64
	Margin      float64
	RealizedPnL float64
}

type SimCloseAction struct {
	PositionID string
	Reason     string
	Legs       []Leg
	CreatedAt  int64
}

// CEXSpotPerpSimulator 执行模拟盘开平仓。它不发送真实订单，
// 但会按真实系统需要的账户隔离、保证金占用和库存检查来拒绝不合规机会。
type CEXSpotPerpSimulator struct {
	mu           sync.RWMutex
	accounts     map[string]*SimAccount
	positions    map[string]*SimArbitragePosition
	closeActions []SimCloseAction
	leverage     float64
	halted       bool
	haltReason   string
}

func NewCEXSpotPerpSimulator(accounts map[string]*SimAccount, leverage float64) *CEXSpotPerpSimulator {
	if leverage <= 0 {
		leverage = 3
	}
	cp := make(map[string]*SimAccount, len(accounts))
	for name, account := range accounts {
		cp[name] = cloneSimAccount(account)
	}
	return &CEXSpotPerpSimulator{
		accounts:  cp,
		positions: make(map[string]*SimArbitragePosition),
		leverage:  leverage,
	}
}

func cloneSimAccount(account *SimAccount) *SimAccount {
	if account == nil {
		return nil
	}
	cloned := *account
	cloned.SpotBalances = make(map[string]float64, len(account.SpotBalances))
	for asset, balance := range account.SpotBalances {
		cloned.SpotBalances[asset] = balance
	}
	cloned.PerpPositions = make(map[string]float64, len(account.PerpPositions))
	for symbol, qty := range account.PerpPositions {
		cloned.PerpPositions[symbol] = qty
	}
	return &cloned
}

func (s *CEXSpotPerpSimulator) Account(exchangeName string) (*SimAccount, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[exchangeName]
	return cloneSimAccount(account), ok
}

func (s *CEXSpotPerpSimulator) Positions() []*SimArbitragePosition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*SimArbitragePosition, 0, len(s.positions))
	for _, pos := range s.positions {
		copyPos := *pos
		result = append(result, &copyPos)
	}
	return result
}

func (s *CEXSpotPerpSimulator) PnLSummary() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var realized, unrealized, openNotional float64
	for _, pos := range s.positions {
		if pos == nil || pos.Opportunity == nil {
			continue
		}
		switch pos.Status {
		case "closed":
			realized += pos.RealizedPnL
		case "open", "closing":
			unrealized += pos.Opportunity.NetProfit
			openNotional += pos.Notional
		}
	}
	return map[string]float64{
		"realizedPnL":   realized,
		"unrealizedPnL": unrealized,
		"totalPnL":      realized + unrealized,
		"openNotional":  openNotional,
	}
}

func (s *CEXSpotPerpSimulator) CloseActions() []SimCloseAction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SimCloseAction, len(s.closeActions))
	copy(result, s.closeActions)
	return result
}

func (s *CEXSpotPerpSimulator) IsHalted() (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.halted, s.haltReason
}

func (s *CEXSpotPerpSimulator) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.halted = false
	s.haltReason = ""
}

func (s *CEXSpotPerpSimulator) ExecuteOpportunity(opp *Opportunity) (*SimArbitragePosition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.halted {
		return nil, ErrSimCircuitBreakerActive
	}
	if opp == nil || opp.NetProfit <= 0 {
		return nil, ErrSimOpportunityNotProfitable
	}

	spotAccount, ok := s.accounts[opp.SpotExchange]
	if !ok || spotAccount == nil {
		return nil, fmt.Errorf("%w: %s", ErrSimAccountNotFound, opp.SpotExchange)
	}
	perpAccount, ok := s.accounts[opp.PerpExchange]
	if !ok || perpAccount == nil {
		return nil, fmt.Errorf("%w: %s", ErrSimAccountNotFound, opp.PerpExchange)
	}

	baseAsset := baseAssetFromSymbol(opp.Symbol)
	quantity := spotPerpQuantity(opp)
	spotFee := opp.Notional * 0.001
	if opp.FeeCost > 0 {
		spotFee = opp.FeeCost / 2
	}
	perpFee := spotFee
	margin := opp.Notional / s.leverage

	switch opp.Direction {
	case DirectionSpotLongPerpShort:
		spotCost := quantity*opp.PriceA + spotFee
		if spotAccount.USDT < spotCost {
			return nil, ErrSimInsufficientUSDT
		}
		if perpAccount.PerpUSDT-perpAccount.FrozenUSDT < margin+perpFee {
			return nil, ErrSimInsufficientMargin
		}
		spotAccount.USDT -= spotCost
		spotAccount.SpotBalances[baseAsset] += quantity
		perpAccount.PerpUSDT -= perpFee
		perpAccount.FrozenUSDT += margin
		perpAccount.PerpPositions[opp.Symbol] -= quantity
	case DirectionSpotShortInventoryPerpLong:
		if spotAccount.SpotBalances[baseAsset]+1e-12 < quantity {
			return nil, ErrSimInsufficientInventory
		}
		if perpAccount.PerpUSDT-perpAccount.FrozenUSDT < margin+perpFee {
			return nil, ErrSimInsufficientMargin
		}
		spotAccount.SpotBalances[baseAsset] -= quantity
		spotAccount.USDT += quantity*opp.PriceA - spotFee
		perpAccount.PerpUSDT -= perpFee
		perpAccount.FrozenUSDT += margin
		perpAccount.PerpPositions[opp.Symbol] += quantity
	default:
		return nil, ErrSimUnsupportedDirection
	}

	position := &SimArbitragePosition{
		ID:          generateID("sim", opp.ID),
		Opportunity: opp,
		Status:      "open",
		OpenedAt:    time.Now().UnixMilli(),
		Notional:    opp.Notional,
		Margin:      margin,
		Trades: []SimTrade{
			{Exchange: opp.SpotExchange, Market: MarketTypeSpot, Symbol: opp.Symbol, Side: opp.Legs[0].Side, Quantity: quantity, Price: opp.PriceA, Fee: spotFee},
			{Exchange: opp.PerpExchange, Market: MarketTypePerp, Symbol: opp.Symbol, Side: opp.Legs[1].Side, Quantity: quantity, Price: opp.PriceB, Fee: perpFee},
		},
	}
	s.positions[position.ID] = position
	return position, nil
}

// ClosePosition 执行普通模拟平仓。这里按开仓价反向平掉两条腿，
// 重点验证账户释放、库存回补和持仓状态流转；真实盘必须改用实时盘口或真实成交回报。
func (s *CEXSpotPerpSimulator) ClosePosition(positionID, reason string) (*SimCloseAction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos, ok := s.positions[positionID]
	if !ok || pos == nil {
		return nil, ErrSimPositionNotFound
	}
	if pos.Status != "open" {
		return nil, ErrSimPositionNotOpen
	}

	opp := pos.Opportunity
	spotAccount, ok := s.accounts[opp.SpotExchange]
	if !ok || spotAccount == nil {
		return nil, fmt.Errorf("%w: %s", ErrSimAccountNotFound, opp.SpotExchange)
	}
	perpAccount, ok := s.accounts[opp.PerpExchange]
	if !ok || perpAccount == nil {
		return nil, fmt.Errorf("%w: %s", ErrSimAccountNotFound, opp.PerpExchange)
	}

	quantity := spotPerpQuantity(opp)
	baseAsset := baseAssetFromSymbol(opp.Symbol)
	switch opp.Direction {
	case DirectionSpotLongPerpShort:
		spotAccount.SpotBalances[baseAsset] = math.Max(0, spotAccount.SpotBalances[baseAsset]-quantity)
		spotAccount.USDT += quantity * opp.PriceA
		perpAccount.PerpPositions[opp.Symbol] += quantity
	case DirectionSpotShortInventoryPerpLong:
		spotAccount.USDT = math.Max(0, spotAccount.USDT-quantity*opp.PriceA)
		spotAccount.SpotBalances[baseAsset] += quantity
		perpAccount.PerpPositions[opp.Symbol] -= quantity
	default:
		return nil, ErrSimUnsupportedDirection
	}

	perpAccount.FrozenUSDT = math.Max(0, perpAccount.FrozenUSDT-pos.Margin)
	pos.Status = "closed"
	pos.ClosedAt = time.Now().UnixMilli()
	// 第一版模拟盘没有实时平仓盘口，先把开仓时估算的净收益作为已实现收益。
	// 后续接订单簿或真实成交回报时，应在这里改成真实平仓盈亏。
	pos.RealizedPnL = opp.NetProfit

	action := SimCloseAction{
		PositionID: pos.ID,
		Reason:     reason,
		Legs:       closeLegsForOpportunity(opp),
		CreatedAt:  pos.ClosedAt,
	}
	s.closeActions = append(s.closeActions, action)
	return &action, nil
}

// TriggerCircuitBreaker 触发熔断：停止新开仓，并为所有未平组合生成紧急平仓动作。
// 真实盘实现这里不能盲目全平，应先评估每条腿的可成交性，避免平掉一边制造裸露风险。
func (s *CEXSpotPerpSimulator) TriggerCircuitBreaker(reason string) []SimCloseAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.halted = true
	s.haltReason = reason
	for _, pos := range s.positions {
		if pos.Status != "open" {
			continue
		}
		action := SimCloseAction{
			PositionID: pos.ID,
			Reason:     reason,
			Legs:       closeLegsForOpportunity(pos.Opportunity),
			CreatedAt:  time.Now().UnixMilli(),
		}
		s.closeActions = append(s.closeActions, action)
	}
	result := make([]SimCloseAction, len(s.closeActions))
	copy(result, s.closeActions)
	return result
}

func closeLegsForOpportunity(opp *Opportunity) []Leg {
	if opp == nil {
		return nil
	}
	quantity := spotPerpQuantity(opp)
	switch opp.Direction {
	case DirectionSpotLongPerpShort:
		return []Leg{
			{ID: 1, Exchange: opp.SpotExchange, Symbol: opp.Symbol, Side: "sell", Quantity: quantity, Status: "pending"},
			{ID: 2, Exchange: opp.PerpExchange, Symbol: opp.Symbol, Side: "buy", Quantity: quantity, Status: "pending"},
		}
	case DirectionSpotShortInventoryPerpLong:
		return []Leg{
			{ID: 1, Exchange: opp.SpotExchange, Symbol: opp.Symbol, Side: "buy", Quantity: quantity, Status: "pending"},
			{ID: 2, Exchange: opp.PerpExchange, Symbol: opp.Symbol, Side: "sell", Quantity: quantity, Status: "pending"},
		}
	default:
		return nil
	}
}

func spotPerpQuantity(opp *Opportunity) float64 {
	if opp == nil || len(opp.Legs) == 0 {
		return 0
	}
	return math.Abs(opp.Legs[0].Quantity)
}

func baseAssetFromSymbol(symbol string) string {
	return strings.TrimSuffix(symbol, "USDT")
}
