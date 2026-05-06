package exchange

import (
	"context"
	"fmt"
	"strconv"

	"github.com/joker782311/cryptoArbitrage/internal/exchange/binance"
	"github.com/joker782311/cryptoArbitrage/internal/exchange/bitget"
	"github.com/joker782311/cryptoArbitrage/internal/exchange/okx"
)

// 币安适配器
type BinanceAdapter struct {
	client *binance.Client
}

func (a *BinanceAdapter) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	t, err := a.client.GetTicker(ctx, symbol)
	if err != nil {
		return nil, err
	}
	return &Ticker{
		Exchange:  "binance",
		Symbol:    t.Symbol,
		Price:     t.LastPrice,
		Bid:       t.BidPrice,
		Ask:       t.AskPrice,
		Volume24h: t.Volume,
		Change24h: t.PriceChangePercent,
	}, nil
}

func (a *BinanceAdapter) GetTickers(ctx context.Context) ([]Ticker, error) {
	tickers, err := a.client.GetTickers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Ticker, len(tickers))
	for i, t := range tickers {
		result[i] = Ticker{
			Exchange:  "binance",
			Symbol:    t.Symbol,
			Price:     t.LastPrice,
			Bid:       t.BidPrice,
			Ask:       t.AskPrice,
			Volume24h: t.Volume,
			Change24h: t.PriceChangePercent,
		}
	}
	return result, nil
}

func (a *BinanceAdapter) GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error) {
	ob, err := a.client.GetOrderBook(ctx, symbol, limit)
	if err != nil {
		return nil, err
	}
	bids := make([]PriceLevel, len(ob.Bids))
	asks := make([]PriceLevel, len(ob.Asks))
	for i, b := range ob.Bids {
		bids[i] = PriceLevel{Price: b.Price, Quantity: b.Quantity}
	}
	for i, a := range ob.Asks {
		asks[i] = PriceLevel{Price: a.Price, Quantity: a.Quantity}
	}
	return &OrderBook{
		Exchange: "binance",
		Symbol:   symbol,
		Bids:     bids,
		Asks:     asks,
	}, nil
}

func (a *BinanceAdapter) GetFundingRate(ctx context.Context, symbol string) (*FundingRate, error) {
	rate, err := a.client.GetFundingRate(ctx, symbol)
	if err != nil {
		return nil, err
	}
	f, _ := strconv.ParseFloat(rate.FundingRate, 64)
	return &FundingRate{
		Exchange:    "binance",
		Symbol:      rate.Symbol,
		FundingRate: f,
		NextFunding: rate.FundingTime,
	}, nil
}

func (a *BinanceAdapter) PlaceOrder(ctx context.Context, symbol, side, orderType string, quantity, price float64) (*Order, error) {
	o, err := a.client.PlaceOrder(ctx, symbol, binance.OrderSide(side), binance.OrderType(orderType), quantity, price)
	if err != nil {
		return nil, err
	}
	return &Order{
		Exchange:    "binance",
		Symbol:      o.Symbol,
		Side:        string(o.Side),
		Type:        string(o.Type),
		Price:       o.Price,
		Quantity:    o.Quantity,
		ExecutedQty: o.ExecutedQty,
		Status:      string(o.Status),
		OrderID:     fmt.Sprintf("%d", o.OrderID),
	}, nil
}

func (a *BinanceAdapter) CancelOrder(ctx context.Context, symbol, orderID string) error {
	oid, _ := strconv.ParseInt(orderID, 10, 64)
	return a.client.CancelOrder(ctx, symbol, oid)
}

func (a *BinanceAdapter) GetOrder(ctx context.Context, symbol, orderID string) (*Order, error) {
	oid, _ := strconv.ParseInt(orderID, 10, 64)
	o, err := a.client.GetOrder(ctx, symbol, oid)
	if err != nil {
		return nil, err
	}
	return &Order{
		Exchange:    "binance",
		Symbol:      o.Symbol,
		Side:        string(o.Side),
		Type:        string(o.Type),
		Price:       o.Price,
		Quantity:    o.Quantity,
		ExecutedQty: o.ExecutedQty,
		Status:      string(o.Status),
		OrderID:     fmt.Sprintf("%d", o.OrderID),
	}, nil
}

