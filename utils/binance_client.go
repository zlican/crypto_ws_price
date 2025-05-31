package utils

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// BinanceTickerMessage 定义了从Binance WebSocket接收到的Ticker消息的结构
type BinanceTickerMessage struct {
	EventType string `json:"e"` // 事件类型
	EventTime int64  `json:"E"` // 事件时间
	Symbol    string `json:"s"` // 交易对
	Kline     struct {
		StartTime     int64  `json:"t"` // K线开始时间
		CloseTime     int64  `json:"T"` // K线结束时间
		Symbol        string `json:"s"` // 交易对
		Interval      string `json:"i"` // K线间隔
		FirstTradeID  int64  `json:"f"` // 起始交易ID
		LastTradeID   int64  `json:"L"` // 结束交易ID
		OpenPrice     string `json:"o"` // 开盘价
		ClosePrice    string `json:"c"` // 收盘价
		HighPrice     string `json:"h"` // 最高价
		LowPrice      string `json:"l"` // 最低价
		Volume        string `json:"v"` // 成交量
		TradeCount    int64  `json:"n"` // 成交笔数
		IsFinal       bool   `json:"x"` // 是否为最终K线
		QuoteVolume   string `json:"q"` // 成交额
		TakerBuyBase  string `json:"V"` // 主动买入成交量
		TakerBuyQuote string `json:"Q"` // 主动买入成交额
	} `json:"k"`
}

// BinanceClient 币安WebSocket客户端
type BinanceClient struct {
	dataManager   *MarketDataManager
	proxyURL      string
	symbols       []string
	interval      time.Duration
	stopChan      chan struct{}
	reconnectLock sync.Mutex
	connections   map[string]*websocket.Conn
	connStartTime map[string]time.Time
}

// NewBinanceClient 创建新的币安WebSocket客户端
func NewBinanceClient(dataManager *MarketDataManager, proxyURL string, symbols []string, intervalSeconds int) *BinanceClient {
	return &BinanceClient{
		dataManager:   dataManager,
		proxyURL:      proxyURL,
		symbols:       symbols,
		interval:      time.Duration(intervalSeconds) * time.Second,
		stopChan:      make(chan struct{}),
		connections:   make(map[string]*websocket.Conn),
		connStartTime: make(map[string]time.Time),
	}
}

// Start 启动WebSocket客户端
func (c *BinanceClient) Start() {
	for _, symbol := range c.symbols {
		go c.startTickerStream(symbol)
	}

	// 启动一个协程定期检查连接状态
	go c.monitorConnections()
}

// Stop 停止WebSocket客户端
func (c *BinanceClient) Stop() {
	close(c.stopChan)

	// 关闭所有连接
	c.reconnectLock.Lock()
	defer c.reconnectLock.Unlock()

	for symbol, conn := range c.connections {
		if conn != nil {
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			_ = conn.Close()
			delete(c.connections, symbol)
		}
	}
}

// monitorConnections 监控所有连接，确保它们保持活跃
func (c *BinanceClient) monitorConnections() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.checkAndRenewConnections()
		}
	}
}

// checkAndRenewConnections 检查并更新连接
func (c *BinanceClient) checkAndRenewConnections() {
	c.reconnectLock.Lock()
	defer c.reconnectLock.Unlock()

	now := time.Now()

	// 检查每个连接的时间，如果接近24小时，则重新连接
	for symbol, startTime := range c.connStartTime {
		// 如果连接已经存在超过23小时，则重新连接
		if now.Sub(startTime) > 23*time.Hour {
			log.Printf("连接 %s 已接近24小时限制，准备重新连接", symbol)

			// 关闭旧连接
			if conn := c.connections[symbol]; conn != nil {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				_ = conn.Close()
				delete(c.connections, symbol)
			}

			// 删除开始时间记录
			delete(c.connStartTime, symbol)

			// 触发重新连接
			go c.startTickerStream(symbol)
		}
	}
}

// startTickerStream 启动指定交易对的ticker数据流
func (c *BinanceClient) startTickerStream(symbol string) {
	for {
		select {
		case <-c.stopChan:
			return
		default:
			c.connectAndListen(symbol)

			// 如果连接断开，等待一段时间后重新连接
			// 使用指数退避策略，避免频繁重连
			time.Sleep(5 * time.Second)
		}
	}
}

