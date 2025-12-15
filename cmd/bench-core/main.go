package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goovo/matching-engine/engine"
	"github.com/goovo/matching-engine/util"
)

// stats 统计压测数据
type stats struct {
	requests  int64
	success   int64
	failed    int64
	latencyNs int64
}

const (
	duration = 5 * time.Second
)

func main() {
	// 设置 P-State 以利用多核
	runtime.GOMAXPROCS(runtime.NumCPU())

	fmt.Println("=========================================================")
	fmt.Println("   Polymarket Matching Engine Core Performance Benchmark")
	fmt.Println("=========================================================")
	fmt.Printf("CPU Cores: %d\n", runtime.NumCPU())
	fmt.Println("Optimization: Pre-generated Decimals, No String Parsing")
	fmt.Println("---------------------------------------------------------")

	// 场景 1: 单线程限价单 (Limit Order - Single Thread)
	// 这是一个最纯粹的基准，衡量没有任何锁竞争时引擎的理论上限
	runBenchmark("Limit Order (Single Thread)", 1, false)

	// 场景 2: 并发限价单 (Limit Order - Multi Thread)
	// 衡量在有锁竞争情况下的表现 (10 workers)
	runBenchmark("Limit Order (Concurrency 10)", 10, false)

	// 场景 3: 市价单 (Market Order - Single Thread)
	// 先填充订单簿，然后用市价单吃单
	runBenchmark("Market Order (Single Thread)", 1, true)
}

func runBenchmark(name string, workers int, isMarket bool) {
	fmt.Printf("\nRunning: %s ...\n", name)

	ob := engine.NewOrderBook()
	
	// 如果是市价单测试，先预填充一些流动性
	if isMarket {
		fmt.Println("  -> Pre-filling orderbook with 100k limit orders...")
		preFillOrderBook(ob)
	}

	var s stats
	var wg sync.WaitGroup
	wg.Add(workers)

	stop := make(chan struct{})

	// 启动 Worker
	for i := 0; i < workers; i++ {
		go worker(i, ob, stop, &s, &wg, isMarket)
	}

	// 运行指定时间
	time.Sleep(duration)
	close(stop)
	wg.Wait()

	printResults(name, &s)
}

func preFillOrderBook(ob *engine.OrderBook) {
	// 预填充买卖单各 50,000，价格分布广一点避免无法撮合
	for i := 0; i < 50000; i++ {
		// Sell orders: 100 ~ 200
		pSell := 100.0 + float64(i%1000)/10.0
		ob.Process(engine.Order{
			ID:     fmt.Sprintf("pre-sell-%d", i),
			Type:   engine.Sell,
			Amount: util.NewDecimalFromFloat(1.0),
			Price:  util.NewDecimalFromFloat(pSell),
		})
		
		// Buy orders: 0 ~ 100
		pBuy := 99.0 - float64(i%1000)/10.0
		ob.Process(engine.Order{
			ID:     fmt.Sprintf("pre-buy-%d", i),
			Type:   engine.Buy,
			Amount: util.NewDecimalFromFloat(1.0),
			Price:  util.NewDecimalFromFloat(pBuy),
		})
	}
}

func worker(id int, ob *engine.OrderBook, stop <-chan struct{}, s *stats, wg *sync.WaitGroup, isMarket bool) {
	defer wg.Done()

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

	// 预先创建常用变量，减少循环内内存分配
	var (
		priceFloat  float64
		amountFloat float64
		order       engine.Order
		orderType   engine.Side
	)

	// 循环内避免字符串操作
	for {
		select {
		case <-stop:
			return
		default:
			if isMarket {
				// 市价单测试：生成能吃掉预埋单的市价单
				// 随机买或卖
				if r.Intn(2) == 0 {
					orderType = engine.Buy // 市价买，吃掉 Sell 挂单
				} else {
					orderType = engine.Sell // 市价卖，吃掉 Buy 挂单
				}
				amountFloat = 0.1 + r.Float64()*5.0 // 稍微大一点的量
				
				order = engine.Order{
					ID:     "m-bench", // ID 复用不影响核心计算逻辑（除非有map check，但在process内部不关键）
					Type:   orderType,
					Amount: util.NewDecimalFromFloat(amountFloat),
					Price:  util.NewDecimalFromFloat(0), // 市价单价格无所谓
				}
				
				start := time.Now()
				ob.ProcessMarket(order)
				atomic.AddInt64(&s.latencyNs, time.Since(start).Nanoseconds())
				
			} else {
				// 限价单测试
				if r.Intn(2) == 0 {
					orderType = engine.Buy
				} else {
					orderType = engine.Sell
				}

				// 价格集中在 95-105 之间产生频繁撮合
				priceFloat = 95.0 + r.Float64()*10.0
				amountFloat = 0.1 + r.Float64()*2.0

				order = engine.Order{
					ID:     "l-bench",
					Type:   orderType,
					Amount: util.NewDecimalFromFloat(amountFloat),
					Price:  util.NewDecimalFromFloat(priceFloat),
				}

				start := time.Now()
				ob.Process(order)
				atomic.AddInt64(&s.latencyNs, time.Since(start).Nanoseconds())
			}
			
			atomic.AddInt64(&s.requests, 1)
			atomic.AddInt64(&s.success, 1)
		}
	}
}

func printResults(name string, s *stats) {
	dur := duration.Seconds()
	reqs := atomic.LoadInt64(&s.requests)
	totalLat := atomic.LoadInt64(&s.latencyNs)

	avgLat := float64(0)
	if reqs > 0 {
		avgLat = float64(totalLat) / float64(reqs) / 1e6 // ms
	}

	tps := float64(reqs) / dur

	fmt.Printf("  -> Total Reqs:  %d\n", reqs)
	fmt.Printf("  -> TPS:         %.2f /s\n", tps)
	fmt.Printf("  -> Avg Latency: %.3f ms\n", avgLat)
}