func (a *BinanceAdapter) GetBalance(ctx context.Context, asset string) (*Balance, error) {
	b, err := a.client.GetBalance(ctx, asset)
	if err != nil {
		return nil, err
	}
	return &Balance{
		Exchange: "binance",
		Asset:    b.Asset,
		Free:     b.Free,
		Locked:   b.Locked,
	}, nil
}

func (a *BinanceAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	positions, err := a.client.GetFuturesPositions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Position, 0)
	for _, p := range positions {
		if p.PositionAmt != 0 {
			result = append(result, Position{
				Exchange:     "binance",
				Symbol:       p.Symbol,
				Side:         p.PositionSide,
				Quantity:     p.PositionAmt,
				EntryPrice:   p.EntryPrice,
				CurrentPrice: p.MarkPrice,
				PNL:          p.UnRealizedPnL,
			})
		}
	}
	return result, nil
}

func (a *BinanceAdapter) SubscribeTicker(ctx context.Context, symbols []string, handler func(*Ticker)) error {
	return a.client.SubscribeTicker(symbols, func(t *binance.Ticker) {
		handler(&Ticker{
			Exchange:  "binance",
			Symbol:    t.Symbol,
			Price:     t.LastPrice,
			Bid:       t.BidPrice,
			Ask:       t.AskPrice,
			Volume24h: t.Volume,
			Change24h: t.PriceChangePercent,
		})
	})
}

func (a *BinanceAdapter) SubscribeOrderBook(ctx context.Context, symbol string, limit int, handler func(*OrderBook)) error {
	return a.client.SubscribeOrderBook(symbol, limit, func(ob *binance.OrderBook) {
		bids := make([]PriceLevel, len(ob.Bids))
		asks := make([]PriceLevel, len(ob.Asks))
		for i, b := range ob.Bids {
			bids[i] = PriceLevel{Price: b.Price, Quantity: b.Quantity}
		}
		for i, a := range ob.Asks {
			asks[i] = PriceLevel{Price: a.Price, Quantity: a.Quantity}
		}
		handler(&OrderBook{
			Exchange: "binance",
			Symbol:   symbol,
			Bids:     bids,
			Asks:     asks,
		})
	})
}

// OKX 适配器
type OKXAdapter struct {
	client *okx.Client
}

func (a *OKXAdapter) GetTicker(ctx context.Context, instID string) (*Ticker, error) {
	t, err := a.client.GetTicker(ctx, instID)
	if err != nil {
		return nil, err
	}
	return &Ticker{
		Exchange:  "okx",
		Symbol:    t.InstID,
		Price:     parseFloat(t.LastPx),
		Bid:       parseFloat(t.BidPx),
		Ask:       parseFloat(t.AskPx),
		Volume24h: parseFloat(t.Vol24h),
		Change24h: parseFloat(t.ChangePercent24h),
	}, nil
}

func (a *OKXAdapter) GetTickers(ctx context.Context) ([]Ticker, error) {
	tickers, err := a.client.GetTickers(ctx, "SWAP")
	if err != nil {
		return nil, err
	}
	result := make([]Ticker, len(tickers))
	for i, t := range tickers {
		result[i] = Ticker{
			Exchange:  "okx",
			Symbol:    t.InstID,
			Price:     parseFloat(t.LastPx),
			Bid:       parseFloat(t.BidPx),
			Ask:       parseFloat(t.AskPx),
			Volume24h: parseFloat(t.Vol24h),
			Change24h: parseFloat(t.ChangePercent24h),
		}
	}
	return result, nil
}

func (a *OKXAdapter) GetOrderBook(ctx context.Context, instID string, limit int) (*OrderBook, error) {
	ob, err := a.client.GetOrderBook(ctx, instID, limit)
	if err != nil {
		return nil, err
	}
	return &OrderBook{
		Exchange: "okx",
		Symbol:   instID,
		Bids:     convertLevels(ob.Bids),
		Asks:     convertLevels(ob.Asks),
	}, nil
}

func (a *OKXAdapter) GetFundingRate(ctx context.Context, instID string) (*FundingRate, error) {
	rate, err := a.client.GetFundingRate(ctx, instID)
	if err != nil {
		return nil, err
	}
	return &FundingRate{
		Exchange:    "okx",
		Symbol:      rate.InstID,
		FundingRate: parseFloat(rate.FundingRate),
	}, nil
}

