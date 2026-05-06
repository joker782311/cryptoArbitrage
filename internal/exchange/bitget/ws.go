package bitget

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

// WSTicker WebSocket 行情消息
type WSTickerMessage struct {
	InstID string `json:"instId"`
	Last   string `json:"last"`
	BidPr  string `json:"bidPr"`
	AskPr  string `json:"askPr"`
}

// SubscribeTicker 订阅行情 WebSocket
func (c *Client) SubscribeTicker(symbols []string, handler func(*Ticker)) error {
	args := make([]map[string]string, len(symbols))
	for i, symbol := range symbols {
		args[i] = map[string]string{
			"instType": "SPOT",
			"instId":   symbol,
		}
	}

	subscribeMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	jsonBody, _ := json.Marshal(subscribeMsg)

	url := "wss://ws.bitget.com/v2/ws/spot"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	if err := conn.WriteMessage(websocket.TextMessage, jsonBody); err != nil {
		return err
	}

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket error: %v", err)
				return
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(message, &resp); err != nil {
				continue
			}

			if arg, ok := resp["arg"].(map[string]interface{}); ok {
				if channel, ok := arg["channel"].(string); ok && channel == "ticker" {
					if data, ok := resp["data"].(map[string]interface{}); ok {
						dataBytes, _ := json.Marshal(data)
						var ticker Ticker
						if err := json.Unmarshal(dataBytes, &ticker); err == nil {
							handler(&ticker)
						}
					}
				}
			}
		}
	}()

	return nil
}

// SubscribeOrderBook 订阅订单簿 WebSocket
func (c *Client) SubscribeOrderBook(symbol string, depth string, handler func(*OrderBook)) error {
	subscribeMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"instType": "SPOT",
				"instId":   symbol,
				"channel":  fmt.Sprintf("books%s", depth),
			},
		},
	}

	jsonBody, _ := json.Marshal(subscribeMsg)

	url := "wss://ws.bitget.com/v2/ws/spot"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	if err := conn.WriteMessage(websocket.TextMessage, jsonBody); err != nil {
		return err
	}

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("WebSocket error: %v", err)
				return
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(message, &resp); err != nil {
				continue
			}

			if arg, ok := resp["arg"].(map[string]interface{}); ok {
				if channel, ok := arg["channel"].(string); ok && len(channel) >= 5 && channel[:5] == "books" {
					if data, ok := resp["data"].(map[string]interface{}); ok {
						if bids, ok := data["bids"].([]interface{}); ok {
							ob := &OrderBook{
								Bids: make([][]string, len(bids)),
								Asks: make([][]string, 0),
							}
							for i, bid := range bids {
								if level, ok := bid.([]interface{}); ok {
									ob.Bids[i] = []string{
										fmt.Sprintf("%v", level[0]),
										fmt.Sprintf("%v", level[1]),
									}
								}
							}
							handler(ob)
						}
					}
				}
			}
		}
	}()

	return nil
}
