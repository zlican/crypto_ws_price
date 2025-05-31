package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"binance-trend/utils"
)

func main() {
	// 创建市场数据管理器
	dataManager := utils.NewMarketDataManager()

	// 创建币安WebSocket客户端
	// 设置代理地址，如果不需要代理可以设置为空字符串
	proxyURL := "http://127.0.0.1:10809"
	symbols := []string{"btcusdt", "ethusdt"}
	client := utils.NewBinanceClient(dataManager, proxyURL, symbols, 10) // 每10秒更新一次

	// 创建API服务器
	api := utils.NewPriceAPI(8081, dataManager)

	// 启动WebSocket客户端
	client.Start()
	log.Println("WebSocket客户端已启动，正在获取价格数据...")

	// 在单独的goroutine中启动API服务器
	go func() {
		log.Println("API服务器启动中...")
		if err := api.Start(); err != nil {
			log.Fatalf("API服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 收到中断信号后，停止WebSocket客户端
	log.Println("正在关闭...")
	client.Stop()
	log.Println("已关闭")
}
