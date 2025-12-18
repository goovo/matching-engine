package engine

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/goovo/matching-engine/util"
)

// Polymarket 模拟参数
const (
	PolyBasePrice = 0.50
	PolyTickSize  = 0.001 // $0.001 精度
	PolySpread    = 0.005 // 0.5% 价差
)

// 生成符合 Polymarket 特征的随机价格
// 价格集中在 fairPrice 附近，呈正态分布
func generatePolyPrice(r *rand.Rand, fairPrice float64, isBuy bool) string {
	// 简单的随机波动
	// 买单价格 = 公允价 - 随机差值 (0 ~ Spread*2)
	// 卖单价格 = 公允价 + 随机差值 (0 ~ Spread*2)
	
	spread := r.Float64() * PolySpread * 5 // 扩大一点范围模拟深度
	var price float64
	if isBuy {
		price = fairPrice - spread
	} else {
		price = fairPrice + spread
	}

	// 限制在 0.01 - 0.99 之间
	if price < 0.01 { price = 0.01 }
	if price > 0.99 { price = 0.99 }

	// 格式化为字符串，保留3位小数
	return strconv.FormatFloat(price, 'f', 3, 64)
}

// BenchmarkPolymarketSimulation 模拟真实预测市场场景
func BenchmarkPolymarketSimulation(b *testing.B) {
	// 初始化订单簿
	ob := NewOrderBook(nil)
	
	// 固定随机种子，保证结果可复现
	r := rand.New(rand.NewSource(42))

	// 1. 预热阶段：预先填充订单簿 (Pre-fill)
	// 模拟一个流动性充足的市场，有 10000 个挂单
	prefillCount := 10000
	fairPrice := PolyBasePrice
	
	for i := 0; i < prefillCount; i++ {
		isBuy := r.Intn(2) == 0
		price := generatePolyPrice(r, fairPrice, isBuy)
		amount := "10.0" // 每个订单 10 份
		
		side := Buy
		if !isBuy {
			side = Sell
		}
		
		id := fmt.Sprintf("pre-%d", i)
		amountDec, _ := polyDecimal(amount)
		priceDec, _ := polyDecimal(price)
		
		ob.Process(*NewOrder(id, side, amountDec, priceDec))
	}

	// 重置计时器，只计算压测阶段
	b.ResetTimer()
	
	// 2. 压测阶段
	for i := 0; i < b.N; i++ {
		// 模拟市场波动：fairPrice 随机游走
		if i % 100 == 0 {
			change := (r.Float64() - 0.5) * 0.01 // +/- 0.005
			fairPrice += change
			if fairPrice < 0.1 { fairPrice = 0.1 }
			if fairPrice > 0.9 { fairPrice = 0.9 }
		}

		// 决定订单类型
		// 90% 限价单 (Maker)，10% 市价单 (Taker)
		isLimit := r.Intn(100) < 90
		isBuy := r.Intn(2) == 0
		
		amount := "5.0"
		id := fmt.Sprintf("bench-%d", i)
		amountDec, _ := polyDecimal(amount)

		if isLimit {
			// 限价单
			price := generatePolyPrice(r, fairPrice, isBuy)
			priceDec, _ := polyDecimal(price)
			
			side := Buy
			if !isBuy { side = Sell }
			
			ob.Process(*NewOrder(id, side, amountDec, priceDec))
		} else {
			// 市价单 (模拟吃单)
			// 注意：ProcessMarket 实际上是 Process 的一种特殊情况（或者单独调用）
			// 这里我们为了简单，用极端的限价单模拟市价单（Polymarket UI 也是这么做的，设置一个极端价格）
			// 买单：价格设为 1.00
			// 卖单：价格设为 0.00
			
			var price string
			side := Buy
			if isBuy {
				price = "1.00"
			} else {
				side = Sell
				price = "0.00"
			}
			priceDec, _ := polyDecimal(price)
			
			// 实际上引擎有 ProcessMarket 方法，我们混合使用
			// 但为了测试统一入口，这里用 Process
			ob.Process(*NewOrder(id, side, amountDec, priceDec))
		}
	}
}

// 辅助函数：快速创建 Decimal
func polyDecimal(s string) (*util.StandardBigDecimal, error) {
	return util.NewDecimalFromString(s)
}
