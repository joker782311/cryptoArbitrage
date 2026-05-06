# Crypto Arbitrage Platform Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个完整的币圈套利交易平台，支持 CEX 和 DEX 多种套利策略，实时监控、自动交易、前端可视化

**Architecture:** 单体 Go 后端 + Vue 3 前端 + MySQL + Redis，Docker 容器化部署

**Tech Stack:** Go 1.21+, Vue 3 + TypeScript, MySQL 8, Redis 7, Docker

---

## Phase 1: 核心基础

### Task 1: 项目骨架初始化

**Files:**
- Create: `cmd/main.go`
- Create: `internal/config/config.go`
- Create: `internal/database/mysql.go`
- Create: `internal/database/redis.go`
- Create: `pkg/logger/logger.go`
- Create: `docker-compose.yml`
- Create: `Dockerfile`
- Create: `go.mod`
- Create: `.env.example`
- Create: `Makefile`

- [ ] **Step 1: 初始化 Go 模块**

```bash
cd /Users/henry/Documents/mycode/ai-money
go mod init github.com/ai-money/arbitrage
```

- [ ] **Step 2: 创建目录结构**

```bash
mkdir -p cmd internal/config internal/database internal/api internal/service internal/model pkg/logger pkg/utils web
```

- [ ] **Step 3: 创建 go.mod**

```go
module github.com/ai-money/arbitrage

go 1.21

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/go-redis/redis/v8 v8.11.5
	gorm.io/driver/mysql v1.5.2
	gorm.io/gorm v1.25.5
)
```

- [ ] **Step 4: 创建配置结构 `internal/config/config.go`**

```go
package config

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
}

type ServerConfig struct {
	Port string
	Mode string // debug, release
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("SERVER_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "arbitrage"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
```

- [ ] **Step 5: 创建 MySQL 连接 `internal/database/mysql.go`**

```go
package database

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitMySQL(dsn string) error {
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	return err
}
```

- [ ] **Step 6: 创建 Redis 连接 `internal/database/redis.go`**

```go
package database

import (
	"context"
	"github.com/go-redis/redis/v8"
)

var Redis *redis.Client
var Ctx = context.Background()

func InitRedis(addr, password string) error {
	Redis = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	_, err := Redis.Ping(Ctx).Result()
	return err
}
```

- [ ] **Step 7: 创建日志模块 `pkg/logger/logger.go`**

```go
package logger

import "go.uber.org/zap"

var Log *zap.SugaredLogger

func Init() error {
	var err error
	Log, err = zap.NewDevelopment()
	if err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 8: 创建主入口 `cmd/main.go`**

```go
package main

import (
	"fmt"
	"github.com/ai-money/arbitrage/internal/config"
	"github.com/ai-money/arbitrage/internal/database"
	"github.com/ai-money/arbitrage/pkg/logger"
)

func main() {
	cfg := config.Load()
	
	if err := logger.Init(); err != nil {
		panic(err)
	}
	
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName)
	
	if err := database.InitMySQL(dsn); err != nil {
		panic(err)
	}
	
	if err := database.InitRedis(cfg.Redis.Host+":"+cfg.Redis.Port, cfg.Redis.Password); err != nil {
		panic(err)
	}
	
	logger.Log.Info("Server starting on port " + cfg.Server.Port)
}
```

- [ ] **Step 9: 创建 Dockerfile**

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /arbitrage ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /arbitrage .
COPY --from=builder /app/.env.example .env

EXPOSE 8080
CMD ["./arbitrage"]
```

- [ ] **Step 10: 创建 docker-compose.yml**

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=mysql
      - DB_PORT=3306
      - DB_USER=root
      - DB_PASSWORD=root123
      - DB_NAME=arbitrage
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root123
      MYSQL_DATABASE: arbitrage
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3306:3306"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  mysql_data:
```

- [ ] **Step 11: 创建 .env.example**

```
SERVER_PORT=8080
SERVER_MODE=debug

DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=arbitrage

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
```

- [ ] **Step 12: 创建 Makefile**

```makefile
.PHONY: build run test docker

build:
	go build -o bin/arbitrage ./cmd

run:
	go run ./cmd/main.go

test:
	go test -v ./...

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down
```

- [ ] **Step 13: 运行测试编译**

```bash
go mod tidy
go build ./cmd
```

Expected: 编译成功

- [ ] **Step 14: Commit**

```bash
git add .
git commit -m "feat: initialize project skeleton"
```

---

### Task 2: 数据库模型设计

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/model/api_key.go`
- Create: `internal/model/strategy.go`
- Create: `internal/model/order.go`
- Create: `internal/model/position.go`
- Create: `internal/model/alert.go`
- Create: `internal/model/ticker.go`
- Create: `internal/database/migrate.go`

- [ ] **Step 1: 创建用户模型 `internal/model/user.go`**

```go
package model

import "time"

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Username  string    `gorm:"uniqueIndex;size:50" json:"username"`
	Password  string    `gorm:"size:255" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: 创建 API Key 模型 `internal/model/api_key.go`**

```go
package model

import "time"

type APIKey struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	Exchange   string `gorm:"size:20;index" json:"exchange"` // binance, okx, bitget
	Name       string `gorm:"size:50" json:"name"`
	APIKey     string `gorm:"size:255" json:"api_key"`       // 加密存储
	APISecret  string `gorm:"size:255" json:"api_secret"`    // 加密存储
	Passphrase string `gorm:"size:255" json:"passphrase"`    // OKX/Bitget 需要
	IsEnabled  bool   `gorm:"default:true" json:"is_enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
```

- [ ] **Step 3: 创建策略配置模型 `internal/model/strategy.go`**

```go
package model

import "time"

type Strategy struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	Name            string    `gorm:"size:50;uniqueIndex" json:"name"` // cross_exchange, funding_rate, spot_future, triangular, dex_triangular, dex_cross_dex
	IsEnabled       bool      `gorm:"default:false" json:"is_enabled"`
	AutoExecute     bool      `gorm:"default:false" json:"auto_execute"`
	MinProfitRate   float64   `gorm:"type:decimal(10,8)" json:"min_profit_rate"` // 最小利润率
	MaxPosition     float64   `gorm:"type:decimal(20,8)" json:"max_position"`    // 最大仓位
	StopLossRate    float64   `gorm:"type:decimal(10,8)" json:"stop_loss_rate"`  // 止损率
	Config          string    `gorm:"type:text" json:"config"`                   // JSON 配置
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
```

- [ ] **Step 4: 创建订单模型 `internal/model/order.go`**

```go
package model

import "time"

