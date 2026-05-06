package config

import (
	"encoding/json"
	"sync"

	"github.com/joker782311/cryptoArbitrage/internal/database"
	"github.com/joker782311/cryptoArbitrage/internal/model"
)

// Manager 配置管理器
type Manager struct {
	mu         sync.RWMutex
	strategies map[string]*model.Strategy
	apiKeys    map[string]*model.APIKey
}

// NewManager 创建配置管理器
func NewManager() *Manager {
	m := &Manager{
		strategies: make(map[string]*model.Strategy),
		apiKeys:    make(map[string]*model.APIKey),
	}
	m.loadFromDB()
	return m
}

// loadFromDB 从数据库加载配置
func (m *Manager) loadFromDB() {
	// 加载策略配置
	var strategies []model.Strategy
	database.DB.Find(&strategies)
	for _, s := range strategies {
		m.strategies[s.Name] = &s
	}

	// 加载 API Key
	var apiKeys []model.APIKey
	database.DB.Where("is_enabled = ?", true).Find(&apiKeys)
	for _, k := range apiKeys {
		m.apiKeys[k.Exchange] = k
	}
}

// GetStrategy 获取策略配置
func (m *Manager) GetStrategy(name string) *model.Strategy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.strategies[name]
}

// GetAllStrategies 获取所有策略配置
func (m *Manager) GetAllStrategies() []*model.Strategy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.Strategy, 0, len(m.strategies))
	for _, s := range m.strategies {
		result = append(result, s)
	}
	return result
}

// UpdateStrategy 更新策略配置
func (m *Manager) UpdateStrategy(s *model.Strategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.strategies[s.Name] = s
	return database.DB.Save(s).Error
}

// SetStrategyEnabled 设置策略启用状态
func (m *Manager) SetStrategyEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.strategies[name]; ok {
		s.IsEnabled = enabled
		database.DB.Model(s).Update("is_enabled", enabled)
		return nil
	}
	return nil
}

// SetStrategyAutoExecute 设置策略自动执行
func (m *Manager) SetStrategyAutoExecute(name string, auto bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.strategies[name]; ok {
		s.AutoExecute = auto
		database.DB.Model(s).Update("auto_execute", auto)
		return nil
	}
	return nil
}

// SetStrategyParam 设置策略参数
func (m *Manager) SetStrategyParam(name, key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.strategies[name]
	if !ok {
		return nil
	}

	// 解析配置
	var config map[string]interface{}
	json.Unmarshal([]byte(s.Config), &config)
	config[key] = value

	// 序列化回 JSON
	configBytes, _ := json.Marshal(config)
	s.Config = string(configBytes)

	database.DB.Model(s).Update("config", s.Config)
	return nil
}

// GetAPIKey 获取交易所 API Key
func (m *Manager) GetAPIKey(exchange string) (*model.APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.apiKeys[exchange]
	if !ok {
		return nil, nil
	}
	return key, nil
}

// GetAllAPIKeys 获取所有 API Key（脱敏）
func (m *Manager) GetAllAPIKeys() []*model.APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.APIKey, 0, len(m.apiKeys))
	for _, k := range m.apiKeys {
		// 脱敏处理
		safeKey := *k
		if len(safeKey.APIKey) > 8 {
			safeKey.APIKey = safeKey.APIKey[:4] + "****" + safeKey.APIKey[len(safeKey.APIKey)-4:]
		}
		result = append(result, &safeKey)
	}
	return result
}

// SaveAPIKey 保存 API Key
func (m *Manager) SaveAPIKey(key *model.APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO: 加密存储
	if err := database.DB.Save(key).Error; err != nil {
		return err
	}

	m.apiKeys[key.Exchange] = key
	return nil
}

// DeleteAPIKey 删除 API Key
func (m *Manager) DeleteAPIKey(exchange string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := database.DB.Where("exchange = ?", exchange).Delete(&model.APIKey{}).Error; err != nil {
		return err
	}

	delete(m.apiKeys, exchange)
	return nil
}

// SetAPIKeyEnabled 设置 API Key 启用状态
func (m *Manager) SetAPIKeyEnabled(exchange string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := database.DB.Model(&model.APIKey{}).
		Where("exchange = ?", exchange).
		Update("is_enabled", enabled).Error; err != nil {
		return err
	}

	if key, ok := m.apiKeys[exchange]; ok {
		key.IsEnabled = enabled
		if !enabled {
			delete(m.apiKeys, exchange)
		}
	}
	return nil
}

// GetAlertConfigs 获取告警配置
func (m *Manager) GetAlertConfigs() ([]model.AlertConfig, error) {
	var configs []model.AlertConfig
	err := database.DB.Find(&configs).Error
	return configs, err
}

// SaveAlertConfig 保存告警配置
func (m *Manager) SaveAlertConfig(cfg *model.AlertConfig) error {
	return database.DB.Save(cfg).Error
}

// DeleteAlertConfig 删除告警配置
func (m *Manager) DeleteAlertConfig(id uint) error {
	return database.DB.Delete(&model.AlertConfig{}, id).Error
}

// Reload 重新加载所有配置
func (m *Manager) Reload() {
	m.loadFromDB()
}
