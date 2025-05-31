package utils

import (
	"sync"
	"time"
)

// MarketData 表示市场数据
type MarketData struct {
	Symbol    string    // 交易对符号
	Price     float64   // 最新价格
	UpdatedAt time.Time // 更新时间
}

// MarketDataManager 市场数据管理器
type MarketDataManager struct {
	data map[string]*MarketData // 按symbol存储最新数据
	mu   sync.RWMutex
}

// NewMarketDataManager 创建新的市场数据管理器
func NewMarketDataManager() *MarketDataManager {
	return &MarketDataManager{
		data: make(map[string]*MarketData),
	}
}

// UpdatePrice 更新价格数据
func (m *MarketDataManager) UpdatePrice(symbol string, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[symbol] = &MarketData{
		Symbol:    symbol,
		Price:     price,
		UpdatedAt: time.Now(),
	}
}

// GetPrice 获取指定交易对的最新价格数据
func (m *MarketDataManager) GetPrice(symbol string) *MarketData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.data[symbol]
}

// GetAllPrices 获取所有交易对的最新价格数据
func (m *MarketDataManager) GetAllPrices() map[string]*MarketData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建副本以避免并发问题
	result := make(map[string]*MarketData, len(m.data))
	for symbol, data := range m.data {
		result[symbol] = &MarketData{
			Symbol:    data.Symbol,
			Price:     data.Price,
			UpdatedAt: data.UpdatedAt,
		}
	}

	return result
}
