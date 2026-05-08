package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/model"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
)

const cexSpotPerpAutomationSettingName = "cex_spot_perp"

var cexSpotPerpWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type cexSpotPerpState struct {
	mu                sync.RWMutex
	strategy          *strategy.CEXSpotPerpStrategy
	simulator         *strategy.CEXSpotPerpSimulator
	opportunities     map[string]*strategy.Opportunity
	opportunityLogs   map[string]*cexSpotPerpOpportunityLog
	autoTrades        []cexSpotPerpAutoTrade
	symbols           []string
	exchanges         []string
	exchangeSymbols   map[string][]string
	minProfitRate     float64
	leverage          float64
	automation        cexSpotPerpAutomationConfig
	autoStats         cexSpotPerpAutoStats
	lastAutoCheckAt   int64
	persistenceLoaded bool
	wsGeneration      int64
	lastQuoteAt       int64
	marketErrors      map[string]string
	wsStatus          map[string]string
	wsErrors          map[string]string
	httpClient        *http.Client
}

type cexSpotPerpAutomationConfig struct {
	Enabled             bool    `json:"enabled"`
	AutoOpen            bool    `json:"autoOpen"`
	AutoClose           bool    `json:"autoClose"`
	OpenMinProfitRate   float64 `json:"openMinProfitRate"`
	CloseMinProfitRate  float64 `json:"closeMinProfitRate"`
	MaxHoldSeconds      int64   `json:"maxHoldSeconds"`
	MaxOpenPositions    int     `json:"maxOpenPositions"`
	CheckIntervalMillis int64   `json:"checkIntervalMillis"`
}

type cexSpotPerpAutoStats struct {
	AutoOpenCount   int     `json:"autoOpenCount"`
	AutoCloseCount  int     `json:"autoCloseCount"`
	WinCount        int     `json:"winCount"`
	LossCount       int     `json:"lossCount"`
	TotalProfit     float64 `json:"totalProfit"`
	AverageProfit   float64 `json:"averageProfit"`
	WinRate         float64 `json:"winRate"`
	LastActionAt    int64   `json:"lastActionAt"`
	LastActionError string  `json:"lastActionError"`
}

type cexSpotPerpOpportunityLog struct {
	Key              string
	ID               string
	Symbol           string
	Direction        string
	SpotExchange     string
	PerpExchange     string
	FirstSeenAt      int64
	LastSeenAt       int64
	SeenCount        int
	BestProfit       float64
	BestProfitRate   float64
	LastProfit       float64
	LastProfitRate   float64
	LastStatus       string
	LastBlockReason  string
	AutoOpenedCount  int
	AutoRejectedNote string
}

type cexSpotPerpAutoTrade struct {
	ID           string
	PositionID   string
	Opportunity  string
	Symbol       string
	Direction    string
	SpotExchange string
	PerpExchange string
	Action       string
	Reason       string
	Quantity     float64
	Notional     float64
	Margin       float64
	SpotValue    float64
	CapitalUsed  float64
	Profit       float64
	ProfitRate   float64
	CreatedAt    int64
}

var cexSpotPerpSim = newCEXSpotPerpState()

func newCEXSpotPerpState() *cexSpotPerpState {
	cfg := strategy.DefaultCEXSpotPerpConfig()
	cfg.NotionalUSDT = 3000
	cfg.MinNetProfitRate = 0.05
	cfg.CarryFundingIntervals = 6
	s := strategy.NewCEXSpotPerpStrategy(cfg)

	// 第一版 API 使用内存模拟盘：行情、资金、持仓都在服务端统一维护，
	// 前端只负责展示和触发动作，避免前端刷新后状态各跑各的。
	quotes := []strategy.CEXSpotPerpQuote{
		{Exchange: "binance", Symbol: "BTCUSDT", MarketType: strategy.MarketTypeSpot, Bid: 67480, Ask: 67500},
		{Exchange: "okx", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 67920, Ask: 67940, FundingRate: 0.0012},
		{Exchange: "bitget", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 67880, Ask: 67910, FundingRate: 0.0010},
		{Exchange: "bitget", Symbol: "ETHUSDT", MarketType: strategy.MarketTypeSpot, Bid: 3488, Ask: 3490},
		{Exchange: "binance", Symbol: "ETHUSDT", MarketType: strategy.MarketTypePerp, Bid: 3460, Ask: 3462, FundingRate: -0.0011},
		{Exchange: "okx", Symbol: "SOLUSDT", MarketType: strategy.MarketTypeSpot, Bid: 154.1, Ask: 154.2},
		{Exchange: "bitget", Symbol: "SOLUSDT", MarketType: strategy.MarketTypePerp, Bid: 154.7, Ask: 154.8, FundingRate: 0.0009},
	}
	for _, quote := range quotes {
		s.UpdateQuote(quote)
	}

	sim := strategy.NewCEXSpotPerpSimulator(defaultCEXSpotPerpAccounts(), cfg.DefaultLeverage)

	state := &cexSpotPerpState{
		strategy:        s,
		simulator:       sim,
		opportunities:   make(map[string]*strategy.Opportunity),
		opportunityLogs: make(map[string]*cexSpotPerpOpportunityLog),
		symbols:         cfg.Symbols,
		exchanges:       cfg.Exchanges,
		exchangeSymbols: cloneStringSliceMap(cfg.ExchangeSymbols),
		minProfitRate:   cfg.MinNetProfitRate,
		leverage:        cfg.DefaultLeverage,
		automation: cexSpotPerpAutomationConfig{
			Enabled:             false,
			AutoOpen:            true,
			AutoClose:           true,
			OpenMinProfitRate:   cfg.MinNetProfitRate,
			CloseMinProfitRate:  0,
			MaxHoldSeconds:      300,
			MaxOpenPositions:    3,
			CheckIntervalMillis: 1000,
		},
		lastQuoteAt:  time.Now().UnixMilli(),
		marketErrors: make(map[string]string),
		wsStatus:     make(map[string]string),
		wsErrors:     make(map[string]string),
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}
	state.startOfficialMarketWebSockets()
	state.startFundingPolling()
	return state
}

func defaultCEXSpotPerpAccounts() map[string]*strategy.SimAccount {
	return map[string]*strategy.SimAccount{
		"binance": {
			Exchange:      "binance",
			USDT:          12000,
			PerpUSDT:      10000,
			SpotBalances:  map[string]float64{"BTC": 0.12, "ETH": 1.8, "SOL": 40},
			PerpPositions: map[string]float64{},
		},
		"okx": {
			Exchange:      "okx",
			USDT:          9000,
			PerpUSDT:      12000,
			SpotBalances:  map[string]float64{"BTC": 0.04, "ETH": 0.5, "SOL": 20},
			PerpPositions: map[string]float64{},
		},
		"bitget": {
			Exchange:      "bitget",
			USDT:          7000,
			PerpUSDT:      8000,
			SpotBalances:  map[string]float64{"BTC": 0.02, "ETH": 1.2, "SOL": 15},
			PerpPositions: map[string]float64{},
		},
	}
}

func (s *cexSpotPerpState) ensurePersistenceLoaded() {
	if database.DB == nil {
		return
	}
	s.mu.Lock()
	if s.persistenceLoaded {
		s.mu.Unlock()
		return
	}
	s.persistenceLoaded = true
	s.mu.Unlock()

	var setting model.CEXSpotPerpAutomationSetting
	if err := database.DB.First(&setting, "name = ?", cexSpotPerpAutomationSettingName).Error; err == nil {
		s.mu.Lock()
		s.automation = cexSpotPerpAutomationConfig{
			Enabled:             setting.Enabled,
			AutoOpen:            setting.AutoOpen,
			AutoClose:           setting.AutoClose,
			OpenMinProfitRate:   setting.OpenMinProfitRate,
			CloseMinProfitRate:  setting.CloseMinProfitRate,
			MaxHoldSeconds:      setting.MaxHoldSeconds,
			MaxOpenPositions:    setting.MaxOpenPositions,
			CheckIntervalMillis: setting.CheckIntervalMillis,
		}
		s.mu.Unlock()
	} else {
		s.persistAutomationSetting()
	}

	var logs []model.CEXSpotPerpOpportunityLog
	if err := database.DB.Order("last_seen_at DESC").Limit(1000).Find(&logs).Error; err == nil {
		s.mu.Lock()
		for _, item := range logs {
			s.opportunityLogs[item.Key] = &cexSpotPerpOpportunityLog{
				Key:              item.Key,
				ID:               item.OpportunityID,
				Symbol:           item.Symbol,
				Direction:        item.Direction,
				SpotExchange:     item.SpotExchange,
				PerpExchange:     item.PerpExchange,
				FirstSeenAt:      item.FirstSeenAt,
				LastSeenAt:       item.LastSeenAt,
				SeenCount:        item.SeenCount,
				BestProfit:       item.BestProfit,
				BestProfitRate:   item.BestProfitRate,
				LastProfit:       item.LastProfit,
				LastProfitRate:   item.LastProfitRate,
				LastStatus:       item.LastStatus,
				LastBlockReason:  item.LastBlockReason,
				AutoOpenedCount:  item.AutoOpenedCount,
				AutoRejectedNote: item.AutoRejectedNote,
			}
		}
		s.mu.Unlock()
	}

	var trades []model.CEXSpotPerpAutoTrade
	if err := database.DB.Order("created_at_ms ASC").Find(&trades).Error; err == nil {
		s.mu.Lock()
		s.autoTrades = make([]cexSpotPerpAutoTrade, 0, len(trades))
		s.autoStats = cexSpotPerpAutoStats{}
		for _, item := range trades {
			trade := cexSpotPerpAutoTrade{
				ID:           item.ID,
				PositionID:   item.PositionID,
				Opportunity:  item.Opportunity,
				Symbol:       item.Symbol,
				Direction:    item.Direction,
				SpotExchange: item.SpotExchange,
				PerpExchange: item.PerpExchange,
				Action:       item.Action,
				Reason:       item.Reason,
				Quantity:     item.Quantity,
				Notional:     item.Notional,
				Margin:       item.Margin,
				SpotValue:    item.SpotValue,
				CapitalUsed:  item.CapitalUsed,
				Profit:       item.Profit,
				ProfitRate:   item.ProfitRate,
				CreatedAt:    item.CreatedAtMs,
			}
			s.autoTrades = append(s.autoTrades, trade)
			s.applyAutoTradeStatsLocked(trade)
		}
		s.mu.Unlock()
	}

	var storedAccounts []model.CEXSpotPerpSimAccount
	if err := database.DB.Find(&storedAccounts).Error; err == nil && len(storedAccounts) > 0 {
		accounts := defaultCEXSpotPerpAccounts()
		for _, item := range storedAccounts {
			account, err := simAccountFromModel(item)
			if err != nil {
				continue
			}
			accounts[account.Exchange] = account
		}
		positions := s.loadPersistedSimPositions()
		s.simulator.RestoreState(accounts, positions)
	} else {
		s.persistSimState()
	}
}

