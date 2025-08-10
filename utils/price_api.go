package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// PriceAPI API服务器，提供价格数据接口
type PriceAPI struct {
	Port        int
	dataManager *MarketDataManager
	mu          sync.RWMutex
}

// NewPriceAPI 创建新的API服务器
func NewPriceAPI(port int, dataManager *MarketDataManager) *PriceAPI {
	return &PriceAPI{
		Port:        port,
		dataManager: dataManager,
	}
}

// Start 启动API服务器
func (api *PriceAPI) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/prices", api.handleGetAllPrices)
	mux.HandleFunc("/api/price/btc", api.handleGetBTCPrice)
	mux.HandleFunc("/api/price/eth", api.handleGetETHPrice)

	addr := fmt.Sprintf(":%d", api.Port)
	log.Printf("API服务器启动在 http://localhost%s", addr)

	// 加上 CORS 中间件
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

// handleGetAllPrices 处理获取所有价格的请求
func (api *PriceAPI) handleGetAllPrices(w http.ResponseWriter, r *http.Request) {
	prices := api.dataManager.GetAllPrices()

	// 准备响应数据
	response := make(map[string]interface{})

	for symbol, data := range prices {
		response[symbol] = map[string]interface{}{
			"symbol":     symbol,
			"price":      data.Price,
			"updated_at": data.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetBTCPrice 处理获取BTC价格的请求
func (api *PriceAPI) handleGetBTCPrice(w http.ResponseWriter, r *http.Request) {
	data := api.dataManager.GetPrice("BTCUSDT")

	if data != nil {
		// 根据请求格式返回不同的响应
		if r.URL.Query().Get("format") == "text" {
			// 纯文本格式，适合Rainmeter
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "BTC Price: %.2f", data.Price)
		} else {
			// JSON格式
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbol":     "BTC",
				"price":      data.Price,
				"updated_at": data.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	} else {
		// 数据不可用
		if r.URL.Query().Get("format") == "text" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "BTC Price: unknown")
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "BTC price data not available",
			})
		}
	}
}

// handleGetETHPrice 处理获取ETH价格的请求
func (api *PriceAPI) handleGetETHPrice(w http.ResponseWriter, r *http.Request) {
	data := api.dataManager.GetPrice("ETHUSDT")

	if data != nil {
		// 根据请求格式返回不同的响应
		if r.URL.Query().Get("format") == "text" {
			// 纯文本格式，适合Rainmeter
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "ETH Price: %.2f", data.Price)
		} else {
			// JSON格式
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbol":     "ETH",
				"price":      data.Price,
				"updated_at": data.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	} else {
		// 数据不可用
		if r.URL.Query().Get("format") == "text" {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "ETH Price: unknown")
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "ETH price data not available",
			})
		}
	}
}

// corsMiddleware 处理跨域请求
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // 允许所有域访问
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// 处理预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
