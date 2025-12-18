package engine

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

// BenchmarkLimitMatchSimple
// 中文说明：
// - 该基准测试用于评估限价单撮合的吞吐能力（TPS）
// - 预先在订单簿中插入 b.N 笔卖单（数量与价格均相同），然后撮合 b.N 笔买单与之逐一成交
// - 由于引擎内部存在较多 fmt 打印，为避免 IO 干扰，这里将标准输出重定向到 /dev/null
func BenchmarkLimitMatchSimple(b *testing.B) {
	// 将标准输出重定向，避免 fmt 打印影响基准结果
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	origStdout := os.Stdout
	os.Stdout = devnull
	defer func() {
		os.Stdout = origStdout
		_ = devnull.Close()
	}()

	// 构建订单簿并预置卖单
	ob := NewOrderBook(nil)
	for i := 0; i < b.N; i++ {
		sid := fmt.Sprintf("s-%d", i)                      // 卖单ID
		amt := DecimalBig("1.0")                           // 卖单数量
		prc := DecimalBig("100.0")                         // 卖单价格
		ob.Process(*NewOrder(sid, Sell, amt, prc))  // 预置卖单
	}

	b.ReportAllocs()
	b.ResetTimer()

	// 撮合 b.N 笔买单
	for i := 0; i < b.N; i++ {
		bid := fmt.Sprintf("b-%d", i)                  // 买单ID
		amt := DecimalBig("1.0")                       // 买单数量
		prc := DecimalBig("100.0")                      // 买单价格（与卖单一致保证撮合）
		ob.Process(*NewOrder(bid, Buy, amt, prc)) // 进行撮合
	}
}

// BenchmarkMarketMatchSimple
// 中文说明：
// - 该基准测试用于评估市价单撮合的吞吐能力（TPS）
// - 预先在订单簿中插入 b.N 笔卖单，然后撮合 b.N 笔买入市价单
// - 市价单的价格字段传入 0（不参与比较），仅按数量与最优价依次成交
func BenchmarkMarketMatchSimple(b *testing.B) {
	// 将标准输出重定向，避免 fmt 打印影响基准结果
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	origStdout := os.Stdout
	os.Stdout = devnull
	defer func() {
		os.Stdout = origStdout
		_ = devnull.Close()
	}()

	// 构建订单簿并预置卖单
	ob := NewOrderBook(nil)
	for i := 0; i < b.N; i++ {
		sid := fmt.Sprintf("s-%d", i)                      // 卖单ID
		amt := DecimalBig("1.0")                           // 卖单数量
		prc := DecimalBig("100.0")                         // 卖单价格
		ob.Process(*NewOrder(sid, Sell, amt, prc))  // 预置卖单
	}

	b.ReportAllocs()
	b.ResetTimer()

	// 撮合 b.N 笔买入市价单
	for i := 0; i < b.N; i++ {
		bid := fmt.Sprintf("mb-%d", i)                          // 市价买单ID
		amt := DecimalBig("1.0")                                 // 市价买单数量
		zero := DecimalBig("0.0")                                 // 市价单价格字段置零
		ob.ProcessMarket(*NewOrder(bid, Buy, amt, zero))   // 进行撮合
	}
}

// BenchmarkCancelOrder
// 中文说明：
// - 该基准测试用于评估撤单性能（验证 O(1) 优化效果）
// - 场景：在同一价格档位堆积大量订单，然后随机撤单
func BenchmarkCancelOrder(b *testing.B) {
	// 重定向标准输出
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	origStdout := os.Stdout
	os.Stdout = devnull
	defer func() {
		os.Stdout = origStdout
		_ = devnull.Close()
	}()

	ob := NewOrderBook(nil)
	ids := make([]string, 0, b.N)
	
	// 预置 b.N 个订单在同一价格档位
	// 这会创建一个很长的链表（如果未重构则是切片）
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("order-%d", i)
		ids = append(ids, id)
		amt := DecimalBig("1.0")
		prc := DecimalBig("100.0")
		ob.Process(*NewOrder(id, Buy, amt, prc))
	}

	// 打乱 ID 顺序以模拟随机撤单
	rand.Shuffle(len(ids), func(i, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})

	b.ReportAllocs()
	b.ResetTimer()

	for _, id := range ids {
		ob.CancelOrder(id)
	}
}