func (s *cexSpotPerpState) persistAutomationSetting() {
	if database.DB == nil {
		return
	}
	s.mu.RLock()
	cfg := s.automation
	s.mu.RUnlock()
	setting := model.CEXSpotPerpAutomationSetting{
		Name:                cexSpotPerpAutomationSettingName,
		Enabled:             cfg.Enabled,
		AutoOpen:            cfg.AutoOpen,
		AutoClose:           cfg.AutoClose,
		OpenMinProfitRate:   cfg.OpenMinProfitRate,
		CloseMinProfitRate:  cfg.CloseMinProfitRate,
		MaxHoldSeconds:      cfg.MaxHoldSeconds,
		MaxOpenPositions:    cfg.MaxOpenPositions,
		CheckIntervalMillis: cfg.CheckIntervalMillis,
		UpdatedAt:           time.Now(),
	}
	_ = database.DB.Save(&setting).Error
}

func persistCEXSpotPerpOpportunityLog(log cexSpotPerpOpportunityLog) {
	if database.DB == nil {
		return
	}
	item := model.CEXSpotPerpOpportunityLog{
		Key:              log.Key,
		OpportunityID:    log.ID,
		Symbol:           log.Symbol,
		Direction:        log.Direction,
		SpotExchange:     log.SpotExchange,
		PerpExchange:     log.PerpExchange,
		FirstSeenAt:      log.FirstSeenAt,
		LastSeenAt:       log.LastSeenAt,
		SeenCount:        log.SeenCount,
		BestProfit:       log.BestProfit,
		BestProfitRate:   log.BestProfitRate,
		LastProfit:       log.LastProfit,
		LastProfitRate:   log.LastProfitRate,
		LastStatus:       log.LastStatus,
		LastBlockReason:  log.LastBlockReason,
		AutoOpenedCount:  log.AutoOpenedCount,
		AutoRejectedNote: log.AutoRejectedNote,
		UpdatedAt:        time.Now(),
	}
	_ = database.DB.Save(&item).Error
}

func persistCEXSpotPerpAutoTrade(trade cexSpotPerpAutoTrade) {
	if database.DB == nil {
		return
	}
	item := model.CEXSpotPerpAutoTrade{
		ID:           trade.ID,
		PositionID:   trade.PositionID,
		Opportunity:  trade.Opportunity,
		Symbol:       trade.Symbol,
		Direction:    trade.Direction,
		SpotExchange: trade.SpotExchange,
		PerpExchange: trade.PerpExchange,
		Action:       trade.Action,
		Reason:       trade.Reason,
		Quantity:     trade.Quantity,
		Notional:     trade.Notional,
		Margin:       trade.Margin,
		SpotValue:    trade.SpotValue,
		CapitalUsed:  trade.CapitalUsed,
		Profit:       trade.Profit,
		ProfitRate:   trade.ProfitRate,
		CreatedAtMs:  trade.CreatedAt,
		CreatedAt:    time.UnixMilli(trade.CreatedAt),
	}
	_ = database.DB.Save(&item).Error
}

func (s *cexSpotPerpState) persistSimAccounts() {
	if database.DB == nil {
		return
	}
	now := time.Now()
	for _, account := range s.simulator.Accounts() {
		if account == nil {
			continue
		}
		item, err := simAccountToModel(account, now)
		if err != nil {
			continue
		}
		_ = database.DB.Save(&item).Error
	}
}

func (s *cexSpotPerpState) persistSimState() {
	s.persistSimAccounts()
	s.persistSimPositions()
}

func (s *cexSpotPerpState) persistSimPositions() {
	if database.DB == nil {
		return
	}
	positions := s.simulator.Positions()
	active := make(map[string]bool, len(positions))
	now := time.Now()
	for _, pos := range positions {
		if pos == nil || pos.ID == "" {
			continue
		}
		item, err := simPositionToModel(pos, now)
		if err != nil {
			continue
		}
		active[pos.ID] = true
		_ = database.DB.Save(&item).Error
	}
	var stored []model.CEXSpotPerpSimPosition
	if err := database.DB.Find(&stored).Error; err != nil {
		return
	}
	for _, item := range stored {
		if !active[item.ID] {
			_ = database.DB.Delete(&model.CEXSpotPerpSimPosition{}, "id = ?", item.ID).Error
		}
	}
}

func (s *cexSpotPerpState) loadPersistedSimPositions() []*strategy.SimArbitragePosition {
	if database.DB == nil {
		return nil
	}
	var stored []model.CEXSpotPerpSimPosition
	if err := database.DB.Order("opened_at ASC").Find(&stored).Error; err != nil {
		return nil
	}
	positions := make([]*strategy.SimArbitragePosition, 0, len(stored))
	for _, item := range stored {
		pos, err := simPositionFromModel(item)
		if err != nil {
			continue
		}
		positions = append(positions, pos)
	}
	return positions
}

func simAccountToModel(account *strategy.SimAccount, now time.Time) (model.CEXSpotPerpSimAccount, error) {
	spotBalances, err := json.Marshal(account.SpotBalances)
	if err != nil {
		return model.CEXSpotPerpSimAccount{}, err
	}
	perpPositions, err := json.Marshal(account.PerpPositions)
	if err != nil {
		return model.CEXSpotPerpSimAccount{}, err
	}
	return model.CEXSpotPerpSimAccount{
		Exchange:      account.Exchange,
		USDT:          account.USDT,
		PerpUSDT:      account.PerpUSDT,
		FrozenUSDT:    account.FrozenUSDT,
		SpotBalances:  string(spotBalances),
		PerpPositions: string(perpPositions),
		UpdatedAt:     now,
	}, nil
}

func simAccountFromModel(item model.CEXSpotPerpSimAccount) (*strategy.SimAccount, error) {
	spotBalances := make(map[string]float64)
	if strings.TrimSpace(item.SpotBalances) != "" {
		if err := json.Unmarshal([]byte(item.SpotBalances), &spotBalances); err != nil {
			return nil, err
		}
	}
	perpPositions := make(map[string]float64)
	if strings.TrimSpace(item.PerpPositions) != "" {
		if err := json.Unmarshal([]byte(item.PerpPositions), &perpPositions); err != nil {
			return nil, err
		}
	}
	return &strategy.SimAccount{
		Exchange:      item.Exchange,
		USDT:          item.USDT,
		PerpUSDT:      item.PerpUSDT,
		FrozenUSDT:    item.FrozenUSDT,
		SpotBalances:  spotBalances,
		PerpPositions: perpPositions,
	}, nil
}

func simPositionToModel(pos *strategy.SimArbitragePosition, now time.Time) (model.CEXSpotPerpSimPosition, error) {
	opportunityJSON, err := json.Marshal(pos.Opportunity)
	if err != nil {
		return model.CEXSpotPerpSimPosition{}, err
	}
	tradesJSON, err := json.Marshal(pos.Trades)
	if err != nil {
		return model.CEXSpotPerpSimPosition{}, err
	}
	symbol := ""
	direction := ""
	spotExchange := ""
	perpExchange := ""
	opportunityID := ""
	if pos.Opportunity != nil {
		symbol = pos.Opportunity.Symbol
		direction = pos.Opportunity.Direction
		spotExchange = pos.Opportunity.SpotExchange
		perpExchange = pos.Opportunity.PerpExchange
		opportunityID = pos.Opportunity.ID
	}
	return model.CEXSpotPerpSimPosition{
		ID:              pos.ID,
		OpportunityID:   opportunityID,
		Symbol:          symbol,
		Direction:       direction,
		SpotExchange:    spotExchange,
		PerpExchange:    perpExchange,
		Status:          pos.Status,
		OpenedAt:        pos.OpenedAt,
		ClosedAt:        pos.ClosedAt,
		Notional:        pos.Notional,
		Margin:          pos.Margin,
		RealizedPnL:     pos.RealizedPnL,
		OpportunityJSON: string(opportunityJSON),
		TradesJSON:      string(tradesJSON),
		UpdatedAt:       now,
	}, nil
}

