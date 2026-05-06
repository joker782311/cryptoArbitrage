package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// WSTicker WebSocket 行情消息
type WSTickerMessage struct {
	Symbol             string `json:"s"`
	Price              string `json:"c"`
	Open               string `json:"o"`
	High               string `json:"h"`
	Low                string `json:"l"`
	Volume             string `json:"v"`
	QuoteVolume        string `json:"q"`
	PriceChange        string `json:"p"`
	PriceChangePercent string `json:"P"`
}

// WSOrderBook WebSocket 订单簿消息
type WSOrderBookMessage struct {
	Symbol     string          `json:"s"`
	FirstUpdateID int64       `json:"firstUpdateId"`
	FinalUpdateID int64       `json:"finalUpdateId"`
	Bids       [][]string      `json:"b"`
	Asks       [][]string      `json:"a"`
}

// SubscribeTicker 订阅行情 WebSocket
func (c *Client) SubscribeTicker(symbols []string, handler func(*Ticker)) error {
	streams := ""
	for _, symbol := range symbols {
		streams += fmt.Sprintf("%s@ticker/", symbol)
	}
	streams = streams[:len(streams)-1] // 去掉最后一个 /

	url := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s", streams)
	return c.connectWebSocket(url, func(message []byte) {
		var wsTicker WSTickerMessage
		if err := json.Unmarshal(message, &wsTicker); err != nil {
			return
		}

		ticker := &Ticker{
			Symbol:             wsTicker.Symbol,
			LastPrice:          parseFloat(wsTicker.Price),
			PriceChangePercent: parseFloat(wsTicker.PriceChangePercent),
			Volume:             parseFloat(wsTicker.Volume),
		}
		handler(ticker)
	})
}

// SubscribeOrderBook 订阅订单簿 WebSocket
func (c *Client) SubscribeOrderBook(symbol string, level int, handler func(*OrderBook)) error {
	// level: 5, 10, 20
	url := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@depth%d@100ms", symbol, level)
	return c.connectWebSocket(url, func(message []byte) {
		var wsOB WSOrderBookMessage
		if err := json.Unmarshal(message, &wsOB); err != nil {
			return
		}

		ob := &OrderBook{
			Bids: make([]Level, len(wsOB.Bids)),
			Asks: make([]Level, len(wsOB.Asks)),
		}

		for i, bid := range wsOB.Bids {
			ob.Bids[i] = Level{
				Price:    parseFloat(bid[0]),
				Quantity: parseFloat(bid[1]),
			}
		}

		for i, ask := range wsOB.Asks {
			ob.Asks[i] = Level{
				Price:    parseFloat(ask[0]),
				Quantity: parseFloat(ask[1]),
			}
		}

		handler(ob)
	})
}

func (c *Client) connectWebSocket(url string, handler func([]byte)) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer conn.Close()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket error: %v", err)
				return
			}
			handler(message)
		}
	}()

	wg.Wait()
	return nil
}
