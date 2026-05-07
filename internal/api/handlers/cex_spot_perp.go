package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	mu            sync.RWMutex
	strategy      *strategy.CEXSpotPerpStrategy
	simulator     *strategy.CEXSpotPerpSimulator
	opportunities map[string]*strategy.Opportunity
	symbols       []string
	lastQuoteAt   int64
	marketErrors  map[string]string
	wsStatus      map[string]string
	wsErrors      map[string]string
	httpClient    *http.Client
}

var cexSpotPerpSim = newCEXSpotPerpState()

func newCEXSpotPerpState() *cexSpotPerpState {
	cfg := strategy.DefaultCEXSpotPerpConfig()
	cfg.NotionalUSDT = 3000
	cfg.MinNetProfitRate = 0.05
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

	sim := strategy.NewCEXSpotPerpSimulator(map[string]*strategy.SimAccount{
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
	}, cfg.DefaultLeverage)

	state := &cexSpotPerpState{
		strategy:      s,
		simulator:     sim,
		opportunities: make(map[string]*strategy.Opportunity),
		symbols:       cfg.Symbols,
		marketErrors:  make(map[string]string),
		wsStatus:      make(map[string]string),
		wsErrors:      make(map[string]string),
		httpClient:    &http.Client{Timeout: 5 * time.Second},
	}
	state.startOfficialMarketWebSockets()
	state.startFundingPolling()
	return state
}

func (s *cexSpotPerpState) startOfficialMarketWebSockets() {
	go s.runBinanceBookTickerWS(strategy.MarketTypeSpot)
	go s.runBinanceBookTickerWS(strategy.MarketTypePerp)
	go s.runOKXTickerWS()
	go s.runBitgetTickerWS(strategy.MarketTypeSpot)
	go s.runBitgetTickerWS(strategy.MarketTypePerp)
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
	for _, symbol := range s.symbols {
		for _, exchangeName := range []string{"binance", "okx", "bitget"} {
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
}

func (s *cexSpotPerpState) updateQuoteFromWS(q strategy.CEXSpotPerpQuote) {
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

func (s *cexSpotPerpState) snapshot() gin.H {
	s.mu.Lock()
	defer s.mu.Unlock()

	opps := make([]gin.H, 0)
	s.opportunities = make(map[string]*strategy.Opportunity)
	for _, symbol := range []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"} {
		for _, opp := range s.strategy.ScanSymbol(symbol) {
			s.opportunities[opp.ID] = opp
			opps = append(opps, opportunityDTO(opp))
		}
	}

	halted, reason := s.simulator.IsHalted()
	status := "running"
	if halted {
		status = "halted"
	}
	marketErrors := make(map[string]string, len(s.marketErrors))
	for key, value := range s.marketErrors {
		marketErrors[key] = value
	}

	return gin.H{
		"status":        status,
		"haltReason":    reason,
		"accounts":      s.accountDTOs(),
		"opportunities": opps,
		"positions":     positionDTOs(s.simulator.Positions()),
		"closeActions":  closeActionDTOs(s.simulator.CloseActions()),
		"pnl":           s.simulator.PnLSummary(),
		"lastQuoteAt":   s.lastQuoteAt,
		"marketErrors":  marketErrors,
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

func opportunityDTO(opp *strategy.Opportunity) gin.H {
	status := "ready"
	blockReason := ""
	if opp.NetProfit <= 0 {
		status = "blocked"
		blockReason = "净收益小于等于 0"
	}
	return gin.H{
		"id":            opp.ID,
		"symbol":        opp.Symbol,
		"direction":     opp.Direction,
		"spotExchange":  opp.SpotExchange,
		"perpExchange":  opp.PerpExchange,
		"spotPrice":     opp.PriceA,
		"perpPrice":     opp.PriceB,
		"notional":      opp.Notional,
		"basisAmount":   opp.BasisAmount,
		"fundingAmount": opp.FundingAmount,
		"feeCost":       opp.FeeCost,
		"slippage":      opp.Slippage,
		"safetyBuffer":  opp.SafetyBuffer,
		"netProfit":     opp.NetProfit,
		"profitRate":    opp.ProfitRate,
		"status":        status,
		"blockReason":   blockReason,
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

func GetCEXSpotPerpSimulation(c *gin.Context) {
	c.JSON(http.StatusOK, cexSpotPerpSim.snapshot())
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