func simPositionFromModel(item model.CEXSpotPerpSimPosition) (*strategy.SimArbitragePosition, error) {
	var opp strategy.Opportunity
	if strings.TrimSpace(item.OpportunityJSON) != "" {
		if err := json.Unmarshal([]byte(item.OpportunityJSON), &opp); err != nil {
			return nil, err
		}
	}
	var trades []strategy.SimTrade
	if strings.TrimSpace(item.TradesJSON) != "" {
		if err := json.Unmarshal([]byte(item.TradesJSON), &trades); err != nil {
			return nil, err
		}
	}
	return &strategy.SimArbitragePosition{
		ID:          item.ID,
		Opportunity: &opp,
		Status:      item.Status,
		OpenedAt:    item.OpenedAt,
		ClosedAt:    item.ClosedAt,
		Trades:      trades,
		Notional:    item.Notional,
		Margin:      item.Margin,
		RealizedPnL: item.RealizedPnL,
	}, nil
}

func (s *cexSpotPerpState) startOfficialMarketWebSockets() {
	s.mu.Lock()
	s.wsGeneration++
	generation := s.wsGeneration
	s.mu.Unlock()
	s.launchOfficialMarketWebSockets(generation)
}

func (s *cexSpotPerpState) launchOfficialMarketWebSockets(generation int64) {
	enabled := make(map[string]bool)
	for _, exchangeName := range s.enabledExchanges() {
		enabled[exchangeName] = true
	}
	if enabled["binance"] {
		go s.runBinanceBookTickerWS(strategy.MarketTypeSpot, generation)
		go s.runBinanceBookTickerWS(strategy.MarketTypePerp, generation)
	}
	if enabled["okx"] {
		go s.runOKXTickerWS(generation)
	}
	if enabled["bitget"] {
		go s.runBitgetTickerWS(strategy.MarketTypeSpot, generation)
		go s.runBitgetTickerWS(strategy.MarketTypePerp, generation)
	}
}

func (s *cexSpotPerpState) startFundingPolling() {
	go func() {
		s.pollFundingOnce()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.pollFundingOnce()
		}
	}()
}

func (s *cexSpotPerpState) pollFundingOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, exchangeName := range s.enabledExchanges() {
		for _, symbol := range s.symbolsForExchange(exchangeName) {
			wg.Add(1)
			go func(ex, sym string) {
				defer wg.Done()
				funding, err := s.fetchFundingRate(ctx, ex, sym)
				if err != nil {
					s.setMarketError(ex, sym, "funding", err)
					return
				}
				s.updatePerpFunding(ex, sym, funding)
				s.clearMarketError(ex, sym, "funding")
			}(exchangeName, symbol)
		}
	}
	wg.Wait()
}

func (s *cexSpotPerpState) enabledSymbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.symbols...)
}

func (s *cexSpotPerpState) enabledExchanges() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.exchanges...)
}

func (s *cexSpotPerpState) symbolsForExchange(exchangeName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	enabled := false
	for _, item := range s.exchanges {
		if item == exchangeName {
			enabled = true
			break
		}
	}
	if !enabled {
		return nil
	}
	if symbols, ok := s.exchangeSymbols[exchangeName]; ok {
		return append([]string(nil), symbols...)
	}
	return append([]string(nil), s.symbols...)
}

func (s *cexSpotPerpState) isWSGeneration(generation int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wsGeneration == generation
}

func (s *cexSpotPerpState) runBinanceBookTickerWS(marketType string, generation int64) {
	name := "binance:" + marketType
	next := 0
	for s.isWSGeneration(generation) {
		s.setWSStatus(name, "connecting")
		endpoints := s.binanceBookTickerEndpoints(marketType)
		if len(endpoints) == 0 {
			s.setWSError(name, fmt.Errorf("no enabled binance symbols"))
			time.Sleep(3 * time.Second)
			continue
		}
		endpoint := endpoints[next%len(endpoints)]
		next++
		if err := s.consumeBinanceBookTicker(name, marketType, endpoint, generation); err != nil {
			s.setWSError(name, err)
			time.Sleep(3 * time.Second)
		}
	}
}

func (s *cexSpotPerpState) binanceBookTickerEndpoints(marketType string) []string {
	streams := make([]string, 0)
	for _, symbol := range s.symbolsForExchange("binance") {
		streams = append(streams, strings.ToLower(symbol)+"@bookTicker")
	}
	if len(streams) == 0 {
		return nil
	}
	streamPath := strings.Join(streams, "/")
	if marketType == strategy.MarketTypePerp {
		return []string{
			"wss://fstream.binance.com/stream?streams=" + streamPath,
		}
	}
	return []string{
		"wss://stream.binance.com:9443/stream?streams=" + streamPath,
		"wss://stream.binance.com:443/stream?streams=" + streamPath,
		"wss://data-stream.binance.vision/stream?streams=" + streamPath,
	}
}