type Order struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	StrategyID    uint      `gorm:"index" json:"strategy_id"`
	Exchange      string    `gorm:"size:20;index" json:"exchange"`
	Symbol        string    `gorm:"size:20" json:"symbol"`
	Side          string    `gorm:"size:10" json:"side"` // buy, sell
	Type          string    `gorm:"size:10" json:"type"` // market, limit
	Price         float64   `gorm:"type:decimal(20,8)" json:"price"`
	Quantity      float64   `gorm:"type:decimal(20,8)" json:"quantity"`
	ExecutedQty   float64   `gorm:"type:decimal(20,8)" json:"executed_qty"`
	Status        string    `gorm:"size:20" json:"status"` // pending, filled, cancelled, failed
	OrderID       string    `gorm:"size:50" json:"order_id"` // 交易所订单 ID
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
```

- [ ] **Step 5: 创建仓位模型 `internal/model/position.go`**

```go
package model

import "time"

type Position struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Exchange       string    `gorm:"size:20;index" json:"exchange"`
	Symbol         string    `gorm:"size:20" json:"symbol"`
	Side           string    `gorm:"size:10" json:"side"` // long, short
	Quantity       float64   `gorm:"type:decimal(20,8)" json:"quantity"`
	EntryPrice     float64   `gorm:"type:decimal(20,8)" json:"entry_price"`
	CurrentPrice   float64   `gorm:"type:decimal(20,8)" json:"current_price"`
	PNL            float64   `gorm:"type:decimal(20,8)" json:"pnl"`
	PNLPercent     float64   `gorm:"type:decimal(10,4)" json:"pnl_percent"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
```

- [ ] **Step 6: 创建告警模型 `internal/model/alert.go`**

```go
package model

import "time"

type Alert struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Type      string    `gorm:"size:20" json:"type"` // price, opportunity, system, order
	Level     string    `gorm:"size:10" json:"level"` // info, warning, error
	Title     string    `gorm:"size:100" json:"title"`
	Message   string    `gorm:"type:text" json:"message"`
	IsRead    bool      `gorm:"default:false" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type AlertConfig struct {
	ID            uint   `gorm:"primarykey" json:"id"`
	Channel       string `gorm:"size:20" json:"channel"` // telegram, slack, email, sms, webhook
	WebhookURL    string `gorm:"size:255" json:"webhook_url"`
	Email         string `gorm:"size:100" json:"email"`
	Phone         string `gorm:"size:20" json:"phone"`
	ChatID        string `gorm:"size:50" json:"chat_id"` // Telegram
	IsEnabled     bool   `gorm:"default:true" json:"is_enabled"`
}
```

- [ ] **Step 7: 创建行情模型 `internal/model/ticker.go`**

```go
package model

import "time"

type Ticker struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Exchange  string    `gorm:"size:20;index" json:"exchange"`
	Symbol    string    `gorm:"size:20;index" json:"symbol"`
	Price     float64   `gorm:"type:decimal(20,8)" json:"price"`
	Bid       float64   `gorm:"type:decimal(20,8)" json:"bid"`
	Ask       float64   `gorm:"type:decimal(20,8)" json:"ask"`
	Volume24h float64   `gorm:"type:decimal(20,8)" json:"volume_24h"`
	Change24h float64   `gorm:"type:decimal(10,4)" json:"change_24h"`
	Timestamp time.Time `json:"timestamp"`
}

type FundingRate struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Exchange    string    `gorm:"size:20;index" json:"exchange"`
	Symbol      string    `gorm:"size:20;index" json:"symbol"`
	FundingRate float64   `gorm:"type:decimal(10,8)" json:"funding_rate"`
	NextFunding time.Time `json:"next_funding"`
	Timestamp   time.Time `json:"timestamp"`
}
```

- [ ] **Step 8: 创建数据库迁移 `internal/database/migrate.go`**

```go
package database

import (
	"github.com/ai-money/arbitrage/internal/model"
)

func Migrate() error {
	return DB.AutoMigrate(
		&model.User{},
		&model.APIKey{},
		&model.Strategy{},
		&model.Order{},
		&model.Position{},
		&model.Alert{},
		&model.AlertConfig{},
		&model.Ticker{},
		&model.FundingRate{},
	)
}
```

- [ ] **Step 9: 更新 main.go 调用迁移**

```go
if err := database.InitMySQL(dsn); err != nil {
	panic(err)
}

if err := database.Migrate(); err != nil {
	panic(err)
}
```

- [ ] **Step 10: Commit**

```bash
git add .
git commit -m "feat: add database models and migration"
```

---

### Task 3: CEX 模块 - 币安接入

**Files:**
- Create: `internal/exchange/binance/client.go`
- Create: `internal/exchange/binance/market.go`
- Create: `internal/exchange/binance/order.go`
- Create: `internal/exchange/binance/account.go`
- Test: `internal/exchange/binance/client_test.go`

- [ ] **Step 1: 创建币安客户端 `internal/exchange/binance/client.go`**

```go
package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	BaseURL = "https://api.binance.com"
)

type Client struct {
	APIKey    string
	SecretKey string
	Client    *http.Client
}

func NewClient(apiKey, secretKey string) *Client {
	return &Client{
		APIKey:    apiKey,
		SecretKey: secretKey,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(query))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func (c *Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(result)
}
```

- [ ] **Step 2: 创建行情结构 `internal/exchange/binance/market.go`**

```go
package binance

import (
	"encoding/json"
	"net/http"
)

type Ticker struct {
	Symbol             string  `json:"symbol"`
	LastPrice          float64 `json:"lastPrice,string"`
	BidPrice           float64 `json:"bidPrice,string"`
	AskPrice           float64 `json:"askPrice,string"`
	Volume             float64 `json:"volume,string"`
	PriceChangePercent float64 `json:"priceChangePercent,string"`
}

type OrderBook struct {
	LastUpdateID int64          `json:"lastUpdateId"`
	Bids         [][]string     `json:"bids"` // [][price, quantity]
	Asks         [][]string     `json:"asks"`
}

type FundingRate struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
}

func (c *Client) GetTicker(symbol string) (*Ticker, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/24hr?symbol=%s", BaseURL, symbol)
	req, _ := http.NewRequest("GET", url, nil)
	
	var result Ticker
	err := c.do(req, &result)
	return &result, err
}

func (c *Client) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/api/v3/depth?symbol=%s&limit=%d", BaseURL, symbol, limit)
	req, _ := http.NewRequest("GET", url, nil)
	
	var result OrderBook
	err := c.do(req, &result)
	return &result, err
}

func (c *Client) GetFundingRate(symbol string) (*FundingRate, error) {
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", BaseURL, symbol)
	req, _ := http.NewRequest("GET", url, nil)
	
	var result FundingRate
	err := c.do(req, &result)
	return &result, err
}
```

- [ ] **Step 3: 创建订单结构 `internal/exchange/binance/order.go`**

```go
package binance

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type OrderSide string
type OrderType string

const (
	SideBuy  OrderSide = "BUY"
	SideSell OrderSide = "SELL"
	TypeMarket OrderType = "MARKET"
	TypeLimit  OrderType = "LIMIT"
)

