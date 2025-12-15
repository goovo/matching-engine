package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	engineGrpc "github.com/goovo/matching-engine/engineGrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	addr        = flag.String("addr", "localhost:9000", "server address")
	concurrency = flag.Int("c", 10, "concurrency")
	duration    = flag.Duration("d", 10*time.Second, "duration")
	pair        = flag.String("pair", "BTC/USDT", "trading pair")
)

type stats struct {
	requests  int64
	success   int64
	failed    int64
	latencyNs int64
}

func main() {
	flag.Parse()

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := engineGrpc.NewEngineClient(conn)
	
	fmt.Printf("Starting benchmark on %s with %d concurrent workers for %v...\n", *addr, *concurrency, *duration)

	var wg sync.WaitGroup
	start := time.Now()
	stop := make(chan struct{})
	
	// Global stats
	var s stats

	// Timer to stop benchmark
	go func() {
		time.Sleep(*duration)
		close(stop)
	}()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id, client, stop, &s)
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(start)

	printStats(totalDuration, &s)
}

func worker(id int, client engineGrpc.EngineClient, stop <-chan struct{}, s *stats) {
	ctx := context.Background()
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

	for {
		select {
		case <-stop:
			return
		default:
			// Mix of limit buy/sell orders
			isBuy := r.Intn(2) == 0
			side := engineGrpc.Side_sell
			if isBuy {
				side = engineGrpc.Side_buy
			}
			
			// Random price around 100.0
			price := 90.0 + r.Float64()*20.0
			amount := 0.1 + r.Float64()*2.0
			
			orderID := fmt.Sprintf("bench-%d-%d", id, time.Now().UnixNano())

			req := &engineGrpc.Order{
				ID:     orderID,
				Type:   side,
				Amount: fmt.Sprintf("%.2f", amount),
				Price:  fmt.Sprintf("%.2f", price),
				Pair:   *pair,
			}

			start := time.Now()
			atomic.AddInt64(&s.requests, 1)
			
			_, err := client.Process(ctx, req)
			
			lat := time.Since(start).Nanoseconds()
			atomic.AddInt64(&s.latencyNs, lat)

			if err != nil {
				atomic.AddInt64(&s.failed, 1)
			} else {
				atomic.AddInt64(&s.success, 1)
			}
		}
	}
}

func printStats(d time.Duration, s *stats) {
	reqs := atomic.LoadInt64(&s.requests)
	succ := atomic.LoadInt64(&s.success)
	fail := atomic.LoadInt64(&s.failed)
	lat := atomic.LoadInt64(&s.latencyNs)

	qps := float64(reqs) / d.Seconds()
	avgLat := float64(0)
	if reqs > 0 {
		avgLat = float64(lat) / float64(reqs) / 1e6 // ms
	}

	fmt.Println("\nBenchmark Result:")
	fmt.Printf("  Duration:     %v\n", d)
	fmt.Printf("  Total Reqs:   %d\n", reqs)
	fmt.Printf("  Success:      %d\n", succ)
	fmt.Printf("  Failed:       %d\n", fail)
	fmt.Printf("  QPS:          %.2f\n", qps)
	fmt.Printf("  Avg Latency:  %.3f ms\n", avgLat)
}