func (s *cexSpotPerpState) consumeBinanceBookTicker(name, marketType, endpoint string, generation int64) error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 8 * time.Second,
	}
	conn, _, err := dialer.Dial(endpoint, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	s.setWSStatus(name, "connected")

	for {
		if !s.isWSGeneration(generation) {
			return nil
		}
		var msg struct {
			Data struct {
				Symbol      string `json:"s"`
				BidPrice    string `json:"b"`
				BidQuantity string `json:"B"`
				AskPrice    string `json:"a"`
				AskQuantity string `json:"A"`
			} `json:"data"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		if msg.Data.Symbol == "" || msg.Data.BidPrice == "" || msg.Data.AskPrice == "" {
			continue
		}
		// Binance bookTicker 已经是最优买卖一档，用它做模拟盘成交价基准。
		s.updateQuoteFromWS(quoteFromStrings("binance", msg.Data.Symbol, marketType, msg.Data.BidPrice, msg.Data.AskPrice, 0))
	}
}

func (s *cexSpotPerpState) runOKXTickerWS(generation int64) {
	name := "okx:public"
	endpoints := []string{
		"wss://ws.okx.com:8443/ws/v5/public",
		"wss://wsaws.okx.com:8443/ws/v5/public",
	}
	next := 0
	for s.isWSGeneration(generation) {
		s.setWSStatus(name, "connecting")
		endpoint := endpoints[next%len(endpoints)]
		next++
		if err := s.consumeOKXTicker(name, endpoint, generation); err != nil {
			s.setWSError(name, err)
			time.Sleep(3 * time.Second)
		}
	}
}

func (s *cexSpotPerpState) consumeOKXTicker(name, endpoint string, generation int64) error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 8 * time.Second,
	}
	conn, _, err := dialer.Dial(endpoint, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	s.setWSStatus(name, "connected")

	symbols := s.symbolsForExchange("okx")
	if len(symbols) == 0 {
		return fmt.Errorf("no enabled okx symbols")
	}
	args := make([]gin.H, 0, len(symbols)*2)
	for _, symbol := range symbols {
		args = append(args,
			gin.H{"channel": "tickers", "instId": okxSpotSymbol(symbol)},
			gin.H{"channel": "tickers", "instId": okxSwapSymbol(symbol)},
		)
	}
	if err := conn.WriteJSON(gin.H{"op": "subscribe", "args": args}); err != nil {
		return err
	}

	for {
		if !s.isWSGeneration(generation) {
			return nil
		}
		var msg struct {
			Event string `json:"event"`
			Arg   struct {
				InstID string `json:"instId"`
			} `json:"arg"`
			Data []struct {
				InstID string `json:"instId"`
				BidPx  string `json:"bidPx"`
				AskPx  string `json:"askPx"`
			} `json:"data"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		if msg.Event != "" || len(msg.Data) == 0 {
			continue
		}
		instID := msg.Data[0].InstID
		if instID == "" {
			instID = msg.Arg.InstID
		}
		marketType := strategy.MarketTypeSpot
		symbol := okxSymbolToPlain(instID)
		if strings.HasSuffix(instID, "-SWAP") {
			marketType = strategy.MarketTypePerp
		}
		if symbol == "" || msg.Data[0].BidPx == "" || msg.Data[0].AskPx == "" {
			continue
		}
		s.updateQuoteFromWS(quoteFromStrings("okx", symbol, marketType, msg.Data[0].BidPx, msg.Data[0].AskPx, 0))
	}
}

func (s *cexSpotPerpState) runBitgetTickerWS(marketType string, generation int64) {
	name := "bitget:" + marketType
	for s.isWSGeneration(generation) {
		s.setWSStatus(name, "connecting")
		if err := s.consumeBitgetTicker(name, marketType, generation); err != nil {
			s.setWSError(name, err)
			time.Sleep(3 * time.Second)
		}
	}
}

func (s *cexSpotPerpState) consumeBitgetTicker(name, marketType string, generation int64) error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 8 * time.Second,
	}
	conn, _, err := dialer.Dial("wss://ws.bitget.com/v2/ws/public", nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	s.setWSStatus(name, "connected")

	instType := "SPOT"
	if marketType == strategy.MarketTypePerp {
		instType = "USDT-FUTURES"
	}
	symbols := s.symbolsForExchange("bitget")
	if len(symbols) == 0 {
		return fmt.Errorf("no enabled bitget symbols")
	}
	args := make([]gin.H, 0, len(symbols))
	for _, symbol := range symbols {
		args = append(args, gin.H{"instType": instType, "channel": "ticker", "instId": symbol})
	}
	if err := conn.WriteJSON(gin.H{"op": "subscribe", "args": args}); err != nil {
		return err
	}

	for {
		if !s.isWSGeneration(generation) {
			return nil
		}
		var msg struct {
			Event string `json:"event"`
			Arg   struct {
				InstID string `json:"instId"`
			} `json:"arg"`
			Data json.RawMessage `json:"data"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		if msg.Event != "" || len(msg.Data) == 0 {
			continue
		}
		ticker, err := decodeBitgetTicker(msg.Data)
		if err != nil || msg.Arg.InstID == "" || ticker.BidPr == "" || ticker.AskPr == "" {
			continue
		}
		// Bitget v2 公共 WS 同一地址承载现货和 USDT 永续，用 instType 区分市场。
		s.updateQuoteFromWS(quoteFromStrings("bitget", msg.Arg.InstID, marketType, ticker.BidPr, ticker.AskPr, 0))
	}
}

func (s *cexSpotPerpState) setMarketError(exchangeName, symbol, kind string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marketErrors[exchangeName+":"+symbol+":"+kind] = err.Error()
}

func (s *cexSpotPerpState) clearMarketError(exchangeName, symbol, kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.marketErrors, exchangeName+":"+symbol+":"+kind)
	s.lastQuoteAt = time.Now().UnixMilli()
}

func (s *cexSpotPerpState) setWSStatus(name, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wsStatus[name] = status
	if status == "connected" {
		delete(s.wsErrors, name)
	}
}

func (s *cexSpotPerpState) setWSError(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wsStatus[name] = "error"
	s.wsErrors[name] = err.Error()
	s.lastQuoteAt = time.Now().UnixMilli()
}

func (s *cexSpotPerpState) updateQuoteFromWS(q strategy.CEXSpotPerpQuote) {
	if q.Bid <= 0 || q.Ask <= 0 || q.Ask < q.Bid {
		s.setMarketError(q.Exchange, q.Symbol, q.MarketType, fmt.Errorf("invalid quote bid=%f ask=%f", q.Bid, q.Ask))
		return
	}
	s.strategy.UpdateQuote(q)
	s.mu.Lock()
	s.lastQuoteAt = time.Now().UnixMilli()
	delete(s.marketErrors, q.Exchange+":"+q.Symbol+":"+q.MarketType)
	s.mu.Unlock()
}

func (s *cexSpotPerpState) updatePerpFunding(exchangeName, symbol string, fundingRate float64) {
	// CEXSpotPerpStrategy 目前只暴露完整 quote 更新；资金费率低频变化时，
	// 用最近一次盘口近似补写 funding，不改变 bid/ask 的价格来源。
	q := strategy.CEXSpotPerpQuote{
		Exchange:    exchangeName,
		Symbol:      symbol,
		MarketType:  strategy.MarketTypePerp,
		FundingRate: fundingRate,
		Timestamp:   time.Now().UnixMilli(),
	}
	s.strategy.UpdateFunding(q)
}

func (s *cexSpotPerpState) fetchSpotQuote(ctx context.Context, exchangeName, symbol string) (strategy.CEXSpotPerpQuote, error) {
	switch exchangeName {
	case "binance":
		var resp struct {
			Symbol   string `json:"symbol"`
			BidPrice string `json:"bidPrice"`
			AskPrice string `json:"askPrice"`
		}
		if err := s.getJSON(ctx, "https://api.binance.com/api/v3/ticker/bookTicker?symbol="+url.QueryEscape(symbol), &resp); err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		return quoteFromStrings(exchangeName, symbol, strategy.MarketTypeSpot, resp.BidPrice, resp.AskPrice, 0), nil
	case "okx":
		t, err := s.fetchOKXTicker(ctx, okxSpotSymbol(symbol))
		if err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		return quoteFromStrings(exchangeName, symbol, strategy.MarketTypeSpot, t.BidPx, t.AskPx, 0), nil
	case "bitget":
		var resp struct {
			Data []struct {
				BidPr string `json:"bidPr"`
				AskPr string `json:"askPr"`
			} `json:"data"`
		}
		if err := s.getBitgetJSON(ctx, "https://api.bitget.com/api/v2/spot/market/tickers?symbol="+url.QueryEscape(symbol), &resp); err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		if len(resp.Data) == 0 {
			return strategy.CEXSpotPerpQuote{}, fmt.Errorf("bitget spot ticker empty")
		}
		return quoteFromStrings(exchangeName, symbol, strategy.MarketTypeSpot, resp.Data[0].BidPr, resp.Data[0].AskPr, 0), nil
	default:
		return strategy.CEXSpotPerpQuote{}, fmt.Errorf("unsupported exchange: %s", exchangeName)
	}
}

func (s *cexSpotPerpState) fetchPerpQuote(ctx context.Context, exchangeName, symbol string) (strategy.CEXSpotPerpQuote, error) {
	switch exchangeName {
	case "binance":
		var resp struct {
			Symbol   string `json:"symbol"`
			BidPrice string `json:"bidPrice"`
			AskPrice string `json:"askPrice"`
		}
		if err := s.getJSON(ctx, "https://fapi.binance.com/fapi/v1/ticker/bookTicker?symbol="+url.QueryEscape(symbol), &resp); err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		return quoteFromStrings(exchangeName, symbol, strategy.MarketTypePerp, resp.BidPrice, resp.AskPrice, 0), nil
	case "okx":
		t, err := s.fetchOKXTicker(ctx, okxSwapSymbol(symbol))
		if err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		return quoteFromStrings(exchangeName, symbol, strategy.MarketTypePerp, t.BidPx, t.AskPx, 0), nil
	case "bitget":
		var resp struct {
			Data json.RawMessage `json:"data"`
		}
		endpoint := fmt.Sprintf("https://api.bitget.com/api/v2/mix/market/ticker?symbol=%s&productType=USDT-FUTURES", url.QueryEscape(symbol))
		if err := s.getBitgetJSON(ctx, endpoint, &resp); err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		ticker, err := decodeBitgetTicker(resp.Data)
		if err != nil {
			return strategy.CEXSpotPerpQuote{}, err
		}
		return quoteFromStrings(exchangeName, symbol, strategy.MarketTypePerp, ticker.BidPr, ticker.AskPr, 0), nil
	default:
		return strategy.CEXSpotPerpQuote{}, fmt.Errorf("unsupported exchange: %s", exchangeName)
	}
}

type bitgetTickerData struct {
	BidPr string `json:"bidPr"`
	AskPr string `json:"askPr"`
}

func decodeBitgetTicker(raw json.RawMessage) (bitgetTickerData, error) {
	var one bitgetTickerData
	if err := json.Unmarshal(raw, &one); err == nil && (one.BidPr != "" || one.AskPr != "") {
		return one, nil
	}
	var many []bitgetTickerData
	if err := json.Unmarshal(raw, &many); err != nil {
		return bitgetTickerData{}, err
	}
	if len(many) == 0 {
		return bitgetTickerData{}, fmt.Errorf("bitget ticker empty")
	}
	return many[0], nil
}

func (s *cexSpotPerpState) fetchFundingRate(ctx context.Context, exchangeName, symbol string) (float64, error) {
	switch exchangeName {
	case "binance":
		var resp struct {
			FundingRate string `json:"lastFundingRate"`
		}
		if err := s.getJSON(ctx, "https://fapi.binance.com/fapi/v1/premiumIndex?symbol="+url.QueryEscape(symbol), &resp); err != nil {
			return 0, err
		}
		return strconv.ParseFloat(resp.FundingRate, 64)
	case "okx":
		var resp struct {
			Data []struct {
				FundingRate string `json:"fundingRate"`
			} `json:"data"`
		}
		if err := s.getOKXJSON(ctx, "https://www.okx.com/api/v5/public/funding-rate?instId="+url.QueryEscape(okxSwapSymbol(symbol)), &resp); err != nil {
			return 0, err
		}
		if len(resp.Data) == 0 {
			return 0, fmt.Errorf("okx funding empty")
		}
		return strconv.ParseFloat(resp.Data[0].FundingRate, 64)
	case "bitget":
		var resp struct {
			Data []struct {
				FundingRate string `json:"fundingRate"`
			} `json:"data"`
		}
		endpoint := fmt.Sprintf("https://api.bitget.com/api/v2/mix/market/current-fund-rate?symbol=%s&productType=USDT-FUTURES", url.QueryEscape(symbol))
		if err := s.getBitgetJSON(ctx, endpoint, &resp); err != nil {
			return 0, err
		}
		if len(resp.Data) == 0 {
			return 0, fmt.Errorf("bitget funding empty")
		}
		return strconv.ParseFloat(resp.Data[0].FundingRate, 64)
	default:
		return 0, fmt.Errorf("unsupported exchange: %s", exchangeName)
	}
}

type okxTickerData struct {
	BidPx string `json:"bidPx"`
	AskPx string `json:"askPx"`
}

func (s *cexSpotPerpState) fetchOKXTicker(ctx context.Context, instID string) (okxTickerData, error) {
	var resp struct {
		Data []okxTickerData `json:"data"`
	}
	if err := s.getOKXJSON(ctx, "https://www.okx.com/api/v5/market/ticker?instId="+url.QueryEscape(instID), &resp); err != nil {
		return okxTickerData{}, err
	}
	if len(resp.Data) == 0 {
		return okxTickerData{}, fmt.Errorf("okx ticker empty")
	}
	return resp.Data[0], nil
}

func (s *cexSpotPerpState) getJSON(ctx context.Context, endpoint string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *cexSpotPerpState) getOKXJSON(ctx context.Context, endpoint string, out interface{}) error {
	var envelope struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := s.getJSON(ctx, endpoint, &envelope); err != nil {
		return err
	}
	if envelope.Code != "0" {
		return fmt.Errorf("okx code=%s msg=%s", envelope.Code, envelope.Msg)
	}
	wrapped := struct {
		Data json.RawMessage `json:"data"`
	}{Data: envelope.Data}
	b, _ := json.Marshal(wrapped)
	return json.Unmarshal(b, out)
}

func (s *cexSpotPerpState) getBitgetJSON(ctx context.Context, endpoint string, out interface{}) error {
	var envelope struct {
		Code json.RawMessage `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := s.getJSON(ctx, endpoint, &envelope); err != nil {
		return err
	}
	code := string(envelope.Code)
	if code != `"00000"` && code != `"0"` && code != `0` {
		return fmt.Errorf("bitget code=%s msg=%s", code, envelope.Msg)
	}
	wrapped := struct {
		Data json.RawMessage `json:"data"`
	}{Data: envelope.Data}
	b, _ := json.Marshal(wrapped)
	return json.Unmarshal(b, out)
}

func quoteFromStrings(exchangeName, symbol, marketType, bidText, askText string, fundingRate float64) strategy.CEXSpotPerpQuote {
	bid, _ := strconv.ParseFloat(bidText, 64)
	ask, _ := strconv.ParseFloat(askText, 64)
	return strategy.CEXSpotPerpQuote{
		Exchange:    exchangeName,
		Symbol:      symbol,
		MarketType:  marketType,
		Bid:         bid,
		Ask:         ask,
		Last:        (bid + ask) / 2,
		FundingRate: fundingRate,
		Timestamp:   time.Now().UnixMilli(),
	}
}

func okxSpotSymbol(symbol string) string {
	return symbol[:len(symbol)-4] + "-USDT"
}

func okxSwapSymbol(symbol string) string {
	return okxSpotSymbol(symbol) + "-SWAP"
}

func okxSymbolToPlain(instID string) string {
	instID = strings.TrimSuffix(instID, "-SWAP")
	return strings.ReplaceAll(instID, "-", "")
}

func (s *cexSpotPerpState) snapshot() gin.H {
	s.ensurePersistenceLoaded()

	s.mu.RLock()
	symbols := append([]string(nil), s.symbols...)
	minProfitRate := s.minProfitRate
	lastQuoteAt := s.lastQuoteAt
	automation := s.automation
	config := gin.H{
		"symbols":               append([]string(nil), s.symbols...),
		"exchanges":             append([]string(nil), s.exchanges...),
		"exchangeSymbols":       cloneStringSliceMap(s.exchangeSymbols),
		"leverage":              s.leverage,
		"maxLeverage":           3,
		"minNetProfitRate":      s.minProfitRate,
		"carryFundingIntervals": s.strategy.Config().CarryFundingIntervals,
	}
	enabledQuoteSymbols := cloneStringSliceMap(s.exchangeSymbols)
	marketErrors := make(map[string]string, len(s.marketErrors))
	for key, value := range s.marketErrors {
		marketErrors[key] = value
	}
	wsStatus := make(map[string]string, len(s.wsStatus))
	for key, value := range s.wsStatus {
		wsStatus[key] = value
	}
	wsErrors := make(map[string]string, len(s.wsErrors))
	for key, value := range s.wsErrors {
		wsErrors[key] = value
	}
	s.mu.RUnlock()

	opps := make([]gin.H, 0)
	nextOpportunities := make(map[string]*strategy.Opportunity)
	for _, symbol := range symbols {
		for _, opp := range s.strategy.ScanSymbolCandidates(symbol) {
			nextOpportunities[opp.ID] = opp
			status, blockReason := opportunityStatus(opp, minProfitRate)
			s.recordOpportunityLog(opp, status, blockReason)
			opps = append(opps, opportunityDTO(opp, minProfitRate))
		}
	}
	s.mu.Lock()
	s.opportunities = nextOpportunities
	s.mu.Unlock()
	if automation.Enabled {
		s.runAutomation(nextOpportunities, minProfitRate)
	}

	halted, reason := s.simulator.IsHalted()
	status := "running"
	if halted {
		status = "halted"
	}

	return gin.H{
		"status":          status,
		"haltReason":      reason,
		"config":          config,
		"accounts":        s.accountDTOs(),
		"quotes":          quoteDTOs(s.strategy.QuoteStatuses(), enabledQuoteSymbols),
		"opportunities":   opps,
		"positions":       positionDTOs(s.simulator.Positions()),
		"closeActions":    closeActionDTOs(s.simulator.CloseActions()),
		"opportunityLogs": s.opportunityLogDTOs(),
		"autoTrades":      s.autoTradeDTOs(),
		"automation":      s.automationDTO(),
		"autoStats":       s.autoStatsDTO(),
		"pnl":             s.simulator.PnLSummary(),
		"lastQuoteAt":     lastQuoteAt,
		"marketErrors":    marketErrors,
		"wsStatus":        wsStatus,
		"wsErrors":        wsErrors,
	}
}

func quoteDTOs(quotes []strategy.CEXSpotPerpQuoteStatus, enabledSymbols map[string][]string) []gin.H {
	result := make([]gin.H, 0, len(quotes))
	for _, quote := range quotes {
		if !quoteEnabled(quote.Exchange, quote.Symbol, enabledSymbols) {
			continue
		}
		result = append(result, gin.H{
			"exchange":    quote.Exchange,
			"symbol":      quote.Symbol,
			"marketType":  quote.MarketType,
			"bid":         quote.Bid,
			"ask":         quote.Ask,
			"last":        quote.Last,
			"fundingRate": quote.FundingRate,
			"timestamp":   quote.Timestamp,
			"ageMillis":   quote.AgeMillis,
			"stale":       quote.Stale,
		})
	}
	return result
}

func quoteEnabled(exchangeName, symbol string, enabledSymbols map[string][]string) bool {
	symbols, ok := enabledSymbols[exchangeName]
	if !ok {
		return false
	}
	for _, item := range symbols {
		if item == symbol {
			return true
		}
	}
	return false
}

func (s *cexSpotPerpState) recordOpportunityLog(opp *strategy.Opportunity, status, blockReason string) {
	if opp == nil {
		return
	}
	// 机会记录只保存扣除手续费、滑点和安全缓冲后的正净收益机会。
	// 亏损或零收益的候选仍会出现在实时扫描表里用于观察，但不进入历史统计，
	// 否则行情每秒刷新会把“没有套利空间”的组合刷成大量无效记录。
	if opp.NetProfit <= 0 {
		return
	}
	now := time.Now().UnixMilli()
	key := opportunityStableKey(opp)
	var snapshot cexSpotPerpOpportunityLog
	s.mu.Lock()
	log := s.opportunityLogs[key]
	if log == nil {
		log = &cexSpotPerpOpportunityLog{
			Key:            key,
			ID:             opp.ID,
			Symbol:         opp.Symbol,
			Direction:      opp.Direction,
			SpotExchange:   opp.SpotExchange,
			PerpExchange:   opp.PerpExchange,
			FirstSeenAt:    now,
			BestProfit:     opp.NetProfit,
			BestProfitRate: opp.ProfitRate,
		}
		s.opportunityLogs[key] = log
	}
	log.ID = opp.ID
	log.LastSeenAt = now
	log.SeenCount++
	log.LastProfit = opp.NetProfit
	log.LastProfitRate = opp.ProfitRate
	log.LastStatus = status
	log.LastBlockReason = blockReason
	if opp.NetProfit > log.BestProfit {
		log.BestProfit = opp.NetProfit
		log.BestProfitRate = opp.ProfitRate
	}
	snapshot = *log
	s.mu.Unlock()
	persistCEXSpotPerpOpportunityLog(snapshot)
}

func (s *cexSpotPerpState) opportunityLogDTOs() []gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()
	logs := make([]*cexSpotPerpOpportunityLog, 0, len(s.opportunityLogs))
	for _, log := range s.opportunityLogs {
		if log.BestProfit <= 0 {
			continue
		}
		logs = append(logs, log)
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].LastSeenAt > logs[j].LastSeenAt
	})
	if len(logs) > 200 {
		logs = logs[:200]
	}
	result := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		result = append(result, gin.H{
			"key":              log.Key,
			"id":               log.ID,
			"symbol":           log.Symbol,
			"direction":        log.Direction,
			"spotExchange":     log.SpotExchange,
			"perpExchange":     log.PerpExchange,
			"firstSeenAt":      log.FirstSeenAt,
			"lastSeenAt":       log.LastSeenAt,
			"seenCount":        log.SeenCount,
			"bestProfit":       log.BestProfit,
			"bestProfitRate":   log.BestProfitRate,
			"lastProfit":       log.LastProfit,
			"lastProfitRate":   log.LastProfitRate,
			"lastStatus":       log.LastStatus,
			"lastBlockReason":  log.LastBlockReason,
			"autoOpenedCount":  log.AutoOpenedCount,
			"autoRejectedNote": log.AutoRejectedNote,
		})
	}
	return result
}

