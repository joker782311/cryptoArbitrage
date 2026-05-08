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

var defaultCEXSpotPerpSymbols = []string{
	"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
	"DOGEUSDT", "ADAUSDT", "TRXUSDT", "LINKUSDT", "AVAXUSDT",
	"TONUSDT", "SHIBUSDT", "DOTUSDT", "BCHUSDT", "LTCUSDT",
	"UNIUSDT", "NEARUSDT", "APTUSDT", "ICPUSDT", "ETCUSDT",
}

var (
	ErrSimAccountNotFound          = errors.New("sim account not found")
	ErrSimInsufficientUSDT         = errors.New("insufficient simulated USDT")
	ErrSimInsufficientInventory    = errors.New("insufficient simulated spot inventory")
	ErrSimInsufficientMargin       = errors.New("insufficient simulated perp margin")
	ErrSimInvalidAmount            = errors.New("invalid simulated amount")
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

type CEXSpotPerpQuoteStatus struct {
	CEXSpotPerpQuote
	AgeMillis int64
	Stale     bool
}

// CEXSpotPerpConfig 是跨所期现策略的核心参数。第一版用固定滑点和费率，
// 后续可以把费用模型替换成交易所/VIP 等级维度的动态配置。
type CEXSpotPerpConfig struct {
	Symbols                []string
	Exchanges              []string
	ExchangeSymbols        map[string][]string
	NotionalUSDT           float64
	MinNetProfitRate       float64
	FundingIntervals       float64
	CarryFundingIntervals  float64
	SpotTakerFeeRate       float64
	PerpTakerFeeRate       float64
	SlippageRate           float64
	SafetyBufferRate       float64
	DefaultLeverage        float64
	MaxQuoteAgeMillis      int64
	EnableInventoryReverse bool
}

// DefaultCEXSpotPerpConfig 给出保守的模拟盘默认值，避免没有配置时过度高估收益。
func DefaultCEXSpotPerpConfig() CEXSpotPerpConfig {
	symbols := append([]string(nil), defaultCEXSpotPerpSymbols...)
	return CEXSpotPerpConfig{
		Symbols:   symbols,
		Exchanges: []string{"binance", "okx", "bitget"},
		ExchangeSymbols: map[string][]string{
			"binance": append([]string(nil), symbols...),
			"okx":     append([]string(nil), symbols...),
			"bitget":  append([]string(nil), symbols...),
		},
		NotionalUSDT:           1000,
		MinNetProfitRate:       0.2,
		FundingIntervals:       1,
		CarryFundingIntervals:  6,
		SpotTakerFeeRate:       0.001,
		PerpTakerFeeRate:       0.0005,
		SlippageRate:           0.0005,
		SafetyBufferRate:       0.0002,
		DefaultLeverage:        3,
		MaxQuoteAgeMillis:      15_000,
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
	if config.FundingIntervals == 0 {
		config.FundingIntervals = 1
	}
	if config.CarryFundingIntervals == 0 {
		config.CarryFundingIntervals = config.FundingIntervals
	}
	if len(config.ExchangeSymbols) == 0 {
		config.ExchangeSymbols = make(map[string][]string, len(config.Exchanges))
		for _, exchange := range config.Exchanges {
			config.ExchangeSymbols[exchange] = append([]string(nil), config.Symbols...)
		}
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

func (s *CEXSpotPerpStrategy) Config() CEXSpotPerpConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneCEXSpotPerpConfig(s.config)
}

func (s *CEXSpotPerpStrategy) UpdateConfig(config CEXSpotPerpConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cloneCEXSpotPerpConfig(config)
}

// UpdateQuote 更新单个盘口快照。收到任一侧价格后立即扫描该 symbol，
// 这样后续接 WebSocket 时可以做到事件驱动，而不是固定轮询。
func (s *CEXSpotPerpStrategy) UpdateQuote(q CEXSpotPerpQuote) {
	if q.Timestamp == 0 {
		q.Timestamp = time.Now().UnixMilli()
	}
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

// UpdateFunding 只更新永续资金费率，不覆盖 WebSocket 已经写入的 bid/ask。
// 资金费率通常低频变化，盘口却高频变化；分开更新可以避免 HTTP 轮询把实时盘口冲掉。
// 注意：这里不能刷新 Timestamp。Timestamp 表示盘口价格的新鲜度，
// 如果用资金费率轮询给旧 bid/ask 续命，会制造虚假的套利机会。
func (s *CEXSpotPerpStrategy) UpdateFunding(q CEXSpotPerpQuote) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.perpQuotes[q.Exchange]; !ok {
		s.perpQuotes[q.Exchange] = make(map[string]CEXSpotPerpQuote)
	}
	current := s.perpQuotes[q.Exchange][q.Symbol]
	current.Exchange = q.Exchange
	current.Symbol = q.Symbol
	current.MarketType = MarketTypePerp
	current.FundingRate = q.FundingRate
	s.perpQuotes[q.Exchange][q.Symbol] = current
}

// ScanSymbol 按“任意现货所 x 任意合约所”全组合扫描机会，包括同所期现。
func (s *CEXSpotPerpStrategy) ScanSymbol(symbol string) []*Opportunity {
	return s.scanSymbol(symbol, true)
}

// ScanSymbolCandidates 返回所有盘口新鲜的期现组合，包括暂时达不到收益阈值的候选项。
// 前端“机会扫描”需要看到为什么不能开仓，而不是空表让人误以为系统没工作。
func (s *CEXSpotPerpStrategy) ScanSymbolCandidates(symbol string) []*Opportunity {
	return s.scanSymbol(symbol, false)
}

func (s *CEXSpotPerpStrategy) QuoteStatuses() []CEXSpotPerpQuoteStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UnixMilli()
	statuses := make([]CEXSpotPerpQuoteStatus, 0)
	appendQuotes := func(source map[string]map[string]CEXSpotPerpQuote) {
		for _, quotes := range source {
			for _, quote := range quotes {
				age := int64(0)
				if quote.Timestamp > 0 {
					age = now - quote.Timestamp
				}
				statuses = append(statuses, CEXSpotPerpQuoteStatus{
					CEXSpotPerpQuote: quote,
					AgeMillis:        age,
					Stale:            s.isQuoteStale(quote),
				})
			}
		}
	}
	appendQuotes(s.spotQuotes)
	appendQuotes(s.perpQuotes)
	return statuses
}

func (s *CEXSpotPerpStrategy) ClosePrices(symbol, spotExchange, perpExchange, direction string) (float64, float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	spot, spotOK := s.spotQuotes[spotExchange][symbol]
	perp, perpOK := s.perpQuotes[perpExchange][symbol]
	if !spotOK || !perpOK || s.isQuoteStale(spot) || s.isQuoteStale(perp) {
		return 0, 0, false
	}
	switch direction {
	case DirectionSpotLongPerpShort:
		return spot.Bid, perp.Ask, spot.Bid > 0 && perp.Ask > 0
	case DirectionSpotShortInventoryPerpLong:
		return spot.Ask, perp.Bid, spot.Ask > 0 && perp.Bid > 0
	default:
		return 0, 0, false
	}
}

func (s *CEXSpotPerpStrategy) scanSymbol(symbol string, enforceMinProfit bool) []*Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var opportunities []*Opportunity
	for _, spotEx := range s.config.Exchanges {
		if !s.isExchangeSymbolEnabled(spotEx, symbol) {
			continue
		}
		spotQuote, ok := s.spotQuotes[spotEx][symbol]
		if !ok || spotQuote.Ask <= 0 || spotQuote.Bid <= 0 || s.isQuoteStale(spotQuote) {
			continue
		}
		for _, perpEx := range s.config.Exchanges {
			if !s.isExchangeSymbolEnabled(perpEx, symbol) {
				continue
			}
			perpQuote, ok := s.perpQuotes[perpEx][symbol]
			if !ok || perpQuote.Ask <= 0 || perpQuote.Bid <= 0 || s.isQuoteStale(perpQuote) {
				continue
			}
			if opp := s.buildOpportunityWithFilter(spotQuote, perpQuote, DirectionSpotLongPerpShort, enforceMinProfit); opp != nil {
				opportunities = append(opportunities, opp)
			}
			if s.config.EnableInventoryReverse {
				if opp := s.buildOpportunityWithFilter(spotQuote, perpQuote, DirectionSpotShortInventoryPerpLong, enforceMinProfit); opp != nil {
					opportunities = append(opportunities, opp)
				}
			}
		}
	}
	return opportunities
}

func (s *CEXSpotPerpStrategy) isExchangeSymbolEnabled(exchangeName, symbol string) bool {
	symbols, ok := s.config.ExchangeSymbols[exchangeName]
	if !ok {
		return true
	}
	for _, item := range symbols {
		if item == symbol {
			return true
		}
	}
	return false
}

func (s *CEXSpotPerpStrategy) isQuoteStale(q CEXSpotPerpQuote) bool {
	if s.config.MaxQuoteAgeMillis <= 0 {
		return false
	}
	return q.Timestamp == 0 || time.Now().UnixMilli()-q.Timestamp > s.config.MaxQuoteAgeMillis
}

func (s *CEXSpotPerpStrategy) buildOpportunity(spot, perp CEXSpotPerpQuote, direction string) *Opportunity {
	return s.buildOpportunityWithFilter(spot, perp, direction, true)
}

func (s *CEXSpotPerpStrategy) buildOpportunityWithFilter(spot, perp CEXSpotPerpQuote, direction string, enforceMinProfit bool) *Opportunity {
	notional := s.config.NotionalUSDT
	spotFee := notional * s.config.SpotTakerFeeRate
	perpFee := notional * s.config.PerpTakerFeeRate
	feeCost := spotFee + perpFee

	// 滑点不是手续费。这里单独计入安全成本，代表盘口深度不足或成交价漂移。
	slippageCost := notional * s.config.SlippageRate * 2
	safetyBuffer := notional * s.config.SafetyBufferRate

	var quantity, basisAmount, fundingAmount, carryFundingAmount, spotPrice, perpPrice float64
	switch direction {
	case DirectionSpotLongPerpShort:
		spotPrice = spot.Ask
		perpPrice = perp.Bid
		quantity = notional / spotPrice
		basisAmount = (perpPrice - spotPrice) * quantity
		fundingAmount = notional * perp.FundingRate * s.config.FundingIntervals
		carryFundingAmount = notional * perp.FundingRate * s.config.CarryFundingIntervals
	case DirectionSpotShortInventoryPerpLong:
		spotPrice = spot.Bid
		perpPrice = perp.Ask
		quantity = notional / spotPrice
		basisAmount = (spotPrice - perpPrice) * quantity
		// 负资金费率时，多永续收钱；正资金费率会成为成本。
		fundingAmount = -notional * perp.FundingRate * s.config.FundingIntervals
		carryFundingAmount = -notional * perp.FundingRate * s.config.CarryFundingIntervals
	default:
		return nil
	}

	grossProfit := basisAmount + fundingAmount
	netProfit := grossProfit - feeCost - slippageCost - safetyBuffer
	profitRate := netProfit / notional * 100
	// 持仓 carry 模型用更长的资金费率期数估算，不要求当前开仓瞬间就有明显价差。
	// 这更接近真实期现套利：小基差甚至略亏开仓，只要多期资金费率能覆盖成本，就值得进入观察。
	carryGrossProfit := basisAmount + carryFundingAmount
	carryNetProfit := carryGrossProfit - feeCost - slippageCost - safetyBuffer
	carryProfitRate := carryNetProfit / notional * 100
	if enforceMinProfit && profitRate < s.config.MinNetProfitRate {
		return nil
	}

	opp := &Opportunity{
		ID:                    generateID("csp", direction, spot.Exchange, perp.Exchange, spot.Symbol),
		StrategyType:          StrategyTypeCEXSpotPerp,
		Direction:             direction,
		Timestamp:             time.Now().UnixMilli(),
		Notional:              notional,
		ProfitRate:            profitRate,
		ProfitAmount:          grossProfit,
		BasisAmount:           basisAmount,
		FundingAmount:         fundingAmount,
		FeeCost:               feeCost,
		CarryFundingAmount:    carryFundingAmount,
		CarryNetProfit:        carryNetProfit,
		CarryProfitRate:       carryProfitRate,
		CarryFundingIntervals: s.config.CarryFundingIntervals,
		EstimatedGas:          feeCost,
		Slippage:              slippageCost,
		SafetyBuffer:          safetyBuffer,
		NetProfit:             netProfit,
		ExchangeA:             spot.Exchange,
		ExchangeB:             perp.Exchange,
		SpotExchange:          spot.Exchange,
		PerpExchange:          perp.Exchange,
		Symbol:                spot.Symbol,
		PriceA:                spotPrice,
		PriceB:                perpPrice,
		Legs:                  buildSpotPerpLegs(direction, spot.Exchange, perp.Exchange, spot.Symbol, quantity, spotPrice, perpPrice),
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
		"exchange_symbols":         s.config.ExchangeSymbols,
		"notional_usdt":            s.config.NotionalUSDT,
		"min_net_profit_rate":      s.config.MinNetProfitRate,
		"funding_intervals":        s.config.FundingIntervals,
		"carry_funding_intervals":  s.config.CarryFundingIntervals,
		"max_quote_age_millis":     s.config.MaxQuoteAgeMillis,
		"enable_inventory_reverse": s.config.EnableInventoryReverse,
	}
}

func cloneCEXSpotPerpConfig(config CEXSpotPerpConfig) CEXSpotPerpConfig {
	config.Symbols = append([]string(nil), config.Symbols...)
	config.Exchanges = append([]string(nil), config.Exchanges...)
	cloned := make(map[string][]string, len(config.ExchangeSymbols))
	for exchange, symbols := range config.ExchangeSymbols {
		cloned[exchange] = append([]string(nil), symbols...)
	}
	config.ExchangeSymbols = cloned
	return config
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
	if leverage > 3 {
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

func (s *CEXSpotPerpSimulator) SetLeverage(leverage float64) {
	if leverage <= 0 {
		leverage = 1
	}
	if leverage > 3 {
		leverage = 3
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 第一版明确把模拟合约杠杆限制在 3 倍以内，避免前端或外部调用绕过 API 校验后
	// 低估保证金占用。实盘时还要结合交易所逐仓/全仓规则再次校验。
	s.leverage = leverage
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

func (s *CEXSpotPerpSimulator) Accounts() []*SimAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*SimAccount, 0, len(s.accounts))
	for _, account := range s.accounts {
		result = append(result, cloneSimAccount(account))
	}
	return result
}

func (s *CEXSpotPerpSimulator) Account(exchangeName string) (*SimAccount, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	account, ok := s.accounts[exchangeName]
	return cloneSimAccount(account), ok
}

func (s *CEXSpotPerpSimulator) UpdateAccount(exchangeName string, usdt, perpUSDT float64, spotBalances map[string]float64) error {
	if usdt < 0 || perpUSDT < 0 {
		return ErrSimInvalidAmount
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[exchangeName]
	if !ok || account == nil {
		return fmt.Errorf("%w: %s", ErrSimAccountNotFound, exchangeName)
	}
	if perpUSDT+1e-12 < account.FrozenUSDT {
		return ErrSimInsufficientMargin
	}

	normalizedBalances := make(map[string]float64, len(spotBalances))
	for asset, balance := range spotBalances {
		asset = strings.ToUpper(strings.TrimSpace(asset))
		if asset == "" {
			continue
		}
		if balance < 0 {
			return ErrSimInvalidAmount
		}
		normalizedBalances[asset] = balance
	}

	// 资金管理允许调整模拟盘初始资金和手动库存，但保留已开永续持仓和冻结保证金。
	// 这样不会因为页面改配置把已经建立的对冲关系悄悄抹掉。
	account.USDT = usdt
	account.PerpUSDT = perpUSDT
	account.SpotBalances = normalizedBalances
	if account.PerpPositions == nil {
		account.PerpPositions = make(map[string]float64)
	}
	return nil
}

func (s *CEXSpotPerpSimulator) TransferUSDT(exchangeName, from, to string, amount float64) error {
	if amount <= 0 {
		return ErrSimInvalidAmount
	}
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if from == to || (from != MarketTypeSpot && from != MarketTypePerp) || (to != MarketTypeSpot && to != MarketTypePerp) {
		return ErrSimInvalidAmount
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[exchangeName]
	if !ok || account == nil {
		return fmt.Errorf("%w: %s", ErrSimAccountNotFound, exchangeName)
	}

	switch from {
	case MarketTypeSpot:
		if account.USDT+1e-12 < amount {
			return ErrSimInsufficientUSDT
		}
		account.USDT -= amount
		account.PerpUSDT += amount
	case MarketTypePerp:
		available := account.PerpUSDT - account.FrozenUSDT
		if available+1e-12 < amount {
			return ErrSimInsufficientMargin
		}
		account.PerpUSDT -= amount
		account.USDT += amount
	}
	return nil
}

func (s *CEXSpotPerpSimulator) ResetAccounts(accounts map[string]*SimAccount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts = make(map[string]*SimAccount, len(accounts))
	for name, account := range accounts {
		s.accounts[name] = cloneSimAccount(account)
	}
	s.positions = make(map[string]*SimArbitragePosition)
	s.closeActions = nil
	s.halted = false
	s.haltReason = ""
}

func (s *CEXSpotPerpSimulator) RestoreState(accounts map[string]*SimAccount, positions []*SimArbitragePosition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts = make(map[string]*SimAccount, len(accounts))
	for name, account := range accounts {
		s.accounts[name] = cloneSimAccount(account)
	}
	s.positions = make(map[string]*SimArbitragePosition, len(positions))
	for _, pos := range positions {
		if pos == nil || pos.ID == "" {
			continue
		}
		copyPos := *pos
		if pos.Opportunity != nil {
			opp := *pos.Opportunity
			opp.Legs = append([]Leg(nil), pos.Opportunity.Legs...)
			copyPos.Opportunity = &opp
		}
		copyPos.Trades = append([]SimTrade(nil), pos.Trades...)
		s.positions[copyPos.ID] = &copyPos
	}
	s.closeActions = nil
	s.halted = false
	s.haltReason = ""
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

func (s *CEXSpotPerpSimulator) Position(positionID string) (*SimArbitragePosition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pos, ok := s.positions[positionID]
	if !ok || pos == nil {
		return nil, false
	}
	copyPos := *pos
	if pos.Opportunity != nil {
		opp := *pos.Opportunity
		opp.Legs = append([]Leg(nil), pos.Opportunity.Legs...)
		copyPos.Opportunity = &opp
	}
	copyPos.Trades = append([]SimTrade(nil), pos.Trades...)
	return &copyPos, true
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

func (s *CEXSpotPerpSimulator) reservedSpotInventoryLocked(exchangeName, baseAsset string) float64 {
	var reserved float64
	for _, pos := range s.positions {
		if pos == nil || pos.Status != "open" || pos.Opportunity == nil {
			continue
		}
		opp := pos.Opportunity
		if opp.Direction != DirectionSpotLongPerpShort || opp.SpotExchange != exchangeName {
			continue
		}
		if baseAssetFromSymbol(opp.Symbol) != baseAsset {
			continue
		}
		// 买现货 + 空永续中的现货是当前套利组合的对冲腿，
		// 平仓前不能再被第二阶段“卖库存 + 多永续”重复使用。
		reserved += spotPerpQuantity(opp)
	}
	return reserved
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
		availableInventory := spotAccount.SpotBalances[baseAsset] - s.reservedSpotInventoryLocked(opp.SpotExchange, baseAsset)
		if availableInventory+1e-12 < quantity {
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

func (s *CEXSpotPerpSimulator) ClosePosition(positionID, reason string) (*SimCloseAction, error) {
	return s.ClosePositionWithMarket(positionID, reason, 0, 0)
}

func (s *CEXSpotPerpSimulator) EstimateClosePnL(positionID string, closeSpotPrice, closePerpPrice float64) (float64, float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pos, ok := s.positions[positionID]
	if !ok || pos == nil {
		return 0, 0, ErrSimPositionNotFound
	}
	if pos.Status != "open" {
		return 0, 0, ErrSimPositionNotOpen
	}
	pnl := estimateClosePnLLocked(pos, closeSpotPrice, closePerpPrice)
	rate := 0.0
	if pos.Notional > 0 {
		rate = pnl / pos.Notional * 100
	}
	return pnl, rate, nil
}

// ClosePositionWithMarket 执行模拟平仓。平仓价格来自当前盘口估算：
// 正向组合用现货 bid 卖出、永续 ask 买回；反向组合用现货 ask 买回、永续 bid 卖出。
// 这能让“预计收益”和“实际收益”分开，后续接实盘时应替换为交易所成交回报。
func (s *CEXSpotPerpSimulator) ClosePositionWithMarket(positionID, reason string, closeSpotPrice, closePerpPrice float64) (*SimCloseAction, error) {
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
	if closeSpotPrice <= 0 {
		closeSpotPrice = opp.PriceA
	}
	if closePerpPrice <= 0 {
		closePerpPrice = opp.PriceB
	}
	spotCloseValue := quantity * closeSpotPrice
	perpCloseValue := quantity * closePerpPrice
	switch opp.Direction {
	case DirectionSpotLongPerpShort:
		spotAccount.SpotBalances[baseAsset] = math.Max(0, spotAccount.SpotBalances[baseAsset]-quantity)
		spotAccount.USDT += spotCloseValue - closeFeeForMarket(pos, MarketTypeSpot, spotCloseValue)
		perpPnL := (opp.PriceB - closePerpPrice) * quantity
		perpAccount.PerpUSDT += perpPnL - closeFeeForMarket(pos, MarketTypePerp, perpCloseValue)
		perpAccount.PerpPositions[opp.Symbol] += quantity
	case DirectionSpotShortInventoryPerpLong:
		spotAccount.USDT = math.Max(0, spotAccount.USDT-spotCloseValue-closeFeeForMarket(pos, MarketTypeSpot, spotCloseValue))
		spotAccount.SpotBalances[baseAsset] += quantity
		perpPnL := (closePerpPrice - opp.PriceB) * quantity
		perpAccount.PerpUSDT += perpPnL - closeFeeForMarket(pos, MarketTypePerp, perpCloseValue)
		perpAccount.PerpPositions[opp.Symbol] -= quantity
	default:
		return nil, ErrSimUnsupportedDirection
	}

	perpAccount.FrozenUSDT = math.Max(0, perpAccount.FrozenUSDT-pos.Margin)
	pos.Status = "closed"
	pos.ClosedAt = time.Now().UnixMilli()
	pos.RealizedPnL = estimateClosePnLLocked(pos, closeSpotPrice, closePerpPrice)

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

func openingFees(pos *SimArbitragePosition) (float64, float64) {
	var spotFee, perpFee float64
	if pos == nil {
		return 0, 0
	}
	for _, trade := range pos.Trades {
		switch trade.Market {
		case MarketTypeSpot:
			spotFee += trade.Fee
		case MarketTypePerp:
			perpFee += trade.Fee
		}
	}
	return spotFee, perpFee
}

func estimateClosePnLLocked(pos *SimArbitragePosition, closeSpotPrice, closePerpPrice float64) float64 {
	if pos == nil || pos.Opportunity == nil {
		return 0
	}
	opp := pos.Opportunity
	if closeSpotPrice <= 0 {
		closeSpotPrice = opp.PriceA
	}
	if closePerpPrice <= 0 {
		closePerpPrice = opp.PriceB
	}
	quantity := spotPerpQuantity(opp)
	spotOpenFee, perpOpenFee := openingFees(pos)
	spotCloseValue := quantity * closeSpotPrice
	perpCloseValue := quantity * closePerpPrice
	spotCloseFee := closeFeeForMarket(pos, MarketTypeSpot, spotCloseValue)
	perpCloseFee := closeFeeForMarket(pos, MarketTypePerp, perpCloseValue)
	closeSlippage := estimatedCloseSlippage(opp, spotCloseValue+perpCloseValue)
	fundingPnL := opp.FundingAmount

	var grossPnL float64
	switch opp.Direction {
	case DirectionSpotLongPerpShort:
		perpPnL := (opp.PriceB - closePerpPrice) * quantity
		grossPnL = (closeSpotPrice-opp.PriceA)*quantity + perpPnL + fundingPnL
	case DirectionSpotShortInventoryPerpLong:
		perpPnL := (closePerpPrice - opp.PriceB) * quantity
		grossPnL = (opp.PriceA-closeSpotPrice)*quantity + perpPnL + fundingPnL
	default:
		return 0
	}
	return grossPnL - spotOpenFee - perpOpenFee - spotCloseFee - perpCloseFee - closeSlippage
}

func closeFeeForMarket(pos *SimArbitragePosition, market string, closeValue float64) float64 {
	if pos == nil || pos.Opportunity == nil || closeValue <= 0 {
		return 0
	}
	fallbackRate := 0.0005
	if market == MarketTypeSpot {
		fallbackRate = 0.001
	}
	return closeValue * feeRateFromTrade(pos, market, pos.Opportunity.Notional, fallbackRate)
}

func feeRateFromTrade(pos *SimArbitragePosition, market string, fallbackNotional, fallbackRate float64) float64 {
	if pos != nil {
		for _, trade := range pos.Trades {
			if trade.Market != market {
				continue
			}
			value := math.Abs(trade.Quantity * trade.Price)
			if value > 0 && trade.Fee > 0 {
				return trade.Fee / value
			}
		}
	}
	if fallbackNotional > 0 {
		return fallbackRate
	}
	return fallbackRate
}

func estimatedCloseSlippage(opp *Opportunity, closeValue float64) float64 {
	if opp == nil || opp.Notional <= 0 || opp.Slippage <= 0 || closeValue <= 0 {
		return 0
	}
	return closeValue * (opp.Slippage / (opp.Notional * 2))
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