type Order struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	Type         string  `json:"type"`
	Price        float64 `json:"price,string,omitempty"`
	Quantity     float64 `json:"quantity,string"`
	ExecutedQty  float64 `json:"executedQty,string"`
	Status       string  `json:"status"`
	OrderID      int64   `json:"orderId"`
	TransactTime int64   `json:"transactTime"`
}

func (c *Client) PlaceOrder(symbol string, side OrderSide, orderType OrderType, quantity float64, price ...float64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", string(side))
	params.Set("type", string(orderType))
	params.Set("quantity", fmt.Sprintf("%f", quantity))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	
	if len(price) > 0 {
		params.Set("price", fmt.Sprintf("%f", price[0]))
	}
	
	signature := c.sign(params.Encode())
	
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v3/order?%s&signature=%s", BaseURL, params.Encode(), signature), nil)
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	
	var result Order
	err := c.do(req, &result)
	return &result, err
}

func (c *Client) CancelOrder(symbol string, orderID int64) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", fmt.Sprintf("%d", orderID))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	
	signature := c.sign(params.Encode())
	
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v3/order?%s&signature=%s", BaseURL, params.Encode(), signature), nil)
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	
	var result map[string]interface{}
	return c.do(req, &result)
}

func (c *Client) GetOrder(symbol string, orderID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", fmt.Sprintf("%d", orderID))
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	
	signature := c.sign(params.Encode())
	
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v3/order?%s&signature=%s", BaseURL, params.Encode(), signature), nil)
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	
	var result Order
	err := c.do(req, &result)
	return &result, err
}
```

- [ ] **Step 4: 创建账户结构 `internal/exchange/binance/account.go`**

```go
package binance

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Account struct {
	MakerCommission int  `json:"makerCommission"`
	TakerCommission int  `json:"takerCommission"`
	Balances        []Balance `json:"balances"`
}

type Balance struct {
	Asset  string `json:"asset"`
	Free   float64 `json:"free,string"`
	Locked float64 `json:"locked,string"`
}

type Position struct {
	Symbol        string  `json:"symbol"`
	PositionAmt   float64 `json:"positionAmt,string"`
	EntryPrice    float64 `json:"entryPrice,string"`
	MarkPrice     float64 `json:"markPrice,string"`
	UnRealizedPnL float64 `json:"unRealizedPnL,string"`
	Side          string  `json:"positionSide"`
}

func (c *Client) GetAccount() (*Account, error) {
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	
	signature := c.sign(params.Encode())
	
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v3/account?%s&signature=%s", BaseURL, params.Encode(), signature), nil)
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	
	var result Account
	err := c.do(req, &result)
	return &result, err
}

func (c *Client) GetPositions() ([]Position, error) {
	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	
	signature := c.sign(params.Encode())
	
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/fapi/v2/positionRisk?%s&signature=%s", BaseURL, params.Encode(), signature), nil)
	req.Header.Set("X-MBX-APIKEY", c.APIKey)
	
	var result []Position
	err := c.do(req, &result)
	return result, err
}
```

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "feat: add Binance exchange client"
```

---

### Task 4: CEX 模块 - OKX 和 Bitget 接入

**Files:**
- Create: `internal/exchange/okx/client.go`
- Create: `internal/exchange/okx/market.go`
- Create: `internal/exchange/okx/order.go`
- Create: `internal/exchange/bitget/client.go`
- Create: `internal/exchange/bitget/market.go`
- Create: `internal/exchange/bitget/order.go`

- [ ] **Step 1: 创建 OKX 客户端 `internal/exchange/okx/client.go`**

```go
package okx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	BaseURL = "https://www.okx.com"
)

type Client struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	Client     *http.Client
}

func NewClient(apiKey, secretKey, passphrase string) *Client {
	return &Client{
		APIKey:     apiKey,
		SecretKey:  secretKey,
		Passphrase: passphrase,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("OK-ACCESS-KEY", c.APIKey)
	req.Header.Set("OK-ACCESS-SIGN", c.HeaderSign(req.Method, req.URL.Path, ""))
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.Passphrase)
	req.Header.Set("OK-ACCESS-TIMESTAMP", time.Now().UTC().Format(time.RFC3339))
	
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(result)
}

func (c *Client) HeaderSign(method, path, body string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return c.sign(timestamp, method, path, body)
}
```

- [ ] **Step 2: 创建 OKX 行情 `internal/exchange/okx/market.go`**

```go
package okx

import (
	"encoding/json"
	"net/http"
)

type Ticker struct {
	InstID string `json:"instId"`
	Last   string `json:"last"`
	BidPx  string `json:"bidPx"`
	AskPx  string `json:"askPx"`
	Vol24h string `json:"vol24h"`
}

type OrderBook struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

func (c *Client) GetTicker(instID string) (*Ticker, error) {
	url := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", BaseURL, instID)
	req, _ := http.NewRequest("GET", url, nil)
	
	var resp struct {
		Code string   `json:"code"`
		Data []Ticker `json:"data"`
	}
	err := c.do(req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error: %s", resp.Code)
	}
	return &resp.Data[0], nil
}

func (c *Client) GetOrderBook(instID string, limit int) (*OrderBook, error) {
	url := fmt.Sprintf("%s/api/v5/market/books?instId=%s&sz=%d", BaseURL, instID, limit)
	req, _ := http.NewRequest("GET", url, nil)
	
	var resp struct {
		Code string     `json:"code"`
		Data []OrderBook `json:"data"`
	}
	err := c.do(req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error: %s", resp.Code)
	}
	return &resp.Data[0], nil
}
```

- [ ] **Step 3: 创建 OKX 订单 `internal/exchange/okx/order.go`**

```go
package okx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Order struct {
	InstID  string `json:"instId"`
	Side    string `json:"side"` // buy, sell
	OrdType string `json:"ordType"` // market, limit
	Px      string `json:"px,omitempty"`
	Sz      string `json:"sz"`
	OrdID   string `json:"ordId"`
	State   string `json:"state"`
}

func (c *Client) PlaceOrder(instID, side, ordType, sz string, px ...string) (*Order, error) {
	body := map[string]interface{}{
		"instId":  instID,
		"side":    side,
		"ordType": ordType,
		"sz":      sz,
	}
	if len(px) > 0 {
		body["px"] = px[0]
	}
	
	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v5/trade/order", BaseURL)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	var resp struct {
		Code string   `json:"code"`
		Data []Order  `json:"data"`
	}
	err := c.do(req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error: %s", resp.Code)
	}
	return &resp.Data[0], nil
}

func (c *Client) CancelOrder(instID, ordID string) error {
	body := map[string]string{
		"instId": instID,
		"ordId":  ordID,
	}
	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v5/trade/cancel-order", BaseURL)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	
	var resp struct {
		Code string `json:"code"`
	}
	return c.do(req, &resp)
}
```

- [ ] **Step 4: 创建 Bitget 客户端 `internal/exchange/bitget/client.go`**