// connectAndListen 连接WebSocket并监听数据
func (c *BinanceClient) connectAndListen(symbol string) {
	// 设置代理
	var dialer *websocket.Dialer
	if c.proxyURL != "" {
		proxyURL, err := url.Parse(c.proxyURL)
		if err != nil {
			log.Printf("解析代理地址失败: %v", err)
			return
		}
		dialer = &websocket.Dialer{
			Proxy:            http.ProxyURL(proxyURL),
			HandshakeTimeout: 10 * time.Second,
		}
	} else {
		dialer = &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}
	}

	// 构建WebSocket连接地址，使用K线数据流
	wsURL := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   "/ws/" + symbol + "@kline_1m", // 使用1分钟K线
	}

	log.Printf("连接到 %s", wsURL.String())

	// 建立WebSocket连接
	conn, _, err := dialer.Dial(wsURL.String(), nil)
	if err != nil {
		log.Printf("连接失败: %v", err)
		return
	}

	// 记录连接和开始时间
	c.reconnectLock.Lock()
	c.connections[symbol] = conn
	c.connStartTime[symbol] = time.Now()
	c.reconnectLock.Unlock()

	// 确保连接关闭
	defer func() {
		c.reconnectLock.Lock()
		if c.connections[symbol] == conn {
			_ = conn.Close()
			delete(c.connections, symbol)
		}
		c.reconnectLock.Unlock()
	}()

	// 设置读取超时
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))

	// 设置处理Ping/Pong消息
	conn.SetPingHandler(func(appData string) error {
		// 收到ping后立即回复pong
		err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
		if err != nil {
			log.Printf("发送pong失败: %v", err)
		} else {
			log.Printf("收到ping，已回复pong")
		}
		// 重置读取超时
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
		return nil
	})

	// 设置Pong处理器
	conn.SetPongHandler(func(appData string) error {
		// 重置读取超时
		_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
		return nil
	})

	// 每5分钟主动发送一次pong消息
	pongTicker := time.NewTicker(5 * time.Minute)
	defer pongTicker.Stop()

	// 每10秒钟处理一次数据
	dataTicker := time.NewTicker(c.interval)
	defer dataTicker.Stop()

	var lastMessage *BinanceTickerMessage

	// 启动一个goroutine发送心跳
	go func() {
		for {
			select {
			case <-c.stopChan:
				return
			case <-pongTicker.C:
				c.reconnectLock.Lock()
				currentConn := c.connections[symbol]
				c.reconnectLock.Unlock()

				if currentConn != conn {
					return // 连接已更新，退出此goroutine
				}

				// 发送pong消息保持连接活跃
				err := conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(5*time.Second))
				if err != nil {
					log.Printf("发送主动pong失败: %v", err)
					return // 发送失败，退出此goroutine
				}
				log.Printf("已发送主动pong消息")
			}
		}
	}()

	// 限制消息处理速率，避免超过每秒10条消息的限制
	messageLimiter := time.NewTicker(100 * time.Millisecond) // 每秒最多处理10条消息
	defer messageLimiter.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-dataTicker.C:
			if lastMessage != nil {
				// 处理最新数据
				c.processMessage(lastMessage)
				lastMessage = nil
			}
		default:
			// 等待限速器
			<-messageLimiter.C

			// 读取消息
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("读取错误: %v", err)
				return
			}

			// 重置读取超时
			_ = conn.SetReadDeadline(time.Now().Add(15 * time.Minute))

			// 解析消息
			var tickerMsg BinanceTickerMessage
			err = json.Unmarshal(message, &tickerMsg)
			if err != nil {
				log.Printf("JSON解析错误: %v", err)
				continue
			}

			// 保存最新消息
			lastMessage = &tickerMsg
		}
	}
}

// processMessage 处理接收到的消息
func (c *BinanceClient) processMessage(msg *BinanceTickerMessage) {
	// 将价格字符串转换为float64
	price, err := strconv.ParseFloat(msg.Kline.ClosePrice, 64)
	if err != nil {
		log.Printf("价格转换错误: %v", err)
		return
	}

	// 更新价格数据
	c.dataManager.UpdatePrice(msg.Symbol, price)
}
