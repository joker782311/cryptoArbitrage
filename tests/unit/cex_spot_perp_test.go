package unit_test

import (
	"errors"
	"testing"

	"github.com/joker782311/cryptoArbitrage/internal/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSpotPerpConfig() strategy.CEXSpotPerpConfig {
	return strategy.CEXSpotPerpConfig{
		Symbols:                []string{"BTCUSDT"},
		Exchanges:              []string{"binance", "okx", "bitget"},
		NotionalUSDT:           1000,
		MinNetProfitRate:       0.01,
		FundingIntervals:       1,
		CarryFundingIntervals:  6,
		SpotTakerFeeRate:       0.001,
		PerpTakerFeeRate:       0.0005,
		SlippageRate:           0.0005,
		SafetyBufferRate:       0.0002,
		DefaultLeverage:        3,
		EnableInventoryReverse: true,
	}
}

func TestCEXSpotPerp_CarryCandidateCanBeObservedBeforeExecutable(t *testing.T) {
	cfg := testSpotPerpConfig()
	cfg.MinNetProfitRate = 0.2
	cfg.CarryFundingIntervals = 8
	s := strategy.NewCEXSpotPerpStrategy(cfg)
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "binance", Symbol: "BTCUSDT", MarketType: strategy.MarketTypeSpot, Bid: 9999, Ask: 10000})
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "okx", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 10002, Ask: 10003, FundingRate: 0.0005})

	executable := s.ScanSymbol("BTCUSDT")
	require.Empty(t, executable)

	candidates := s.ScanSymbolCandidates("BTCUSDT")
	require.NotEmpty(t, candidates)
	found := candidates[0]
	assert.Less(t, found.ProfitRate, cfg.MinNetProfitRate)
	assert.Greater(t, found.CarryFundingAmount, found.FundingAmount)
	assert.Greater(t, found.CarryNetProfit, 0.0)
	assert.Equal(t, cfg.CarryFundingIntervals, found.CarryFundingIntervals)
}

func TestCEXSpotPerp_LongSpotShortPerpProfit(t *testing.T) {
	s := strategy.NewCEXSpotPerpStrategy(testSpotPerpConfig())
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "binance", Symbol: "BTCUSDT", MarketType: strategy.MarketTypeSpot, Bid: 9990, Ask: 10000})
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "okx", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 10100, Ask: 10110, FundingRate: 0.001})

	opps := s.ScanSymbol("BTCUSDT")
	require.NotEmpty(t, opps)

	var found *strategy.Opportunity
	for _, opp := range opps {
		if opp.Direction == strategy.DirectionSpotLongPerpShort && opp.SpotExchange == "binance" && opp.PerpExchange == "okx" {
			found = opp
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, strategy.StrategyTypeCEXSpotPerp, found.StrategyType)
	assert.Greater(t, found.BasisAmount, 0.0)
	assert.Greater(t, found.FundingAmount, 0.0)
	assert.Greater(t, found.FeeCost, 0.0)
	assert.Greater(t, found.Slippage, 0.0)
	assert.Greater(t, found.NetProfit, 0.0)
}

func TestCEXSpotPerp_InventoryReverseProfit(t *testing.T) {
	s := strategy.NewCEXSpotPerpStrategy(testSpotPerpConfig())
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "binance", Symbol: "BTCUSDT", MarketType: strategy.MarketTypeSpot, Bid: 10100, Ask: 10110})
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "bitget", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 9990, Ask: 10000, FundingRate: -0.001})

	opps := s.ScanSymbol("BTCUSDT")
	require.NotEmpty(t, opps)

	var found *strategy.Opportunity
	for _, opp := range opps {
		if opp.Direction == strategy.DirectionSpotShortInventoryPerpLong && opp.SpotExchange == "binance" && opp.PerpExchange == "bitget" {
			found = opp
			break
		}
	}
	require.NotNil(t, found)
	assert.Greater(t, found.BasisAmount, 0.0)
	assert.Greater(t, found.FundingAmount, 0.0)
	assert.Equal(t, "sell", found.Legs[0].Side)
	assert.Equal(t, "buy", found.Legs[1].Side)
}