```go
package bitget

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	BaseURL = "https://api.bitget.com"
)

type Client struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	Client     *http.Client
}

func NewClient(apiKey, secretKey, passphrase string) *Client {
	return &Client{
		APIKey:     apiKey,
		SecretKey:  secretKey,
		Passphrase: passphrase,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) do(req *http.Request, result interface{}) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("ACCESS-KEY", c.APIKey)
	req.Header.Set("ACCESS-SIGN", c.sign(timestamp, req.Method, req.URL.Path, ""))
	req.Header.Set("ACCESS-PASSPHRASE", c.Passphrase)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(result)
}
```

- [ ] **Step 5: 创建 Bitget 行情和订单 (类似 OKX 结构)**

`internal/exchange/bitget/market.go` 和 `internal/exchange/bitget/order.go` 参照 OKX 实现

- [ ] **Step 6: Commit**

```bash
git add .
git commit -m "feat: add OKX and Bitget exchange clients"
```

---

### Task 5: CEX 统一接口和 WebSocket 行情

**Files:**
- Create: `internal/exchange/interface.go`
- Create: `internal/exchange/factory.go`
- Create: `internal/exchange/binance/ws.go`
- Create: `internal/exchange/okx/ws.go`
- Create: `internal/exchange/bitget/ws.go`

- [ ] **Step 1: 创建交易所统一接口 `internal/exchange/interface.go`**

```go
package exchange

import "context"

type Ticker struct {
	Exchange  string  `json:"exchange"`
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Bid       float64 `json:"bid"`
	Ask       float64 `json:"ask"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"`
	Timestamp int64   `json:"timestamp"`
}

type OrderBook struct {
	Exchange string `json:"exchange"`
	Symbol   string `json:"symbol"`
	Bids     []PriceLevel `json:"bids"`
	Asks     []PriceLevel `json:"asks"`
}

type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

type Order struct {
	Exchange  string  `json:"exchange"`
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Type      string  `json:"type"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"`
	OrderID   string  `json:"order_id"`
	Status    string  `json:"status"`
}

type Exchange interface {
	GetTicker(ctx context.Context, symbol string) (*Ticker, error)
	GetOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error)
	PlaceOrder(ctx context.Context, symbol, side, orderType string, quantity, price float64) (*Order, error)
	CancelOrder(ctx context.Context, symbol, orderID string) error
	GetOrder(ctx context.Context, symbol, orderID string) (*Order, error)
	SubscribeTicker(ctx context.Context, symbol string, handler func(*Ticker)) error
}
```

- [ ] **Step 2: 创建交易所工厂 `internal/exchange/factory.go`**

```go
package exchange

import (
	"fmt"
	"github.com/ai-money/arbitrage/internal/exchange/binance"
	"github.com/ai-money/arbitrage/internal/exchange/okx"
	"github.com/ai-money/arbitrage/internal/exchange/bitget"
)

func NewExchange(name, apiKey, secretKey, passphrase string) (Exchange, error) {
	switch name {
	case "binance":
		return binance.NewClient(apiKey, secretKey), nil
	case "okx":
		return okx.NewClient(apiKey, secretKey, passphrase), nil
	case "bitget":
		return bitget.NewClient(apiKey, secretKey, passphrase), nil
	default:
		return nil, fmt.Errorf("unknown exchange: %s", name)
	}
}
```

- [ ] **Step 3: 创建币安 WebSocket `internal/exchange/binance/ws.go`**

```go
package binance

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
)

type WSTicker struct {
	Symbol string `json:"s"`
	Price  string `json:"c"`
}

