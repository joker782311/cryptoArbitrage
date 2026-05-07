package handlers

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/joker782311/cryptoArbitrage/internal/strategy"
)

type cexSpotPerpState struct {
	mu            sync.RWMutex
	strategy      *strategy.CEXSpotPerpStrategy
	simulator     *strategy.CEXSpotPerpSimulator
	opportunities map[string]*strategy.Opportunity
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

	return &cexSpotPerpState{
		strategy:      s,
		simulator:     sim,
		opportunities: make(map[string]*strategy.Opportunity),
	}
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

	return gin.H{
		"status":        status,
		"haltReason":    reason,
		"accounts":      s.accountDTOs(),
		"opportunities": opps,
		"positions":     positionDTOs(s.simulator.Positions()),
		"closeActions":  closeActionDTOs(s.simulator.CloseActions()),
		"pnl":           s.simulator.PnLSummary(),
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