func (a *OKXAdapter) PlaceOrder(ctx context.Context, instID, side, orderType string, quantity, price float64) (*Order, error) {
	o, err := a.client.PlaceOrder(ctx, instID, okx.OrderSide(side), okx.OrderType(orderType), fmt.Sprintf("%f", quantity), fmt.Sprintf("%f", price))
	if err != nil {
		return nil, err
	}
	return &Order{
		Exchange:    "okx",
		Symbol:      o.InstID,
		Side:        o.Side,
		Type:        o.OrdType,
		Price:       parseFloat(o.FillPx),
		Quantity:    parseFloat(o.Sz),
		ExecutedQty: parseFloat(o.FillSz),
		Status:      o.State,
		OrderID:     o.OrdID,
	}, nil
}

func (a *OKXAdapter) CancelOrder(ctx context.Context, instID, ordID string) error {
	return a.client.CancelOrder(ctx, instID, ordID)
}

func (a *OKXAdapter) GetOrder(ctx context.Context, instID, ordID string) (*Order, error) {
	o, err := a.client.GetOrder(ctx, instID, ordID)
	if err != nil {
		return nil, err
	}
	return &Order{
		Exchange:    "okx",
		Symbol:      o.InstID,
		Side:        o.Side,
		Type:        o.OrdType,
		Price:       parseFloat(o.FillPx),
		Quantity:    parseFloat(o.Sz),
		ExecutedQty: parseFloat(o.FillSz),
		Status:      o.State,
		OrderID:     o.OrdID,
	}, nil
}

func (a *OKXAdapter) GetBalance(ctx context.Context, asset string) (*Balance, error) {
	details, err := a.client.GetBalance(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range details {
		if d.Ccy == asset {
			return &Balance{
				Exchange: "okx",
				Asset:    d.Ccy,
				Free:     parseFloat(d.AvailEq),
				Locked:   parseFloat(d.UavailEq),
			}, nil
		}
	}
	return &Balance{Exchange: "okx", Asset: asset}, nil
}

func (a *OKXAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	positions, err := a.client.GetPositions(ctx, "SWAP", "")
	if err != nil {
		return nil, err
	}
	result := make([]Position, 0)
	for _, p := range positions {
		if parseFloat(p.Pos) != 0 {
			result = append(result, Position{
				Exchange:     "okx",
				Symbol:       p.InstID,
				Side:         p.PosSide,
				Quantity:     parseFloat(p.Pos),
				EntryPrice:   parseFloat(p.AvgPx),
				CurrentPrice: parseFloat(p.AvgPx),
				PNL:          parseFloat(p.Pnl),
				PNLPercent:   parseFloat(p.PnlRatio),
			})
		}
	}
	return result, nil
}

func (a *OKXAdapter) SubscribeTicker(ctx context.Context, instIDs []string, handler func(*Ticker)) error {
	return a.client.SubscribeTicker(instIDs, func(t *okx.Ticker) {
		handler(&Ticker{
			Exchange: "okx",
			Symbol:   t.InstID,
			Price:    parseFloat(t.Last),
			Bid:      parseFloat(t.BidPx),
			Ask:      parseFloat(t.AskPx),
		})
	})
}

func (a *OKXAdapter) SubscribeOrderBook(ctx context.Context, instID string, limit int, handler func(*OrderBook)) error {
	return a.client.SubscribeOrderBook(instID, "400", func(ob *okx.OrderBook) {
		handler(&OrderBook{
			Exchange: "okx",
			Symbol:   instID,
			Bids:     convertLevels(ob.Bids),
			Asks:     convertLevels(ob.Asks),
		})
	})
}

// Bitget 适配器
type BitgetAdapter struct {
	client *bitget.Client
}

func (a *BitgetAdapter) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	t, err := a.client.GetTicker(ctx, symbol)
	if err != nil {
		return nil, err
	}
	return &Ticker{
		Exchange:  "bitget",
		Symbol:    t.Symbol,
		Price:     parseFloat(t.LastPr),
		Bid:       parseFloat(t.BidPr),
		Ask:       parseFloat(t.AskPr),
		Volume24h: parseFloat(t.Vol24h),
		Change24h: parseFloat(t.ChangePct),
	}, nil
}