func (c *Client) SubscribeTicker(symbol string, handler func(*Ticker)) error {
	symbol = fmt.Sprintf("%s@ticker", symbol)
	url := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s", symbol)
	
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		
		var wsTicker WSTicker
		if err := json.Unmarshal(message, &wsTicker); err != nil {
			continue
		}
		
		// 转换为统一 Ticker 结构
		ticker := &Ticker{
			Symbol: wsTicker.Symbol,
			Price:  parseFloat(wsTicker.Price),
		}
		handler(ticker)
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "feat: add exchange interface and WebSocket support"
```

---

## Phase 2: DEX 模块

### Task 6: DEX 模块 - EVM 链集成

**Files:**
- Create: `internal/dex/evm/client.go`
- Create: `internal/dex/evm/price.go`
- Create: `internal/dex/evm/contracts.go`
- Create: `internal/dex/evm/uniswap.go`
- Create: `internal/dex/evm/curve.go`

- [ ] **Step 1: 创建 EVM 客户端 `internal/dex/evm/client.go`**

```go
package evm

import (
	"context"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

type ChainID uint64

const (
	Ethereum ChainID = 1
	BSC      ChainID = 56
	Polygon  ChainID = 137
	Arbitrum ChainID = 42161
	Optimism ChainID = 10
	Base     ChainID = 8453
)

var RPCURLs = map[ChainID]string{
	Ethereum: "https://eth-mainnet.g.alchemy.com/v2/",
	BSC:      "https://bsc-dataseed.binance.org",
	Polygon:  "https://polygon-rpc.com",
	Arbitrum: "https://arb1.arbitrum.io/rpc",
	Optimism: "https://mainnet.optimism.io",
	Base:     "https://mainnet.base.org",
}

type Client struct {
	ChainID ChainID
	Client  *ethclient.Client
}

func NewClient(chainID ChainID, rpcURL string) (*Client, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		ChainID: chainID,
		Client:  client,
	}, nil
}

func (c *Client) GetBlockNumber(ctx context.Context) (uint64, error) {
	return c.Client.BlockNumber(ctx)
}

func (c *Client) GetBalance(ctx context.Context, account string) (*big.Int, error) {
	address := common.HexToAddress(account)
	return c.Client.BalanceAt(ctx, address, nil)
}
```

- [ ] **Step 2: 创建价格查询 `internal/dex/evm/price.go`**

```go
package evm

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type PriceResult struct {
	Token0  string  `json:"token0"`
	Token1  string  `json:"token1"`
	Price   float64 `json:"price"`
	Pool    string  `json:"pool"`
	Dex     string  `json:"dex"`
}

func (c *Client) GetTokenPrice(ctx context.Context, tokenAddress, quoteToken string) (*PriceResult, error) {
	// 通过 Uniswap V3 Pool 获取价格
	poolAddress := getPoolAddress(tokenAddress, quoteToken)
	return c.getUniswapV3Price(ctx, poolAddress)
}

func (c *Client) getUniswapV3Price(ctx context.Context, poolAddress string) (*PriceResult, error) {
	pool := common.HexToAddress(poolAddress)
	// 调用 pool.slot0() 获取 sqrtPriceX96
	// 计算价格 = (sqrtPriceX96 / 2^96)^2
	// 简化实现，实际需要使用 abi/bind
	return &PriceResult{}, nil
}
```

- [ ] **Step 3: 创建 Uniswap 合约绑定 `internal/dex/evm/uniswap.go`**

需要生成 Uniswap V2/V3 合约的 Go binding：

```bash
abigen --abi=UniswapV3Pool.json --pkg=uniswap --out=internal/dex/evm/uniswap_v3.go
```

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "feat: add EVM DEX integration"
```

---

### Task 7: DEX 模块 - Solana 集成

**Files:**
- Create: `internal/dex/solana/client.go`
- Create: `internal/dex/solana/price.go`
- Create: `internal/dex/solana/raydium.go`
- Create: `internal/dex/solana/jupiter.go`

- [ ] **Step 1: 创建 Solana 客户端 `internal/dex/solana/client.go`**

```go
package solana

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Client struct {
	RPC      *rpc.Client
	WS       *rpc.Streamer
}

func NewClient(endpoint, wsEndpoint string) (*Client, error) {
	rpcClient := rpc.New(endpoint)
	wsClient, err := rpc.NewStreamer(wsEndpoint, rpc.StreamerOpts{})
	if err != nil {
		return nil, err
	}
	return &Client{
		RPC: rpcClient,
		WS:  wsClient,
	}, nil
}

func (c *Client) GetSlot(ctx context.Context) (uint64, error) {
	return c.RPC.GetSlot(ctx, rpc.CommitmentFinalized)
}
```

- [ ] **Step 2: 创建 Raydium 价格查询 `internal/dex/solana/raydium.go`**

```go
package solana

import (
	"context"
)

func (c *Client) GetRaydiumPrice(ctx context.Context, poolAddress string) (float64, error) {
	// 查询 Raydium AMM 池价格
	// 需要解析池子账户数据
	return 0, nil
}
```

- [ ] **Step 3: 创建 Jupiter 聚合器 `internal/dex/solana/jupiter.go`**

```go
package solana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const JupiterAPI = "https://quote-api.jup.ag/v6"

type QuoteRequest struct {
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	Amount     uint64 `json:"amount"`
}

type QuoteResponse struct {
	InAmount        string `json:"inAmount"`
	OutAmount       string `json:"outAmount"`
	PriceImpactPct  string `json:"priceImpactPct"`
}

func (c *Client) GetJupiterQuote(ctx context.Context, inputMint, outputMint string, amount uint64) (*QuoteResponse, error) {
	url := fmt.Sprintf("%s/quote?inputMint=%s&outputMint=%s&amount=%d",
		JupiterAPI, inputMint, outputMint, amount)
	
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	
	var quote QuoteResponse
	json.NewDecoder(resp.Body).Decode(&quote)
	return &quote, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "feat: add Solana DEX integration"
```

---

## Phase 3: 策略引擎

### Task 8: 跨交易所套利策略

**Files:**
- Create: `internal/strategy/cross_exchange.go`
- Create: `internal/strategy/opportunity.go`
- Test: `internal/strategy/cross_exchange_test.go`

- [ ] **Step 1: 创建套利机会结构 `internal/strategy/opportunity.go`**

```go
package strategy

type Opportunity struct {
	StrategyType string  `json:"strategy_type"`
	Timestamp    int64   `json:"timestamp"`
	ProfitRate   float64 `json:"profit_rate"` // 利润率
	ProfitAmount float64 `json:"profit_amount"` // 利润金额
	
	// 跨交易所字段
	ExchangeA   string  `json:"exchange_a"`
	ExchangeB   string  `json:"exchange_b"`
	Symbol      string  `json:"symbol"`
	PriceA      float64 `json:"price_a"`
	PriceB      float64 `json:"price_b"`
	
	// 执行信息
	Legs        []Leg   `json:"legs"`
	EstimatedGas float64 `json:"estimated_gas"`
	Slippage    float64 `json:"slippage"`
}

type Leg struct {
	Exchange string  `json:"exchange"`
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
}

func (o *Opportunity) NetProfit() float64 {
	return o.ProfitAmount - o.EstimatedGas - o.Slippage
}
```

- [ ] **Step 2: 创建跨交易所策略 `internal/strategy/cross_exchange.go`**

```go
package strategy

import (
	"context"
	"sync"
)

type CrossExchangeStrategy struct {
	mu          sync.RWMutex
	pairs       map[string][]string // symbol -> [exchange symbols]
	exchanges   map[string]Exchange
	minProfit   float64
	lastPrices  map[string]map[string]float64 // exchange -> symbol -> price
}

func NewCrossExchangeStrategy(exchanges map[string]Exchange, minProfit float64) *CrossExchangeStrategy {
	return &CrossExchangeStrategy{
		exchanges:  exchanges,
		minProfit:  minProfit,
		lastPrices: make(map[string]map[string]float64),
	}
}

func (s *CrossExchangeStrategy) CheckOpportunity(symbol string) *Opportunity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var bestOpp *Opportunity
	
	// 遍历所有交易所对，寻找套利机会
	for exA, priceA := range s.lastPrices[symbol] {
		for exB, priceB := range s.lastPrices[symbol] {
			if exA == exB {
				continue
			}
			
			// 计算价差
			spread := (priceB - priceA) / priceA * 100
			
			if spread > s.minProfit {
				opp := &Opportunity{
					StrategyType: "cross_exchange",
					ExchangeA:    exA,
					ExchangeB:    exB,
					Symbol:       symbol,
					PriceA:       priceA,
					PriceB:       priceB,
					ProfitRate:   spread,
				}
				
				if bestOpp == nil || opp.ProfitRate > bestOpp.ProfitRate {
					bestOpp = opp
				}
			}
		}
	}
	
	return bestOpp
}

func (s *CrossExchangeStrategy) UpdatePrice(exchange, symbol string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, ok := s.lastPrices[exchange]; !ok {
		s.lastPrices[exchange] = make(map[string]float64)
	}
	s.lastPrices[exchange][symbol] = price
}
```

- [ ] **Step 3: 编写测试 `internal/strategy/cross_exchange_test.go`**

```go
package strategy

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCrossExchangeStrategy(t *testing.T) {
	strategy := NewCrossExchangeStrategy(nil, 0.5)
	
	// 更新价格
	strategy.UpdatePrice("binance", "BTC/USDT", 50000)
	strategy.UpdatePrice("okx", "BTC/USDT", 50300)
	
	// 检查机会
	opp := strategy.CheckOpportunity("BTC/USDT")
	
	assert.NotNil(t, opp)
	assert.Equal(t, 0.6, opp.ProfitRate) // (50300-50000)/50000*100 = 0.6%
}
```

- [ ] **Step 4: 运行测试**

```bash
go test -v ./internal/strategy/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "feat: add cross-exchange arbitrage strategy"
```

---

### Task 9: 资金费率套利策略

**Files:**
- Create: `internal/strategy/funding_rate.go`
- Test: `internal/strategy/funding_rate_test.go`

- [ ] **Step 1: 创建资金费率策略 `internal/strategy/funding_rate.go`**

```go
package strategy

type FundingRateStrategy struct {
	exchanges    map[string]Exchange
	minRateDiff  float64
	lastRates    map[string]map[string]float64 // exchange -> symbol -> rate
}

func NewFundingRateStrategy(exchanges map[string]Exchange, minRateDiff float64) *FundingRateStrategy {
	return &FundingRateStrategy{
		exchanges:   exchanges,
		minRateDiff: minRateDiff,
		lastRates:   make(map[string]map[string]float64),
	}
}

func (s *FundingRateStrategy) UpdateRate(exchange, symbol string, rate float64) {
	if _, ok := s.lastRates[exchange]; !ok {
		s.lastRates[exchange] = make(map[string]float64)
	}
	s.lastRates[exchange][symbol] = rate
}

func (s *FundingRateStrategy) CheckOpportunity(symbol string) *Opportunity {
	// 寻找资金费率差异
	// 高费率做空，低费率做多
	var bestOpp *Opportunity
	
	for exA, ratesA := range s.lastRates {
		for exB, ratesB := range s.lastRates {
			if exA == exB {
				continue
			}
			
			rateA := ratesA[symbol]
			rateB := ratesB[symbol]
			
			diff := rateA - rateB
			
			if diff > s.minRateDiff {
				opp := &Opportunity{
					StrategyType: "funding_rate",
					ExchangeA:    exA,
					ExchangeB:    exB,
					Symbol:       symbol,
					ProfitRate:   diff * 100,
					Legs: []Leg{
						{Exchange: exA, Symbol: symbol, Side: "sell"},
						{Exchange: exB, Symbol: symbol, Side: "buy"},
					},
				}
				
				if bestOpp == nil || opp.ProfitRate > bestOpp.ProfitRate {
					bestOpp = opp
				}
			}
		}
	}
	
	return bestOpp
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add funding rate arbitrage strategy"
```

---

### Task 10: 期现套利策略

**Files:**
- Create: `internal/strategy/spot_future.go`
- Test: `internal/strategy/spot_future_test.go`

- [ ] **Step 1: 创建期现套利策略 `internal/strategy/spot_future.go`**

```go
package strategy

type SpotFutureStrategy struct {
	exchange   Exchange
	minBasis   float64
}

func NewSpotFutureStrategy(exchange Exchange, minBasis float64) *SpotFutureStrategy {
	return &SpotFutureStrategy{
		exchange: exchange,
		minBasis: minBasis,
	}
}

func (s *SpotFutureStrategy) CheckOpportunity(symbol string, spotPrice, futurePrice float64) *Opportunity {
	basis := (futurePrice - spotPrice) / spotPrice * 100
	
	if basis > s.minBasis {
		return &Opportunity{
			StrategyType: "spot_future",
			Symbol:       symbol,
			PriceA:       spotPrice,
			PriceB:       futurePrice,
			ProfitRate:   basis,
			Legs: []Leg{
				{Symbol: symbol, Side: "buy"},   // 买入现货
				{Symbol: symbol, Side: "sell"},  // 卖出期货
			},
		}
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add spot-future arbitrage strategy"
```

---

### Task 11: DEX 三角套利策略

**Files:**
- Create: `internal/strategy/triangular.go`
- Create: `internal/strategy/dex_cross_dex.go`
- Test: `internal/strategy/triangular_test.go`

- [ ] **Step 1: 创建三角套利策略 `internal/strategy/triangular.go`**

```go
package strategy

type TriangularStrategy struct {
	dex        DEX
	minProfit  float64
	pairs      []TriPair
}

type TriPair struct {
	Base  string
	Quote string
}

func NewTriangularStrategy(dex DEX, minProfit float64, pairs []TriPair) *TriangularStrategy {
	return &TriangularStrategy{
		dex:       dex,
		minProfit: minProfit,
		pairs:     pairs,
	}
}

func (s *TriangularStrategy) CheckOpportunity(path []string) *Opportunity {
	// path: USDT -> BTC -> ETH -> USDT
	prices, err := s.dex.GetPrices(path)
	if err != nil {
		return nil
	}
	
	// 计算最终金额
	startAmount := 1000.0 // 假设 1000 USDT 起始
	current := startAmount
	
	for i := 0; i < len(path)-1; i++ {
		price := prices[i]
		current = current / price
	}
	
	profitRate := (current - startAmount) / startAmount * 100
	
	if profitRate > s.minProfit {
		return &Opportunity{
			StrategyType: "triangular",
			ProfitRate:   profitRate,
			ProfitAmount: current - startAmount,
		}
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add triangular arbitrage strategy"
```

---

## Phase 4: 交易核心模块

### Task 12: 订单管理器

**Files:**
- Create: `internal/service/order_manager.go`
- Test: `internal/service/order_manager_test.go`

- [ ] **Step 1: 创建订单管理器 `internal/service/order_manager.go`**

```go
package service

import (
	"context"
	"github.com/ai-money/arbitrage/internal/database"
	"github.com/ai-money/arbitrage/internal/model"
	"github.com/ai-money/arbitrage/internal/exchange"
)

type OrderManager struct {
	exchanges map[string]exchange.Exchange
}

func NewOrderManager(exchanges map[string]exchange.Exchange) *OrderManager {
	return &OrderManager{
		exchanges: exchanges,
	}
}

func (m *OrderManager) ExecuteOrder(ctx context.Context, leg *Leg) (*model.Order, error) {
	ex, ok := m.exchanges[leg.Exchange]
	if !ok {
		return nil, fmt.Errorf("unknown exchange: %s", leg.Exchange)
	}
	
	order, err := ex.PlaceOrder(ctx, leg.Symbol, leg.Side, "market", leg.Quantity, 0)
	if err != nil {
		return nil, err
	}
	
	// 保存到数据库
	dbOrder := &model.Order{
		Exchange: leg.Exchange,
		Symbol:   leg.Symbol,
		Side:     leg.Side,
		Type:     "market",
		Price:    order.Price,
		Quantity: order.Quantity,
		OrderID:  order.OrderID,
		Status:   order.Status,
	}
	
	return dbOrder, database.DB.Create(dbOrder).Error
}

func (m *OrderManager) ExecuteArbitrage(ctx context.Context, opp *Opportunity) error {
	// 并发执行所有腿
	var wg sync.WaitGroup
	errChan := make(chan error, len(opp.Legs))
	
	for _, leg := range opp.Legs {
		wg.Add(1)
		go func(l Leg) {
			defer wg.Done()
			_, err := m.ExecuteOrder(ctx, &l)
			if err != nil {
				errChan <- err
			}
		}(leg)
	}
	
	wg.Wait()
	close(errChan)
	
	// 检查是否有错误
	if err := <-errChan; err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add order manager"
```

---

### Task 13: 风险管理器

**Files:**
- Create: `internal/service/risk_manager.go`
- Test: `internal/service/risk_manager_test.go`

- [ ] **Step 1: 创建风险管理器 `internal/service/risk_manager.go`**

```go
package service

import (
	"sync"
	"github.com/ai-money/arbitrage/internal/model"
)

type RiskManager struct {
	mu              sync.RWMutex
	maxPosition     float64
	dailyStopLoss   float64
	strategyLimits  map[string]StrategyLimit
	dailyPnL        float64
	currentPositions map[string]*model.Position
}

type StrategyLimit struct {
	MaxPosition   float64
	MaxDailyLoss  float64
	AutoExecute   bool
}

func NewRiskManager(maxPosition, dailyStopLoss float64) *RiskManager {
	return &RiskManager{
		maxPosition:      maxPosition,
		dailyStopLoss:    dailyStopLoss,
		strategyLimits:   make(map[string]StrategyLimit),
		currentPositions: make(map[string]*model.Position),
	}
}

func (m *RiskManager) CanExecute(opp *Opportunity) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 检查是否超过日亏损限制
	if m.dailyPnL < -m.dailyStopLoss {
		return false
	}
	
	// 检查策略是否允许自动执行
	limit := m.strategyLimits[opp.StrategyType]
	if !limit.AutoExecute {
		return false
	}
	
	// 检查仓位限制
	// ...
	
	return true
}

func (m *RiskManager) UpdatePosition(exchange, symbol string, position *model.Position) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := exchange + ":" + symbol
	m.currentPositions[key] = position
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add risk manager"
```

---

### Task 14: 告警引擎

**Files:**
- Create: `internal/service/alert_engine.go`
- Create: `internal/service/notify/telegram.go`
- Create: `internal/service/notify/slack.go`
- Create: `internal/service/notify/email.go`

- [ ] **Step 1: 创建告警引擎 `internal/service/alert_engine.go`**

```go
package service

import (
	"github.com/ai-money/arbitrage/internal/model"
	"github.com/ai-money/arbitrage/internal/database"
)

type AlertEngine struct {
	configs   []model.AlertConfig
	notifiers map[string]Notifier
}

type Notifier interface {
	Send(title, message string) error
}

func NewAlertEngine() *AlertEngine {
	return &AlertEngine{
		notifiers: make(map[string]Notifier),
	}
}

func (e *AlertEngine) RegisterNotifier(channel string, n Notifier) {
	e.notifiers[channel] = n
}

func (e *AlertEngine) SendAlert(alertType, level, title, message string) error {
	// 保存到数据库
	dbAlert := &model.Alert{
		Type:    alertType,
		Level:   level,
		Title:   title,
		Message: message,
	}
	database.DB.Create(dbAlert)
	
	// 发送到各个渠道
	for _, cfg := range e.configs {
		if !cfg.IsEnabled {
			continue
		}
		
		notifier, ok := e.notifiers[cfg.Channel]
		if ok {
			notifier.Send(title, message)
		}
	}
	
	return nil
}
```

- [ ] **Step 2: 创建 Telegram 通知 `internal/service/notify/telegram.go`**

```go
package notify

import (
	"fmt"
	"net/http"
	"net/url"
)

type TelegramNotifier struct {
	BotToken string
	ChatID   string
}

func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
	}
}

func (n *TelegramNotifier) Send(title, message string) error {
	text := fmt.Sprintf("*%s*\n\n%s", title, message)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.BotToken)
	
	data := url.Values{}
	data.Set("chat_id", n.ChatID)
	data.Set("text", text)
	data.Set("parse_mode", "Markdown")
	
	resp, err := http.PostForm(apiURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add .
git commit -m "feat: add alert engine and notifiers"
```

---

### Task 15: 动态配置管理

**Files:**
- Create: `internal/service/config_manager.go`
- Create: `internal/api/config.go`

- [ ] **Step 1: 创建配置管理器 `internal/service/config_manager.go`**

```go
package service

import (
	"encoding/json"
	"sync"
	"github.com/ai-money/arbitrage/internal/model"
	"github.com/ai-money/arbitrage/internal/database"
)

type ConfigManager struct {
	mu         sync.RWMutex
	strategies map[string]*model.Strategy
}

func NewConfigManager() *ConfigManager {
	cm := &ConfigManager{
		strategies: make(map[string]*model.Strategy),
	}
	cm.loadFromDB()
	return cm
}

func (m *ConfigManager) loadFromDB() {
	var strategies []model.Strategy
	database.DB.Find(&strategies)
	for _, s := range strategies {
		m.strategies[s.Name] = &s
	}
}

func (m *ConfigManager) GetStrategy(name string) *model.Strategy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.strategies[name]
}

func (m *ConfigManager) UpdateStrategy(s *model.Strategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.strategies[s.Name] = s
	return database.DB.Save(s).Error
}

func (m *ConfigManager) SetStrategyEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if s, ok := m.strategies[name]; ok {
		s.IsEnabled = enabled
		return database.DB.Save(s).Error
	}
	return fmt.Errorf("strategy not found: %s", name)
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add dynamic config manager"
```

---

## Phase 5: API 和前端

### Task 16: HTTP API

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/handlers/ticker.go`
- Create: `internal/api/handlers/strategy.go`
- Create: `internal/api/handlers/order.go`
- Create: `internal/api/handlers/position.go`
- Create: `internal/api/handlers/alert.go`

- [ ] **Step 1: 创建 API 服务器 `internal/api/server.go`**

```go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ai-money/arbitrage/internal/api/handlers"
)