func (s *cexSpotPerpState) automationDTO() gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return gin.H{
		"enabled":             s.automation.Enabled,
		"autoOpen":            s.automation.AutoOpen,
		"autoClose":           s.automation.AutoClose,
		"openMinProfitRate":   s.automation.OpenMinProfitRate,
		"closeMinProfitRate":  s.automation.CloseMinProfitRate,
		"maxHoldSeconds":      s.automation.MaxHoldSeconds,
		"maxOpenPositions":    s.automation.MaxOpenPositions,
		"checkIntervalMillis": s.automation.CheckIntervalMillis,
	}
}

func (s *cexSpotPerpState) autoStatsDTO() gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := s.autoStats
	return gin.H{
		"autoOpenCount":   stats.AutoOpenCount,
		"autoCloseCount":  stats.AutoCloseCount,
		"winCount":        stats.WinCount,
		"lossCount":       stats.LossCount,
		"totalProfit":     stats.TotalProfit,
		"averageProfit":   stats.AverageProfit,
		"winRate":         stats.WinRate,
		"lastActionAt":    stats.LastActionAt,
		"lastActionError": stats.LastActionError,
	}
}

func (s *cexSpotPerpState) autoTradeDTOs() []gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()
	start := 0
	if len(s.autoTrades) > 200 {
		start = len(s.autoTrades) - 200
	}
	result := make([]gin.H, 0, len(s.autoTrades)-start)
	for i := len(s.autoTrades) - 1; i >= start; i-- {
		trade := s.autoTrades[i]
		result = append(result, gin.H{
			"id":           trade.ID,
			"positionId":   trade.PositionID,
			"opportunity":  trade.Opportunity,
			"symbol":       trade.Symbol,
			"direction":    trade.Direction,
			"spotExchange": trade.SpotExchange,
			"perpExchange": trade.PerpExchange,
			"action":       trade.Action,
			"reason":       trade.Reason,
			"quantity":     trade.Quantity,
			"notional":     trade.Notional,
			"margin":       trade.Margin,
			"spotValue":    trade.SpotValue,
			"capitalUsed":  trade.CapitalUsed,
			"profit":       trade.Profit,
			"profitRate":   trade.ProfitRate,
			"createdAt":    trade.CreatedAt,
		})
	}
	return result
}

