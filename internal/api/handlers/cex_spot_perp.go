package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
)

var cexSpotPerpWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type cexSpotPerpState struct {
	mu              sync.RWMutex
	strategy        *strategy.CEXSpotPerpStrategy
	simulator       *strategy.CEXSpotPerpSimulator
	opportunities   map[string]*strategy.Opportunity
	symbols         []string
	exchanges       []string
	exchangeSymbols map[string][]string
	minProfitRate   float64
	leverage        float64
	wsGeneration    int64
	lastQuoteAt     int64
	marketErrors    map[string]string
	wsStatus        map[string]string
	wsErrors        map[string]string
	httpClient      *http.Client
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
		symbols:         cfg.Symbols,
		exchanges:       cfg.Exchanges,
		exchangeSymbols: cloneStringSliceMap(cfg.ExchangeSymbols),
		minProfitRate:   cfg.MinNetProfitRate,
		leverage:        cfg.DefaultLeverage,
		lastQuoteAt:     time.Now().UnixMilli(),
		marketErrors:    make(map[string]string),
		wsStatus:        make(map[string]string),
		wsErrors:        make(map[string]string),
		httpClient:      &http.Client{Timeout: 5 * time.Second},
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
	s.mu.RLock()
	symbols := append([]string(nil), s.symbols...)
	minProfitRate := s.minProfitRate
	lastQuoteAt := s.lastQuoteAt
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
			opps = append(opps, opportunityDTO(opp, minProfitRate))
		}
	}
	s.mu.Lock()
	s.opportunities = nextOpportunities
	s.mu.Unlock()

	halted, reason := s.simulator.IsHalted()
	status := "running"
	if halted {
		status = "halted"
	}

	return gin.H{
		"status":        status,
		"haltReason":    reason,
		"config":        config,
		"accounts":      s.accountDTOs(),
		"quotes":        quoteDTOs(s.strategy.QuoteStatuses(), enabledQuoteSymbols),
		"opportunities": opps,
		"positions":     positionDTOs(s.simulator.Positions()),
		"closeActions":  closeActionDTOs(s.simulator.CloseActions()),
		"pnl":           s.simulator.PnLSummary(),
		"lastQuoteAt":   lastQuoteAt,
		"marketErrors":  marketErrors,
		"wsStatus":      wsStatus,
		"wsErrors":      wsErrors,
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

func opportunityDTO(opp *strategy.Opportunity, minProfitRate float64) gin.H {
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
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
}

func ResetCEXSpotPerpAccounts(c *gin.Context) {
	cexSpotPerpSim.simulator.ResetAccounts(defaultCEXSpotPerpAccounts())
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
	if _, err := cexSpotPerpSim.simulator.ClosePosition(id, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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