type Server struct {
	router *gin.Engine
}

func NewServer() *Server {
	gin.SetMode(gin.DebugMode)
	router := gin.Default()
	return &Server{router: router}
}

func (s *Server) SetupRoutes() {
	api := s.router.Group("/api/v1")
	{
		api.GET("/tickers", handlers.GetTickers)
		api.GET("/tickers/:symbol", handlers.GetTicker)
		
		api.GET("/strategies", handlers.ListStrategies)
		api.PUT("/strategies/:name", handlers.UpdateStrategy)
		api.POST("/strategies/:name/enable", handlers.EnableStrategy)
		api.POST("/strategies/:name/disable", handlers.DisableStrategy)
		
		api.GET("/orders", handlers.ListOrders)
		api.POST("/orders", handlers.PlaceOrder)
		api.DELETE("/orders/:id", handlers.CancelOrder)
		
		api.GET("/positions", handlers.GetPositions)
		
		api.GET("/alerts", handlers.ListAlerts)
		api.PUT("/alerts/:id/read", handlers.MarkAlertRead)
	}
}

func (s *Server) Run(port string) error {
	return s.router.Run(":" + port)
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add HTTP API server"
```

---

### Task 17: WebSocket 实时推送

**Files:**
- Create: `internal/api/websocket/hub.go`
- Create: `internal/api/websocket/client.go`
- Create: `internal/api/websocket/handler.go`

- [ ] **Step 1: 创建 WebSocket Hub `internal/api/websocket/hub.go`**

```go
package websocket

import (
	"sync"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 100),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}
