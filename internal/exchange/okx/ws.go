package okx

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// WSTicker WebSocket 行情消息
type WSTickerMessage struct {
	InstID string `json:"instId"`
	Last   string `json:"last"`
	BidPx  string `json:"bidPx"`
	AskPx  string `json:"askPx"`
}

// SubscribeTicker 订阅行情 WebSocket
func (c *Client) SubscribeTicker(instIDs []string, handler func(*Ticker)) error {
	args := make([]map[string]string, len(instIDs))
	for i, instID := range instIDs {
		args[i] = map[string]string{
			"channel": "tickers",
			"instId":  instID,
		}
	}

	subscribeMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	jsonBody, _ := json.Marshal(subscribeMsg)

	url := "wss://ws.okx.com:8443/ws/v5/public"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	// 发送订阅消息
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

			// 跳过 pong 消息
			var resp map[string]interface{}
			if err := json.Unmarshal(message, &resp); err != nil {
				continue
			}

			if arg, ok := resp["arg"].(map[string]interface{}); ok {
				if channel, ok := arg["channel"].(string); ok && channel == "tickers" {
					if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
						dataBytes, _ := json.Marshal(data[0])
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
func (c *Client) SubscribeOrderBook(instID string, depth string, handler func(*OrderBook)) error {
	subscribeMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"channel": fmt.Sprintf("books%s", depth),
				"instId":  instID,
			},
		},
	}

	jsonBody, _ := json.Marshal(subscribeMsg)

	url := "wss://ws.okx.com:8443/ws/v5/public"
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
					if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
						dataBytes, _ := json.Marshal(data[0])
						var ob OrderBook
						if err := json.Unmarshal(dataBytes, &ob); err == nil {
							handler(&ob)
						}
					}
				}
			}
		}
	}()

	return nil
}

// login WebSocket 登录（私有频道需要）
func (c *Client) WebSocketLogin(conn *websocket.Conn) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sign := c.sign(timestamp, "GET", "/users/self/verify", "")

	loginMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     c.APIKey,
				"passphrase": c.Passphrase,
				"timestamp":  timestamp,
				"sign":       sign,
			},
		},
	}

	jsonBody, _ := json.Marshal(loginMsg)
	return conn.WriteMessage(websocket.TextMessage, jsonBody)
}