func (a *BitgetAdapter) GetTickers(ctx context.Context) ([]Ticker, error) {
	tickers, err := a.client.GetTickers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Ticker, len(tickers))
	for i, t := range tickers {
		result[i] = Ticker{
			Exchange:  "bitget",
			Symbol:    t.Symbol,
			Price:     parseFloat(t.LastPr),
			Bid:       parseFloat(t.BidPr),
			Ask:       parseFloat(t.AskPr),
			Volume24h: parseFloat(t.Vol24h),
			Change24h: parseFloat(t.ChangePct),
		}
	}
	return result, nil
}

func (a *BitgetAdapter) GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error) {
	ob, err := a.client.GetOrderBook(ctx, symbol, limit)
	if err != nil {
		return nil, err
	}
	return &OrderBook{
		Exchange: "bitget",
		Symbol:   symbol,
		Bids:     convertLevels(ob.Bids),
		Asks:     convertLevels(ob.Asks),
	}, nil
}

func (a *BitgetAdapter) GetFundingRate(ctx context.Context, symbol string) (*FundingRate, error) {
	rate, err := a.client.GetFundingRate(ctx, symbol)
	if err != nil {
		return nil, err
	}
	return &FundingRate{
		Exchange:    "bitget",
		Symbol:      rate.Symbol,
		FundingRate: parseFloat(rate.FundingRate),
	}, nil
}

func (a *BitgetAdapter) PlaceOrder(ctx context.Context, symbol, side, orderType string, quantity, price float64) (*Order, error) {
	o, err := a.client.PlaceOrder(ctx, symbol, bitget.OrderSide(side), bitget.OrderType(orderType), fmt.Sprintf("%f", quantity), fmt.Sprintf("%f", price))
	if err != nil {
		return nil, err
	}
	return &Order{
		Exchange:    "bitget",
		Symbol:      o.Symbol,
		Side:        o.Side,
		Type:        o.OrderType,
		Price:       parseFloat(o.FillPrice),
		Quantity:    parseFloat(o.Size),
		ExecutedQty: parseFloat(o.FillSize),
		Status:      o.Status,
		OrderID:     o.OrderID,
	}, nil
}

func (a *BitgetAdapter) CancelOrder(ctx context.Context, symbol, orderID string) error {
	return a.client.CancelOrder(ctx, symbol, orderID)
}

func (a *BitgetAdapter) GetOrder(ctx context.Context, symbol, orderID string) (*Order, error) {
	o, err := a.client.GetOrder(ctx, symbol, orderID)
	if err != nil {
		return nil, err
	}
	return &Order{
		Exchange:    "bitget",
		Symbol:      o.Symbol,
		Side:        o.Side,
		Type:        o.OrderType,
		Price:       parseFloat(o.FillPrice),
		Quantity:    parseFloat(o.Size),
		ExecutedQty: parseFloat(o.FillSize),
		Status:      o.Status,
		OrderID:     o.OrderID,
	}, nil
}

func (a *BitgetAdapter) GetBalance(ctx context.Context, asset string) (*Balance, error) {
	b, err := a.client.GetBalance(ctx, asset)
	if err != nil {
		return nil, err
	}
	return &Balance{
		Exchange: "bitget",
		Asset:    b.CoinName,
		Free:     parseFloat(b.Available),
		Locked:   parseFloat(b.Frozen),
	}, nil
}

func (a *BitgetAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	return []Position{}, nil
}

func (a *BitgetAdapter) SubscribeTicker(ctx context.Context, symbols []string, handler func(*Ticker)) error {
	return a.client.SubscribeTicker(symbols, func(t *bitget.Ticker) {
		handler(&Ticker{
			Exchange: "bitget",
			Symbol:   t.Symbol,
			Price:    parseFloat(t.LastPr),
			Bid:      parseFloat(t.BidPr),
			Ask:      parseFloat(t.AskPr),
		})
	})
}

func (a *BitgetAdapter) SubscribeOrderBook(ctx context.Context, symbol string, limit int, handler func(*OrderBook)) error {
	return a.client.SubscribeOrderBook(symbol, "20", func(ob *bitget.OrderBook) {
		handler(&OrderBook{
			Exchange: "bitget",
			Symbol:   symbol,
			Bids:     convertLevels(ob.Bids),
			Asks:     convertLevels(ob.Asks),
		})
	})
}

// 辅助函数
func convertLevels(levels [][]string) []PriceLevel {
	result := make([]PriceLevel, len(levels))
	for i, level := range levels {
		result[i] = PriceLevel{
			Price:    parseFloat(level[0]),
			Quantity: parseFloat(level[1]),
		}
	}
	return result
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}