```

- [ ] **Step 2: Commit**

```bash
git add .
git commit -m "feat: add WebSocket support for real-time updates"
```

---

### Task 18: 前端项目初始化

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/src/main.ts`
- Create: `web/src/App.vue`
- Create: `web/index.html`

- [ ] **Step 1: 初始化 Vue 3 项目**

```bash
cd web
npm init vue@latest
# 选择：TypeScript, Vue Router, Pinia, ESLint, Prettier
```

- [ ] **Step 2: 安装依赖**

```bash
npm install
npm install pinia axios @element-plus/icons-vue echarts vue-echarts
```

- [ ] **Step 3: Commit**

```bash
git add web/
git commit -m "feat: initialize Vue 3 frontend project"
```

---

### Task 19: 前端页面 - 行情看板

**Files:**
- Create: `web/src/views/Dashboard.vue`
- Create: `web/src/components/TickerTable.vue`
- Create: `web/src/components/PriceChart.vue`
- Create: `web/src/stores/ticker.ts`

- [ ] **Step 1: 创建行情 Store `web/src/stores/ticker.ts`**

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import axios from 'axios'

export interface Ticker {
  exchange: string
  symbol: string
  price: number
  bid: number
  ask: number
  volume24h: number
  change24h: number
  timestamp: number
}

