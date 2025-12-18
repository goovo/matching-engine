package main

import (
	"fmt"

	"github.com/goovo/matching-engine/engine"
	"github.com/goovo/matching-engine/util"
)

// DemoListener 演示用的监听器
type DemoListener struct {
	TradeCount int
}

func (l *DemoListener) OnTrade(makerID, takerID string, side engine.Side, price, amount int64) {
	fmt.Printf("  -> [Output] Trade Executed: Maker=%s Taker=%s Price=%.2f Amount=%.8f\n",
		makerID, takerID, float64(price)/1e8, float64(amount)/1e8)
	l.TradeCount++
}

func (l *DemoListener) OnOrderAccepted(id string) {
	fmt.Printf("  -> [Output] Order Accepted: %s\n", id)
}

func (l *DemoListener) OnOrderCancelled(id string) {
	fmt.Printf("  -> [Output] Order Cancelled: %s\n", id)
}

func main() {
	fmt.Println("=== Starting Matching Engine Simulation ===")

	// 1. 初始化引擎
	listener := &DemoListener{}
	ob := engine.NewOrderBook(listener)

	// 2. 模拟订单流
	orders := []struct {
		ID     string
		Side   engine.Side
		Price  string
		Amount string
	}{
		{"maker-1", engine.Buy, "100.0", "1.0"},
		{"maker-2", engine.Sell, "101.0", "1.0"},
		{"taker-1", engine.Buy, "101.0", "0.5"}, // 吃掉 maker-2 一半
		{"taker-2", engine.Sell, "99.0", "2.0"}, // 吃掉 maker-1 全部，剩余 1.0 挂单
	}

	// 3. 执行流水线
	for _, o := range orders {
		fmt.Printf("\n[Input] Processing Order %s (%s @ %s)...\n", o.ID, o.Side, o.Price)

		// Step 1: Risk Check
		fmt.Println("  -> [Risk] Check Balance: Passed")

		// Step 2: WAL
		fmt.Println("  -> [WAL] Write Log: Success")

		// Step 3: Engine Process
		fmt.Println("  -> [Engine] Matching...")
		price, _ := util.NewDecimalFromString(o.Price)
		amount, _ := util.NewDecimalFromString(o.Amount)
		ob.Process(*engine.NewOrder(o.ID, o.Side, amount, price))
	}

	fmt.Printf("\n=== Simulation Complete. Total Trades: %d ===\n", listener.TradeCount)
}
