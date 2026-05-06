package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/model"
)

// Level 告警级别
type Level string

const (
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
	LevelCritical Level = "critical"
)

// Type 告警类型
type Type string

const (
	TypePrice       Type = "price"
	TypeOpportunity Type = "opportunity"
	TypeOrder       Type = "order"
	TypeSystem      Type = "system"
	TypeRisk        Type = "risk"
)

// Engine 告警引擎
type Engine struct {
	mu        sync.RWMutex
	configs   []model.AlertConfig
	notifiers map[string]Notifier
	queue     chan *Alert
}

// Alert 告警
type Alert struct {
	Type      Type    `json:"type"`
	Level     Level   `json:"level"`
	Title     string  `json:"title"`
	Message   string  `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

// Notifier 通知器接口
type Notifier interface {
	Send(ctx context.Context, alert *Alert) error
}

// NewEngine 创建告警引擎
func NewEngine() *Engine {
	e := &Engine{
		notifiers: make(map[string]Notifier),
		queue:     make(chan *Alert, 100),
	}
	e.loadConfigs()
	go e.processQueue()
	return e
}

// loadConfigs 加载告警配置
func (e *Engine) loadConfigs() {
	e.mu.Lock()
	defer e.mu.Unlock()

	var configs []model.AlertConfig
	database.DB.Where("is_enabled = ?", true).Find(&configs)
	e.configs = configs
}

// RegisterNotifier 注册通知器
func (e *Engine) RegisterNotifier(channel string, n Notifier) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.notifiers[channel] = n
}

// Send 发送告警
func (e *Engine) Send(alertType Type, level Level, title, message string, data ...interface{}) {
	alert := &Alert{
		Type:      alertType,
		Level:     level,
		Title:     title,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}

	if len(data) > 0 {
		alert.Data = data[0]
	}

	// 存入队列
	select {
	case e.queue <- alert:
	default:
		// 队列已满，丢弃
	}

	// 同步保存到数据库
	e.saveToDB(alert)
}

// SendPriceAlert 发送价格告警
func (e *Engine) SendPriceAlert(symbol, exchange string, price float64, changePercent float64) {
	e.Send(
		TypePrice,
		LevelWarning,
		fmt.Sprintf("%s 价格异动", symbol),
		fmt.Sprintf("%s %s 当前价格：%.4f, 24h 涨跌幅：%.2f%%", exchange, symbol, price, changePercent),
		map[string]interface{}{
			"symbol":   symbol,
			"exchange": exchange,
			"price":    price,
			"change":   changePercent,
		},
	)
}

// SendOpportunityAlert 发送套利机会告警
func (e *Engine) SendOpportunityAlert(strategyType string, profitRate float64, details map[string]interface{}) {
	e.Send(
		TypeOpportunity,
		LevelInfo,
		fmt.Sprintf("发现套利机会：%s", strategyType),
		fmt.Sprintf("策略：%s, 预估利润率：%.2f%%", strategyType, profitRate),
		details,
	)
}

// SendOrderAlert 发送订单告警
func (e *Engine) SendOrderAlert(event string, orderID, symbol, side string, err error) {
	level := LevelInfo
	if err != nil {
		level = LevelError
	}

	e.Send(
		TypeOrder,
		level,
		fmt.Sprintf("订单%s", event),
		fmt.Sprintf("订单 ID: %s, 交易对：%s, 方向：%s, 错误：%v", orderID, symbol, side, err),
		map[string]interface{}{
			"order_id": orderID,
			"symbol":   symbol,
			"side":     side,
			"error":    err,
		},
	)
}

// SendSystemAlert 发送系统告警
func (e *Engine) SendSystemAlert(component, message string, level Level) {
	e.Send(
		TypeSystem,
		level,
		fmt.Sprintf("系统告警：%s", component),
		message,
		map[string]interface{}{
			"component": component,
		},
	)
}

// SendRiskAlert 发送风控告警
func (e *Engine) SendRiskAlert(reason string, details map[string]interface{}) {
	e.Send(
		TypeRisk,
		LevelWarning,
		"风控触发",
		fmt.Sprintf("原因：%s", reason),
		details,
	)
}

// saveToDB 保存到数据库
func (e *Engine) saveToDB(alert *Alert) {
	dbAlert := &model.Alert{
		Type:    string(alert.Type),
		Level:   string(alert.Level),
		Title:   alert.Title,
		Message: alert.Message,
	}
	database.DB.Create(dbAlert)
}

// processQueue 处理告警队列
func (e *Engine) processQueue() {
	for alert := range e.queue {
		e.sendNotifications(alert)
	}
}

// sendNotifications 发送通知到各个渠道
func (e *Engine) sendNotifications(alert *Alert) {
	e.mu.RLock()
	configs := make([]model.AlertConfig, len(e.configs))
	copy(configs, e.configs)
	notifiers := make(map[string]Notifier)
	for k, v := range e.notifiers {
		notifiers[k] = v
	}
	e.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, cfg := range configs {
		notifier, ok := notifiers[cfg.Channel]
		if !ok {
			continue
		}

		// 异步发送，不阻塞
		go func(cfg model.AlertConfig) {
			if err := notifier.Send(ctx, alert); err != nil {
				// 记录发送失败
				fmt.Printf("Failed to send alert to %s: %v\n", cfg.Channel, err)
			}
		}(cfg)
	}
}

// ReloadConfigs 重新加载配置
func (e *Engine) ReloadConfigs() {
	e.loadConfigs()
}

// GetUnreadAlerts 获取未读告警
func (e *Engine) GetUnreadAlerts(limit int) ([]model.Alert, error) {
	var alerts []model.Alert
	err := database.DB.Where("is_read = ?", false).
		Order("created_at DESC").
		Limit(limit).
		Find(&alerts).Error
	return alerts, err
}

// MarkAsRead 标记告警为已读
func (e *Engine) MarkAsRead(ids []uint) error {
	return database.DB.Model(&model.Alert{}).
		Where("id IN ?", ids).
		Update("is_read", true).Error
}

// ClearOldAlerts 清理旧告警（保留最近 7 天）
func (e *Engine) ClearOldAlerts() error {
	cutoff := time.Now().AddDate(0, 0, -7)
	return database.DB.Where("created_at < ?", cutoff).Delete(&model.Alert{}).Error
}

// GetAlertStats 获取告警统计
type Stats struct {
	TotalAlerts   int64 `json:"total_alerts"`
	UnreadAlerts  int64 `json:"unread_alerts"`
	TodayAlerts   int64 `json:"today_alerts"`
	ByType        map[string]int64 `json:"by_type"`
	ByLevel       map[string]int64 `json:"by_level"`
}

func (e *Engine) GetAlertStats() (*Stats, error) {
	var total, unread, today int64
	byType := make(map[string]int64)
	byLevel := make(map[string]int64)

	database.DB.Model(&model.Alert{}).Count(&total)
	database.DB.Model(&model.Alert{}).Where("is_read = ?", false).Count(&unread)

	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&model.Alert{}).Where("created_at >= ?", todayStart).Count(&today)

	// 按类型统计
	rows, err := database.DB.Model(&model.Alert{}).Select("type, COUNT(*) as count").Group("type").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t string
		var count int64
		rows.Scan(&t, &count)
		byType[t] = count
	}

	// 按级别统计
	rows2, err := database.DB.Model(&model.Alert{}).Select("level, COUNT(*) as count").Group("level").Rows()
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var l string
		var count int64
		rows2.Scan(&l, &count)
		byLevel[l] = count
	}

	return &Stats{
		TotalAlerts:  total,
		UnreadAlerts: unread,
		TodayAlerts:  today,
		ByType:       byType,
		ByLevel:      byLevel,
	}, nil
}

// TelegramNotifier Telegram 通知器
type TelegramNotifier struct {
	BotToken string
	ChatID   string
}

// NewTelegramNotifier 创建 Telegram 通知器
func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
	}
}

// Send 发送 Telegram 消息
func (n *TelegramNotifier) Send(ctx context.Context, alert *Alert) error {
	text := fmt.Sprintf("*%s*\n\n%s", alert.Title, alert.Message)
	if alert.Data != nil {
		dataBytes, _ := json.Marshal(alert.Data)
		text += fmt.Sprintf("\n\n`%s`", string(dataBytes))
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.BotToken)
	payload := map[string]string{
		"chat_id":    n.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// SlackNotifier Slack 通知器
type SlackNotifier struct {
	WebhookURL string
}

// NewSlackNotifier 创建 Slack 通知器
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL: webhookURL,
	}
}

// Send 发送 Slack 消息
func (n *SlackNotifier) Send(ctx context.Context, alert *Alert) error {
	color := "good"
	switch alert.Level {
	case LevelWarning:
		color = "warning"
	case LevelError, LevelCritical:
		color = "danger"
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": alert.Title,
				"text":  alert.Message,
				"ts":    alert.Timestamp / 1000,
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", n.WebhookURL, bytes.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// WebhookNotifier 通用 Webhook 通知器
type WebhookNotifier struct {
	URL string
}

// NewWebhookNotifier 创建 Webhook 通知器
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{URL: url}
}

// Send 发送 Webhook 通知
func (n *WebhookNotifier) Send(ctx context.Context, alert *Alert) error {
	jsonPayload, _ := json.Marshal(alert)
	req, _ := http.NewRequestWithContext(ctx, "POST", n.URL, bytes.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