func (s *cexSpotPerpState) configDTO() gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return gin.H{
		"symbols":               append([]string(nil), s.symbols...),
		"exchanges":             append([]string(nil), s.exchanges...),
		"exchangeSymbols":       cloneStringSliceMap(s.exchangeSymbols),
		"leverage":              s.leverage,
		"maxLeverage":           3,
		"minNetProfitRate":      s.minProfitRate,
		"carryFundingIntervals": s.strategy.Config().CarryFundingIntervals,
	}
}

func (s *cexSpotPerpState) accountDTOs() []gin.H {
	result := make([]gin.H, 0, 3)
	for _, exchange := range []string{"binance", "okx", "bitget"} {
		account, ok := s.simulator.Account(exchange)
		if !ok {
			continue
		}
		result = append(result, gin.H{
			"exchange":      account.Exchange,
			"usdt":          account.USDT,
			"perpUsdt":      account.PerpUSDT,
			"frozenUsdt":    account.FrozenUSDT,
			"spotBalances":  account.SpotBalances,
			"perpPositions": account.PerpPositions,
		})
	}
	return result
}

func opportunityStatus(opp *strategy.Opportunity, minProfitRate float64) (string, string) {
	status := "ready"
	blockReason := ""
	if opp.NetProfit <= 0 {
		if opp.CarryNetProfit > 0 {
			status = "watch"
			blockReason = fmt.Sprintf("当前净收益小于等于 0，但 %g 期资金费率后预期为正", opp.CarryFundingIntervals)
		} else {
			status = "blocked"
			blockReason = "净收益小于等于 0，持仓资金费率后仍不划算"
		}
	} else if opp.ProfitRate < minProfitRate {
		if opp.CarryNetProfit > 0 {
			status = "watch"
			blockReason = fmt.Sprintf("当前收益率 %.4f%% 低于阈值 %.4f%%，但持仓预期为正", opp.ProfitRate, minProfitRate)
		} else {
			status = "blocked"
			blockReason = fmt.Sprintf("收益率 %.4f%% 低于阈值 %.4f%%", opp.ProfitRate, minProfitRate)
		}
	}
	return status, blockReason
}

func opportunityDTO(opp *strategy.Opportunity, minProfitRate float64) gin.H {
	status, blockReason := opportunityStatus(opp, minProfitRate)
	return gin.H{
		"id":                    opp.ID,
		"symbol":                opp.Symbol,
		"direction":             opp.Direction,
		"spotExchange":          opp.SpotExchange,
		"perpExchange":          opp.PerpExchange,
		"spotPrice":             opp.PriceA,
		"perpPrice":             opp.PriceB,
		"notional":              opp.Notional,
		"basisAmount":           opp.BasisAmount,
		"fundingAmount":         opp.FundingAmount,
		"feeCost":               opp.FeeCost,
		"slippage":              opp.Slippage,
		"safetyBuffer":          opp.SafetyBuffer,
		"netProfit":             opp.NetProfit,
		"profitRate":            opp.ProfitRate,
		"carryFundingAmount":    opp.CarryFundingAmount,
		"carryNetProfit":        opp.CarryNetProfit,
		"carryProfitRate":       opp.CarryProfitRate,
		"carryFundingIntervals": opp.CarryFundingIntervals,
		"status":                status,
		"blockReason":           blockReason,
	}
}

func opportunityStableKey(opp *strategy.Opportunity) string {
	if opp == nil {
		return ""
	}
	return strings.Join([]string{opp.Symbol, opp.Direction, opp.SpotExchange, opp.PerpExchange}, ":")
}

func opportunityHedgeKey(opp *strategy.Opportunity) string {
	if opp == nil {
		return ""
	}
	return strings.Join([]string{opp.Symbol, opp.SpotExchange, opp.PerpExchange}, ":")
}

func persistenceIDPart(value string) string {
	value = strings.ReplaceAll(value, ":", "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func (s *cexSpotPerpState) runAutomation(opps map[string]*strategy.Opportunity, minProfitRate float64) {
	now := time.Now().UnixMilli()
	s.mu.Lock()
	cfg := s.automation
	if cfg.CheckIntervalMillis <= 0 {
		cfg.CheckIntervalMillis = 1000
	}
	if s.lastAutoCheckAt > 0 && now-s.lastAutoCheckAt < cfg.CheckIntervalMillis {
		s.mu.Unlock()
		return
	}
	s.lastAutoCheckAt = now
	s.mu.Unlock()

	// 自动逻辑先尝试平仓释放保证金，再开新仓，避免仓位上限被旧仓位卡住。
	if cfg.AutoClose {
		s.autoClosePositions(cfg, now)
	}
	if cfg.AutoOpen {
		s.autoOpenOpportunities(opps, cfg, minProfitRate, now)
	}
}

func (s *cexSpotPerpState) autoOpenOpportunities(opps map[string]*strategy.Opportunity, cfg cexSpotPerpAutomationConfig, minProfitRate float64, now int64) {
	maxOpen := cfg.MaxOpenPositions
	if maxOpen <= 0 {
		maxOpen = 1
	}
	openKeys := make(map[string]bool)
	openHedgeKeys := make(map[string]bool)
	openCount := 0
	for _, pos := range s.simulator.Positions() {
		if pos == nil || pos.Status != "open" || pos.Opportunity == nil {
			continue
		}
		openCount++
		openKeys[opportunityStableKey(pos.Opportunity)] = true
		openHedgeKeys[opportunityHedgeKey(pos.Opportunity)] = true
	}
	if openCount >= maxOpen {
		return
	}

	candidates := make([]*strategy.Opportunity, 0, len(opps))
	for _, opp := range opps {
		status, _ := opportunityStatus(opp, minProfitRate)
		if status != "ready" || opp.ProfitRate < cfg.OpenMinProfitRate || openKeys[opportunityStableKey(opp)] || openHedgeKeys[opportunityHedgeKey(opp)] {
			continue
		}
		candidates = append(candidates, opp)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].NetProfit > candidates[j].NetProfit
	})

	for _, opp := range candidates {
		if openCount >= maxOpen {
			return
		}
		pos, err := s.simulator.ExecuteOpportunity(opp)
		if err != nil {
			s.recordAutoError(err)
			s.markOpportunityRejected(opp, err.Error())
			continue
		}
		openCount++
		s.persistSimState()
		s.recordAutoOpen(opp, pos, now)
	}
}

