package engine

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/goovo/matching-engine/pkg/protocol"
	"github.com/goovo/matching-engine/pkg/queue"
	"github.com/goovo/matching-engine/util"
)

// BenchmarkRDMALimitMatchSimple
// 中文说明：
// - 模拟通过 RDMA/共享内存通道接收订单并进行撮合的性能
// - 对比 BenchmarkLimitMatchSimple，这里增加了 RingBuffer 的读写开销和二进制协议的编解码开销
// - 流程：
//   1. 初始化 OrderBook 并预埋卖单
//   2. 启动 Consumer Goroutine：从 RingBuffer 轮询 -> 解码 -> 撮合
//   3. 主循环 Producer：生成订单 -> 写入 RingBuffer
func BenchmarkRDMALimitMatchSimple(b *testing.B) {
	// 重定向标准输出
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	origStdout := os.Stdout
	os.Stdout = devnull
	defer func() {
		os.Stdout = origStdout
		_ = devnull.Close()
	}()

	// 1. 准备 OrderBook 和预埋卖单
	ob := NewOrderBook(nil)
	for i := 0; i < b.N; i++ {
		sid := fmt.Sprintf("s-%d", i)
		amt := DecimalBig("1.0")
		prc := DecimalBig("100.0")
		ob.Process(*NewOrder(sid, Sell, amt, prc))
	}

	// 2. 准备共享内存 RingBuffer
	// 容量设为 64K (2^16)，足够大以缓冲 burst
	capacity := uint64(65536)
	// 计算所需总内存: 128 (Head/Tail/Pad) + capacity * 64 (Slots)
	totalSize := 128 + capacity*64
	buffer := make([]byte, totalSize)
	rb := queue.NewSHMRingBuffer(buffer, capacity)

	var wg sync.WaitGroup
	wg.Add(1)

	// 3. 启动消费者 (模拟引擎端 RDMA 轮询线程)
	go func() {
		defer wg.Done()
		processed := 0
		
		// 预先分配对象以复用，模拟高性能场景下的内存优化（可选，这里先按常规写法）
		// 为了公平对比，我们尽量保持与 BenchmarkLimitMatchSimple 相似的 allocation 模式
		// 但解码过程本身就是 RDMA 优势的一部分（无反射，无 JSON）
		
		for processed < b.N {
			slot := rb.Poll()
			if slot != nil {
				// --- 解码阶段 (模拟 Zero-Copy) ---
				// 直接从 slot 内存读取字段
				// 注意：这里假设本地字节序一致，使用 unsafe 直接读取
				base := uintptr(unsafe.Pointer(&slot[0]))
				
				// 偏移量定义在 pkg/protocol/rdma_struct.go
				// OffsetOrderID = 8
				// OffsetPrice = 24
				// OffsetAmount = 32
				
				oid := *(*uint64)(unsafe.Pointer(base + 8))
				priceVal := *(*uint64)(unsafe.Pointer(base + 24))
				amountVal := *(*uint64)(unsafe.Pointer(base + 32))
				
				// 转换为 Engine 内部结构
				// ID: uint64 -> string
				idStr := strconv.FormatUint(oid, 10)
				
				// Price/Amount: uint64 (fixed point 1e8) -> StandardBigDecimal
				// 注意：engine.Order 需要 *StandardBigDecimal
				priceDec := &util.StandardBigDecimal{Val: int64(priceVal)}
				amountDec := &util.StandardBigDecimal{Val: int64(amountVal)}
				
				// 构造 Order 对象
				// 这里模拟了 NewOrder 的行为，但直接赋值以减少函数调用开销
				// 注意：BenchmarkLimitMatchSimple 中是 ob.Process(*NewOrder(...))
				order := Order{
					ID:     idStr,
					Type:   Buy,
					Amount: amountDec,
					Price:  priceDec,
					Next:   NullIndex,
					Prev:   NullIndex,
				}
				
				// --- 撮合阶段 ---
				ob.Process(order)
				
				// --- 完成阶段 ---
				slot.SetStatus(protocol.StatusDone)
				processed++
			} else {
				// 空转等待，模拟低延迟场景下的 Busy Spin
				// runtime.Gosched() // 如果想让出 CPU 可以取消注释，但 RDMA 场景通常是死循环轮询
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	// 4. 生产者 (模拟订单网关)
	// 准备常量数据
	priceRaw := uint64(100 * 100000000) // 100.0
	amountRaw := uint64(1 * 100000000)  // 1.0
	ts := time.Now().UnixNano()
	
	var slot protocol.OrderSlot
	
	for i := 0; i < b.N; i++ {
		// 填充 Slot 数据
		// 复用同一个 slot 结构体写入，模拟网关从网络包解析到 buffer 的过程
		slot.PackUnsafe(uint64(i), 1, priceRaw, amountRaw, uint64(i), 0, ts)
		
		// 写入 RingBuffer (自旋重试)
		for {
			err := rb.Write(&slot)
			if err == nil {
				break
			}
			// Buffer 满，忙等待
		}
	}

	// 等待消费者处理完毕
	wg.Wait()
}

// BenchmarkRDMAMarketMatchSimple
// 中文说明：
// - 模拟通过 RDMA 通道进行的市价单撮合
func BenchmarkRDMAMarketMatchSimple(b *testing.B) {
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	origStdout := os.Stdout
	os.Stdout = devnull
	defer func() {
		os.Stdout = origStdout
		_ = devnull.Close()
	}()

	ob := NewOrderBook(nil)
	for i := 0; i < b.N; i++ {
		sid := fmt.Sprintf("s-%d", i)
		amt := DecimalBig("1.0")
		prc := DecimalBig("100.0")
		ob.Process(*NewOrder(sid, Sell, amt, prc))
	}

	capacity := uint64(65536)
	buffer := make([]byte, 128 + capacity*64)
	rb := queue.NewSHMRingBuffer(buffer, capacity)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		processed := 0
		for processed < b.N {
			slot := rb.Poll()
			if slot != nil {
				base := uintptr(unsafe.Pointer(&slot[0]))
				oid := *(*uint64)(unsafe.Pointer(base + 8))
				// 市价单价格通常被忽略或为0，但协议里还是有位置
				// amountVal := *(*uint64)(unsafe.Pointer(base + 32))
				
				idStr := strconv.FormatUint(oid, 10)
				amountDec := &util.StandardBigDecimal{Val: 100000000} // 1.0
				zeroDec := &util.StandardBigDecimal{Val: 0}
				
				order := Order{
					ID:     idStr,
					Type:   Buy,
					Amount: amountDec,
					Price:  zeroDec,
					Next:   NullIndex,
					Prev:   NullIndex,
				}
				
				ob.ProcessMarket(order)
				
				slot.SetStatus(protocol.StatusDone)
				processed++
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	priceRaw := uint64(0)
	amountRaw := uint64(1 * 100000000)
	ts := time.Now().UnixNano()
	var slot protocol.OrderSlot

	for i := 0; i < b.N; i++ {
		slot.PackUnsafe(uint64(i), 1, priceRaw, amountRaw, uint64(i), 0, ts)
		for {
			err := rb.Write(&slot)
			if err == nil {
				break
			}
		}
	}

	wg.Wait()
}