export const useTickerStore = defineStore('ticker', () => {
  const tickers = ref<Ticker[]>([])
  const loading = ref(false)

  async function fetchTickers() {
    loading.value = true
    try {
      const resp = await axios.get('/api/v1/tickers')
      tickers.value = resp.data
    } finally {
      loading.value = false
    }
  }

  return { tickers, loading, fetchTickers }
})
```

- [ ] **Step 2: 创建行情看板组件 `web/src/views/Dashboard.vue`**

```vue
<template>
  <div class="dashboard">
    <el-card title="实时行情">
      <TickerTable :tickers="tickerStore.tickers" />
    </el-card>
    
    <el-row :gutter="16">
      <el-col :span="12">
        <el-card title="价格走势图">
          <PriceChart symbol="BTC/USDT" />
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card title="套利机会">
          <OpportunityList />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useTickerStore } from '@/stores/ticker'
import TickerTable from '@/components/TickerTable.vue'
import PriceChart from '@/components/PriceChart.vue'

const tickerStore = useTickerStore()

onMounted(() => {
  tickerStore.fetchTickers()
})
</script>
```

- [ ] **Step 3: Commit**

```bash
git add web/
git commit -m "feat: add dashboard page with ticker display"
```

---

### Task 20: 前端页面 - 策略配置

**Files:**
- Create: `web/src/views/Strategies.vue`
- Create: `web/src/components/StrategyCard.vue`
- Create: `web/src/stores/strategy.ts`

- [ ] **Step 1: 创建策略 Store `web/src/stores/strategy.ts`**

```typescript
import { defineStore } from 'pinia'
import axios from 'axios'

export interface Strategy {
  id: number
  name: string
  isEnabled: boolean
  autoExecute: boolean
  minProfitRate: number
  maxPosition: number
  stopLossRate: number
  config: any
}

export const useStrategyStore = defineStore('strategy', () => {
  const strategies = ref<Strategy[]>([])

  async function fetchStrategies() {
    const resp = await axios.get('/api/v1/strategies')
    strategies.value = resp.data
  }

  async function updateStrategy(strategy: Strategy) {
    await axios.put(`/api/v1/strategies/${strategy.name}`, strategy)
  }

  async function toggleEnabled(name: string, enabled: boolean) {
    const action = enabled ? 'enable' : 'disable'
    await axios.post(`/api/v1/strategies/${name}/${action}`)
  }

  return { strategies, fetchStrategies, updateStrategy, toggleEnabled }
})
```

- [ ] **Step 2: 创建策略配置页面 `web/src/views/Strategies.vue`**

```vue
<template>
  <div class="strategies">
    <el-row :gutter="16">
      <el-col :span="8" v-for="s in strategyStore.strategies" :key="s.id">
        <StrategyCard :strategy="s" />
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useStrategyStore } from '@/stores/strategy'
import StrategyCard from '@/components/StrategyCard.vue'

const strategyStore = useStrategyStore()

onMounted(() => {
  strategyStore.fetchStrategies()
})
</script>
```

- [ ] **Step 3: Commit**

```bash
git add web/
git add .
git commit -m "feat: add strategies configuration page"
```

---

### Task 21: 前端页面 - 仓位管理和告警

**Files:**
- Create: `web/src/views/Positions.vue`
- Create: `web/src/views/Alerts.vue`
- Create: `web/src/stores/position.ts`
- Create: `web/src/stores/alert.ts`

- [ ] **Step 1: 创建仓位和告警 Store**

类似前面的模式

- [ ] **Step 2: 创建页面组件**

- [ ] **Step 3: Commit**

```bash
git add web/
git commit -m "feat: add positions and alerts pages"
```

---

## Phase 6: 测试与部署

### Task 22: 集成测试

**Files:**
- Create: `tests/integration/exchange_test.go`
- Create: `tests/integration/strategy_test.go`
- Create: `tests/integration/api_test.go`

- [ ] **Step 1: 创建交易所集成测试**

- [ ] **Step 2: 创建策略集成测试**

- [ ] **Step 3: 创建 API 集成测试**

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "test: add integration tests"
```

---

### Task 23: Docker 镜像和部署文档

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Create: `docs/deployment.md`
- Create: `docs/configuration.md`

- [ ] **Step 1: 完善 Dockerfile 支持多阶段构建**

- [ ] **Step 2: 完善 docker-compose.yml**

- [ ] **Step 3: 编写部署文档 `docs/deployment.md`**

```markdown
# 部署指南

## 环境要求

- Docker 20+
- Docker Compose 2+

## 快速开始

1. 克隆项目
```bash
git clone https://github.com/ai-money/arbitrage.git
cd arbitrage
```

2. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 文件，填入数据库密码等配置
```

3. 启动服务
```bash
docker-compose up -d
```

4. 查看日志
```bash
docker-compose logs -f app
```

## 生产环境部署

...
```

- [ ] **Step 4: 编写配置文档 `docs/configuration.md`**

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "docs: add deployment and configuration guides"
```

---

## 总结

本计划共计 23 个任务，涵盖：

- **Phase 1 (Task 1-5):** 项目骨架、数据库模型、CEX 模块
- **Phase 2 (Task 6-7):** DEX 模块 (EVM + Solana)
- **Phase 3 (Task 8-11):** 策略引擎 (5 种策略)
- **Phase 4 (Task 12-15):** 交易核心模块 (订单、风控、告警、配置)
- **Phase 5 (Task 16-21):** API 和前端
- **Phase 6 (Task 22-23):** 测试与部署

计划完成后可运行：

```bash
docker-compose up -d
```

访问 `http://localhost:8080` 使用前端界面。