func (s *cexSpotPerpState) autoClosePositions(cfg cexSpotPerpAutomationConfig, now int64) {
	for _, pos := range s.simulator.Positions() {
		if pos == nil || pos.Status != "open" || pos.Opportunity == nil {
			continue
		}
		reason := ""
		holdMillis := now - pos.OpenedAt
		if cfg.MaxHoldSeconds > 0 && holdMillis >= cfg.MaxHoldSeconds*1000 {
			reason = fmt.Sprintf("自动平仓：持仓超过 %d 秒", cfg.MaxHoldSeconds)
		}
		if reason == "" && cfg.CloseMinProfitRate > 0 {
			_, currentProfitRate, ok := s.estimatePositionClosePnL(pos)
			if ok && currentProfitRate >= cfg.CloseMinProfitRate {
				reason = fmt.Sprintf("自动平仓：当前预计平仓收益率 %.4f%% 达到 %.4f%%", currentProfitRate, cfg.CloseMinProfitRate)
			}
		}
		if reason == "" {
			continue
		}
		closedPos, err := s.closePositionWithCurrentMarket(pos.ID, reason)
		if err != nil {
			s.recordAutoError(err)
			continue
		}
		s.persistSimState()
		s.recordAutoClose(closedPos, reason, now)
	}
}

func (s *cexSpotPerpState) estimatePositionClosePnL(pos *strategy.SimArbitragePosition) (float64, float64, bool) {
	if pos == nil || pos.Opportunity == nil {
		return 0, 0, false
	}
	closeSpotPrice, closePerpPrice, ok := s.strategy.ClosePrices(pos.Opportunity.Symbol, pos.Opportunity.SpotExchange, pos.Opportunity.PerpExchange, pos.Opportunity.Direction)
	if !ok {
		return 0, 0, false
	}
	pnl, rate, err := s.simulator.EstimateClosePnL(pos.ID, closeSpotPrice, closePerpPrice)
	if err != nil {
		return 0, 0, false
	}
	return pnl, rate, true
}

func (s *cexSpotPerpState) closePositionWithCurrentMarket(positionID, reason string) (*strategy.SimArbitragePosition, error) {
	pos, ok := s.simulator.Position(positionID)
	if !ok || pos == nil || pos.Opportunity == nil {
		return nil, strategy.ErrSimPositionNotFound
	}
	closeSpotPrice, closePerpPrice, ok := s.strategy.ClosePrices(pos.Opportunity.Symbol, pos.Opportunity.SpotExchange, pos.Opportunity.PerpExchange, pos.Opportunity.Direction)
	if ok {
		if _, err := s.simulator.ClosePositionWithMarket(positionID, reason, closeSpotPrice, closePerpPrice); err != nil {
			return nil, err
		}
	} else {
		if _, err := s.simulator.ClosePosition(positionID, reason); err != nil {
			return nil, err
		}
	}
	closedPos, ok := s.simulator.Position(positionID)
	if !ok || closedPos == nil {
		return nil, strategy.ErrSimPositionNotFound
	}
	return closedPos, nil
}

func (s *cexSpotPerpState) recordAutoOpen(opp *strategy.Opportunity, pos *strategy.SimArbitragePosition, now int64) {
	positionID := ""
	if pos != nil {
		positionID = pos.ID
	}
	key := opportunityStableKey(opp)
	quantity, notional, margin, spotValue, capitalUsed := autoTradeCapitalFields(pos, opp)
	trade := cexSpotPerpAutoTrade{
		ID:           fmt.Sprintf("auto-open-%d-%s", now, persistenceIDPart(key)),
		PositionID:   positionID,
		Opportunity:  opp.ID,
		Symbol:       opp.Symbol,
		Direction:    opp.Direction,
		SpotExchange: opp.SpotExchange,
		PerpExchange: opp.PerpExchange,
		Action:       "open",
		Reason:       "自动开仓：机会达到开仓阈值",
		Quantity:     quantity,
		Notional:     notional,
		Margin:       margin,
		SpotValue:    spotValue,
		CapitalUsed:  capitalUsed,
		Profit:       opp.NetProfit,
		ProfitRate:   opp.ProfitRate,
		CreatedAt:    now,
	}
	var logSnapshot *cexSpotPerpOpportunityLog
	s.mu.Lock()
	s.autoTrades = append(s.autoTrades, trade)
	s.applyAutoTradeStatsLocked(trade)
	s.autoStats.LastActionAt = now
	s.autoStats.LastActionError = ""
	if log := s.opportunityLogs[key]; log != nil {
		log.AutoOpenedCount++
		log.AutoRejectedNote = ""
		copied := *log
		logSnapshot = &copied
	}
	s.mu.Unlock()
	persistCEXSpotPerpAutoTrade(trade)
	if logSnapshot != nil {
		persistCEXSpotPerpOpportunityLog(*logSnapshot)
	}
}

func (s *cexSpotPerpState) recordAutoClose(pos *strategy.SimArbitragePosition, reason string, now int64) {
	opp := pos.Opportunity
	profit := pos.RealizedPnL
	profitRate := 0.0
	if pos.Notional > 0 {
		profitRate = profit / pos.Notional * 100
	}
	quantity, notional, margin, spotValue, capitalUsed := autoTradeCapitalFields(pos, opp)
	trade := cexSpotPerpAutoTrade{
		ID:           fmt.Sprintf("auto-close-%d-%s", now, persistenceIDPart(pos.ID)),
		PositionID:   pos.ID,
		Opportunity:  opp.ID,
		Symbol:       opp.Symbol,
		Direction:    opp.Direction,
		SpotExchange: opp.SpotExchange,
		PerpExchange: opp.PerpExchange,
		Action:       "close",
		Reason:       reason,
		Quantity:     quantity,
		Notional:     notional,
		Margin:       margin,
		SpotValue:    spotValue,
		CapitalUsed:  capitalUsed,
		Profit:       profit,
		ProfitRate:   profitRate,
		CreatedAt:    now,
	}
	s.mu.Lock()
	s.autoTrades = append(s.autoTrades, trade)
	s.applyAutoTradeStatsLocked(trade)
	s.autoStats.LastActionAt = now
	s.autoStats.LastActionError = ""
	s.mu.Unlock()
	persistCEXSpotPerpAutoTrade(trade)
}

func autoTradeCapitalFields(pos *strategy.SimArbitragePosition, opp *strategy.Opportunity) (float64, float64, float64, float64, float64) {
	if opp == nil {
		return 0, 0, 0, 0, 0
	}
	quantity := spotPerpQuantityFromOpportunity(opp)
	notional := opp.Notional
	margin := 0.0
	if pos != nil {
		if pos.Notional > 0 {
			notional = pos.Notional
		}
		margin = pos.Margin
	}
	spotValue := quantity * opp.PriceA
	if spotValue <= 0 {
		spotValue = notional
	}
	// 这里记录的是资金足迹，不是单纯现金流。
	// 正向策略是现货买入价值 + 合约保证金；反向策略是库存占用价值 + 合约保证金。
	capitalUsed := spotValue + margin
	return quantity, notional, margin, spotValue, capitalUsed
}

func (s *cexSpotPerpState) applyAutoTradeStatsLocked(trade cexSpotPerpAutoTrade) {
	if trade.CreatedAt > s.autoStats.LastActionAt {
		s.autoStats.LastActionAt = trade.CreatedAt
	}
	if trade.Action == "open" {
		s.autoStats.AutoOpenCount++
		return
	}
	if trade.Action != "close" {
		return
	}
	s.autoStats.AutoCloseCount++
	if trade.Profit >= 0 {
		s.autoStats.WinCount++
	} else {
		s.autoStats.LossCount++
	}
	s.autoStats.TotalProfit += trade.Profit
	if s.autoStats.AutoCloseCount > 0 {
		s.autoStats.AverageProfit = s.autoStats.TotalProfit / float64(s.autoStats.AutoCloseCount)
		s.autoStats.WinRate = float64(s.autoStats.WinCount) / float64(s.autoStats.AutoCloseCount) * 100
	}
}

func (s *cexSpotPerpState) recordAutoError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoStats.LastActionAt = time.Now().UnixMilli()
	s.autoStats.LastActionError = err.Error()
}

func (s *cexSpotPerpState) markOpportunityRejected(opp *strategy.Opportunity, note string) {
	key := opportunityStableKey(opp)
	var snapshot *cexSpotPerpOpportunityLog
	s.mu.Lock()
	if log := s.opportunityLogs[key]; log != nil {
		log.AutoRejectedNote = note
		copied := *log
		snapshot = &copied
	}
	s.mu.Unlock()
	if snapshot != nil {
		persistCEXSpotPerpOpportunityLog(*snapshot)
	}
}

func positionDTOs(positions []*strategy.SimArbitragePosition) []gin.H {
	result := make([]gin.H, 0, len(positions))
	for _, pos := range positions {
		if pos == nil || pos.Opportunity == nil {
			continue
		}
		opp := pos.Opportunity
		result = append(result, gin.H{
			"id":            pos.ID,
			"opportunityId": opp.ID,
			"symbol":        opp.Symbol,
			"direction":     opp.Direction,
			"spotExchange":  opp.SpotExchange,
			"perpExchange":  opp.PerpExchange,
			"quantity":      spotPerpQuantityFromOpportunity(opp),
			"notional":      pos.Notional,
			"margin":        pos.Margin,
			"spotPrice":     opp.PriceA,
			"perpPrice":     opp.PriceB,
			"netProfit":     opp.NetProfit,
			"realizedPnL":   pos.RealizedPnL,
			"openedAt":      pos.OpenedAt,
			"closedAt":      pos.ClosedAt,
			"status":        pos.Status,
		})
	}
	return result
}