func TestCEXSpotPerp_SimulatorChecksIndependentExchangeBalances(t *testing.T) {
	opp := &strategy.Opportunity{
		ID:           "opp-1",
		StrategyType: strategy.StrategyTypeCEXSpotPerp,
		Direction:    strategy.DirectionSpotLongPerpShort,
		Notional:     1000,
		NetProfit:    10,
		SpotExchange: "bitget",
		PerpExchange: "okx",
		Symbol:       "BTCUSDT",
		PriceA:       10000,
		PriceB:       10100,
		FeeCost:      1.5,
		Legs: []strategy.Leg{
			{Exchange: "bitget", Symbol: "BTCUSDT", Side: "buy", Quantity: 0.1, Price: 10000},
			{Exchange: "okx", Symbol: "BTCUSDT", Side: "sell", Quantity: 0.1, Price: 10100},
		},
	}
	sim := strategy.NewCEXSpotPerpSimulator(map[string]*strategy.SimAccount{
		"binance": {Exchange: "binance", USDT: 10000, PerpUSDT: 10000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
		"bitget":  {Exchange: "bitget", USDT: 0, PerpUSDT: 10000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
		"okx":     {Exchange: "okx", USDT: 10000, PerpUSDT: 10000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
	}, 3)

	_, err := sim.ExecuteOpportunity(opp)
	require.Error(t, err)
	assert.True(t, errors.Is(err, strategy.ErrSimInsufficientUSDT))
}

func TestCEXSpotPerp_SimulatorRequiresInventoryForReverse(t *testing.T) {
	opp := &strategy.Opportunity{
		ID:           "opp-2",
		StrategyType: strategy.StrategyTypeCEXSpotPerp,
		Direction:    strategy.DirectionSpotShortInventoryPerpLong,
		Notional:     1000,
		NetProfit:    10,
		SpotExchange: "binance",
		PerpExchange: "okx",
		Symbol:       "BTCUSDT",
		PriceA:       10000,
		PriceB:       9900,
		FeeCost:      1.5,
		Legs: []strategy.Leg{
			{Exchange: "binance", Symbol: "BTCUSDT", Side: "sell", Quantity: 0.1, Price: 10000},
			{Exchange: "okx", Symbol: "BTCUSDT", Side: "buy", Quantity: 0.1, Price: 9900},
		},
	}
	sim := strategy.NewCEXSpotPerpSimulator(map[string]*strategy.SimAccount{
		"binance": {Exchange: "binance", USDT: 10000, PerpUSDT: 10000, SpotBalances: map[string]float64{"BTC": 0.01}, PerpPositions: map[string]float64{}},
		"okx":     {Exchange: "okx", USDT: 10000, PerpUSDT: 10000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
	}, 3)

	_, err := sim.ExecuteOpportunity(opp)
	require.Error(t, err)
	assert.True(t, errors.Is(err, strategy.ErrSimInsufficientInventory))
}

func TestCEXSpotPerp_SimulatorDoesNotReuseHedgedSpotInventory(t *testing.T) {
	longOpp := &strategy.Opportunity{
		ID:           "opp-long",
		StrategyType: strategy.StrategyTypeCEXSpotPerp,
		Direction:    strategy.DirectionSpotLongPerpShort,
		Notional:     1000,
		NetProfit:    10,
		SpotExchange: "binance",
		PerpExchange: "okx",
		Symbol:       "BTCUSDT",
		PriceA:       10000,
		PriceB:       10100,
		FeeCost:      1.5,
		Legs: []strategy.Leg{
			{Exchange: "binance", Symbol: "BTCUSDT", Side: "buy", Quantity: 0.1, Price: 10000},
			{Exchange: "okx", Symbol: "BTCUSDT", Side: "sell", Quantity: 0.1, Price: 10100},
		},
	}
	reverseOpp := &strategy.Opportunity{
		ID:           "opp-reverse",
		StrategyType: strategy.StrategyTypeCEXSpotPerp,
		Direction:    strategy.DirectionSpotShortInventoryPerpLong,
		Notional:     1000,
		NetProfit:    8,
		SpotExchange: "binance",
		PerpExchange: "okx",
		Symbol:       "BTCUSDT",
		PriceA:       10000,
		PriceB:       9900,
		FeeCost:      1.5,
		Legs: []strategy.Leg{
			{Exchange: "binance", Symbol: "BTCUSDT", Side: "sell", Quantity: 0.1, Price: 10000},
			{Exchange: "okx", Symbol: "BTCUSDT", Side: "buy", Quantity: 0.1, Price: 9900},
		},
	}
	sim := strategy.NewCEXSpotPerpSimulator(map[string]*strategy.SimAccount{
		"binance": {Exchange: "binance", USDT: 5000, PerpUSDT: 5000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
		"okx":     {Exchange: "okx", USDT: 5000, PerpUSDT: 5000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
	}, 3)

	_, err := sim.ExecuteOpportunity(longOpp)
	require.NoError(t, err)

	_, err = sim.ExecuteOpportunity(reverseOpp)
	require.Error(t, err)
	assert.True(t, errors.Is(err, strategy.ErrSimInsufficientInventory))
}

func TestCEXSpotPerp_SimulatorExecutesAndCircuitBreakerCreatesCloseActions(t *testing.T) {
	s := strategy.NewCEXSpotPerpStrategy(testSpotPerpConfig())
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "binance", Symbol: "BTCUSDT", MarketType: strategy.MarketTypeSpot, Bid: 9990, Ask: 10000})
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "okx", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 10100, Ask: 10110, FundingRate: 0.001})
	opp := s.ScanSymbol("BTCUSDT")[0]

	sim := strategy.NewCEXSpotPerpSimulator(map[string]*strategy.SimAccount{
		"binance": {Exchange: "binance", USDT: 5000, PerpUSDT: 5000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
		"okx":     {Exchange: "okx", USDT: 5000, PerpUSDT: 5000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
	}, 3)

	pos, err := sim.ExecuteOpportunity(opp)
	require.NoError(t, err)
	require.Equal(t, "open", pos.Status)

	actions := sim.TriggerCircuitBreaker("daily loss limit")
	require.Len(t, actions, 1)
	assert.Equal(t, pos.ID, actions[0].PositionID)
	assert.Equal(t, "sell", actions[0].Legs[0].Side)
	assert.Equal(t, "buy", actions[0].Legs[1].Side)

	_, err = sim.ExecuteOpportunity(opp)
	require.Error(t, err)
	assert.True(t, errors.Is(err, strategy.ErrSimCircuitBreakerActive))
}

func TestCEXSpotPerp_SimulatorClosesPosition(t *testing.T) {
	s := strategy.NewCEXSpotPerpStrategy(testSpotPerpConfig())
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "binance", Symbol: "BTCUSDT", MarketType: strategy.MarketTypeSpot, Bid: 9990, Ask: 10000})
	s.UpdateQuote(strategy.CEXSpotPerpQuote{Exchange: "okx", Symbol: "BTCUSDT", MarketType: strategy.MarketTypePerp, Bid: 10100, Ask: 10110, FundingRate: 0.001})
	opp := s.ScanSymbol("BTCUSDT")[0]

	sim := strategy.NewCEXSpotPerpSimulator(map[string]*strategy.SimAccount{
		"binance": {Exchange: "binance", USDT: 5000, PerpUSDT: 5000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
		"okx":     {Exchange: "okx", USDT: 5000, PerpUSDT: 5000, SpotBalances: map[string]float64{}, PerpPositions: map[string]float64{}},
	}, 3)

	pos, err := sim.ExecuteOpportunity(opp)
	require.NoError(t, err)
	require.Equal(t, "open", pos.Status)

	estimatedPnL, estimatedRate, err := sim.EstimateClosePnL(pos.ID, 10020, 10040)
	require.NoError(t, err)
	require.NotEqual(t, opp.NetProfit, estimatedPnL)
	assert.InDelta(t, estimatedPnL/pos.Notional*100, estimatedRate, 1e-9)

	action, err := sim.ClosePositionWithMarket(pos.ID, "manual close", 10020, 10040)
	require.NoError(t, err)
	assert.Equal(t, pos.ID, action.PositionID)
	assert.Equal(t, "closed", pos.Status)
	assert.InDelta(t, estimatedPnL, pos.RealizedPnL, 1e-9)

	account, ok := sim.Account("okx")
	require.True(t, ok)
	assert.Equal(t, 0.0, account.FrozenUSDT)

	pnl := sim.PnLSummary()
	assert.InDelta(t, pos.RealizedPnL, pnl["realizedPnL"], 1e-9)
	assert.Equal(t, 0.0, pnl["unrealizedPnL"])
}