func closeActionDTOs(actions []strategy.SimCloseAction) []gin.H {
	result := make([]gin.H, 0, len(actions))
	for i, action := range actions {
		spotAction := ""
		perpAction := ""
		if len(action.Legs) >= 2 {
			spotAction = action.Legs[0].Side
			perpAction = action.Legs[1].Side
		}
		result = append(result, gin.H{
			"id":         fmt.Sprintf("%s-%d", action.PositionID, i),
			"positionId": action.PositionID,
			"reason":     action.Reason,
			"spotAction": spotAction,
			"perpAction": perpAction,
			"createdAt":  action.CreatedAt,
		})
	}
	return result
}

func spotPerpQuantityFromOpportunity(opp *strategy.Opportunity) float64 {
	if opp == nil || len(opp.Legs) == 0 {
		return 0
	}
	return opp.Legs[0].Quantity
}

type cexSpotPerpConfigRequest struct {
	Symbols         []string            `json:"symbols"`
	Exchanges       []string            `json:"exchanges"`
	ExchangeSymbols map[string][]string `json:"exchangeSymbols"`
	Leverage        float64             `json:"leverage"`
}

type cexSpotPerpAccountRequest struct {
	USDT         float64            `json:"usdt"`
	PerpUSDT     float64            `json:"perpUsdt"`
	SpotBalances map[string]float64 `json:"spotBalances"`
}

type cexSpotPerpTransferRequest struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}

func GetCEXSpotPerpSimulation(c *gin.Context) {
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func UpdateCEXSpotPerpAutomation(c *gin.Context) {
	var req cexSpotPerpAutomationConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OpenMinProfitRate < 0 || req.CloseMinProfitRate < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profit rate thresholds cannot be negative"})
		return
	}
	if req.MaxOpenPositions <= 0 {
		req.MaxOpenPositions = 1
	}
	if req.MaxOpenPositions > 20 {
		req.MaxOpenPositions = 20
	}
	if req.CheckIntervalMillis < 500 {
		req.CheckIntervalMillis = 500
	}
	if req.MaxHoldSeconds < 0 {
		req.MaxHoldSeconds = 0
	}
	cexSpotPerpSim.mu.Lock()
	cexSpotPerpSim.automation = req
	cexSpotPerpSim.lastAutoCheckAt = 0
	cexSpotPerpSim.autoStats.LastActionError = ""
	cexSpotPerpSim.mu.Unlock()
	cexSpotPerpSim.persistAutomationSetting()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func UpdateCEXSpotPerpConfig(c *gin.Context) {
	var req cexSpotPerpConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	symbols := normalizeSymbols(req.Symbols)
	exchanges := normalizeExchanges(req.Exchanges)
	if len(symbols) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols cannot be empty"})
		return
	}
	if len(exchanges) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exchanges cannot be empty"})
		return
	}
	if req.Leverage < 1 || req.Leverage > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "leverage must be between 1 and 3"})
		return
	}

	exchangeSymbols := normalizeExchangeSymbols(req.ExchangeSymbols, exchanges, symbols)
	cexSpotPerpSim.applyConfig(symbols, exchanges, exchangeSymbols, req.Leverage)
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func (s *cexSpotPerpState) applyConfig(symbols, exchanges []string, exchangeSymbols map[string][]string, leverage float64) {
	cfg := s.strategy.Config()
	cfg.Symbols = append([]string(nil), symbols...)
	cfg.Exchanges = append([]string(nil), exchanges...)
	cfg.ExchangeSymbols = cloneStringSliceMap(exchangeSymbols)
	cfg.DefaultLeverage = leverage
	s.strategy.UpdateConfig(cfg)
	s.simulator.SetLeverage(leverage)

	s.mu.Lock()
	s.symbols = append([]string(nil), symbols...)
	s.exchanges = append([]string(nil), exchanges...)
	s.exchangeSymbols = cloneStringSliceMap(exchangeSymbols)
	s.leverage = leverage
	s.opportunities = make(map[string]*strategy.Opportunity)
	s.marketErrors = make(map[string]string)
	s.wsStatus = make(map[string]string)
	s.wsErrors = make(map[string]string)
	s.wsGeneration++
	generation := s.wsGeneration
	s.mu.Unlock()

	// 配置变更后重新订阅官方 WS。旧连接读循环会检测 generation 并自然退出，
	// 这样新增币种或关闭交易所能尽快体现在行情源和机会扫描里。
	s.launchOfficialMarketWebSockets(generation)
}

func UpdateCEXSpotPerpAccount(c *gin.Context) {
	exchangeName := strings.ToLower(strings.TrimSpace(c.Param("exchange")))
	var req cexSpotPerpAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SpotBalances == nil {
		req.SpotBalances = map[string]float64{}
	}
	if err := cexSpotPerpSim.simulator.UpdateAccount(exchangeName, req.USDT, req.PerpUSDT, req.SpotBalances); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cexSpotPerpSim.persistSimState()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func TransferCEXSpotPerpAccount(c *gin.Context) {
	exchangeName := strings.ToLower(strings.TrimSpace(c.Param("exchange")))
	var req cexSpotPerpTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := cexSpotPerpSim.simulator.TransferUSDT(exchangeName, req.From, req.To, req.Amount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cexSpotPerpSim.persistSimState()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func ResetCEXSpotPerpAccounts(c *gin.Context) {
	cexSpotPerpSim.simulator.ResetAccounts(defaultCEXSpotPerpAccounts())
	cexSpotPerpSim.persistSimState()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func normalizeSymbols(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		symbol := strings.ToUpper(strings.TrimSpace(value))
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		result = append(result, symbol)
	}
	return result
}

func normalizeExchanges(values []string) []string {
	allowed := map[string]bool{"binance": true, "okx": true, "bitget": true}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		exchangeName := strings.ToLower(strings.TrimSpace(value))
		if !allowed[exchangeName] || seen[exchangeName] {
			continue
		}
		seen[exchangeName] = true
		result = append(result, exchangeName)
	}
	return result
}

func normalizeExchangeSymbols(input map[string][]string, exchanges, symbols []string) map[string][]string {
	symbolSet := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		symbolSet[symbol] = true
	}
	result := make(map[string][]string, len(exchanges))
	for _, exchangeName := range exchanges {
		candidates := symbols
		if configured, ok := input[exchangeName]; ok {
			candidates = normalizeSymbols(configured)
		}
		for _, symbol := range candidates {
			if symbolSet[symbol] {
				result[exchangeName] = append(result[exchangeName], symbol)
			}
		}
		if len(result[exchangeName]) == 0 {
			result[exchangeName] = append([]string(nil), symbols...)
		}
	}
	return result
}

func cloneStringSliceMap(input map[string][]string) map[string][]string {
	result := make(map[string][]string, len(input))
	for key, value := range input {
		result[key] = append([]string(nil), value...)
	}
	return result
}

func ExecuteCEXSpotPerpOpportunity(c *gin.Context) {
	id := c.Param("id")
	cexSpotPerpSim.mu.RLock()
	opp := cexSpotPerpSim.opportunities[id]
	cexSpotPerpSim.mu.RUnlock()
	if opp == nil {
		cexSpotPerpSim.snapshot()
		cexSpotPerpSim.mu.RLock()
		opp = cexSpotPerpSim.opportunities[id]
		cexSpotPerpSim.mu.RUnlock()
	}
	if opp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "opportunity not found"})
		return
	}
	if opp.ProfitRate < cexSpotPerpSim.minProfitRate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "opportunity is only in observation state"})
		return
	}
	if _, err := cexSpotPerpSim.simulator.ExecuteOpportunity(opp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cexSpotPerpSim.persistSimState()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func CloseCEXSpotPerpPosition(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Reason == "" {
		req.Reason = "手动平仓"
	}
	if _, err := cexSpotPerpSim.closePositionWithCurrentMarket(id, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cexSpotPerpSim.persistSimState()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func HaltCEXSpotPerpSimulation(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Reason == "" {
		req.Reason = "手动熔断"
	}
	cexSpotPerpSim.simulator.TriggerCircuitBreaker(req.Reason)
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func ResumeCEXSpotPerpSimulation(c *gin.Context) {
	cexSpotPerpSim.simulator.Resume()
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func CEXSpotPerpWebSocket(c *gin.Context) {
	conn, err := cexSpotPerpWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 客户端不需要发送业务消息，但读泵能及时感知浏览器断开，避免写协程悬挂。
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	if err := writeCEXSpotPerpSnapshot(conn); err != nil {
		return
	}
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := writeCEXSpotPerpSnapshot(conn); err != nil {
				return
			}
		}
	}
}

func writeCEXSpotPerpSnapshot(conn *websocket.Conn) error {
	return conn.WriteJSON(gin.H{
		"type": "cex_spot_perp_snapshot",
		"data": cexSpotPerpSim.snapshot(),
	})
}
