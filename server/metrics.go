package server

import (
	"fmt"
	"sync/atomic"
	"time"
)

// 中文注释：
// 该文件实现了一个轻量级的性能指标采集与打印模块，不依赖任何第三方库。
// 采集的核心指标包括：
// - 每秒请求数（QPS）：按 RPC 方法分别统计 Process/ProcessMarket/Cancel/FetchBook
// - 平均延迟（Avg Latency）：按方法统计平均耗时（毫秒）
// - 每秒撮合笔数（Matches/s）：统计限价与市价撮合返回的成交订单数量
//
// 使用方式：
// - 在 main.go 中调用 StartMetrics() 启动后台打印协程
// - 在各 RPC 方法内调用对应的 Inc* 函数上报计数与耗时

// 下面是各类原子计数器（使用 int64 原子操作，避免锁开销）
var (
	// RPC 调用计数
	processCalls      int64
	processMarketCalls int64
	cancelCalls       int64
	fetchBookCalls    int64

	// 撮合笔数计数（ordersProcessed 的长度）
	processMatches       int64
	processMarketMatches int64

	// 总耗时（纳秒），用于计算平均耗时
	processLatencyNs      int64
	processMarketLatencyNs int64
	cancelLatencyNs       int64
	fetchBookLatencyNs    int64
)

// StartMetrics 启动后台打印任务，每秒输出一行核心性能指标
// 中文注释：该函数应在服务启动时调用一次
func StartMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		// 上一秒的快照，用于计算每秒增量
		var prevProcess, prevProcessMarket, prevCancel, prevFetch int64
		var prevMatchProcess, prevMatchMarket int64
		var prevLatencyProcess, prevLatencyProcessMarket, prevLatencyCancel, prevLatencyFetch int64

		for range ticker.C {
			// 读取当前计数
			curProcess := atomic.LoadInt64(&processCalls)
			curProcessMarket := atomic.LoadInt64(&processMarketCalls)
			curCancel := atomic.LoadInt64(&cancelCalls)
			curFetch := atomic.LoadInt64(&fetchBookCalls)

			curMatchProcess := atomic.LoadInt64(&processMatches)
			curMatchMarket := atomic.LoadInt64(&processMarketMatches)

			curLatencyProcess := atomic.LoadInt64(&processLatencyNs)
			curLatencyProcessMarket := atomic.LoadInt64(&processMarketLatencyNs)
			curLatencyCancel := atomic.LoadInt64(&cancelLatencyNs)
			curLatencyFetch := atomic.LoadInt64(&fetchBookLatencyNs)

			// 计算每秒增量
			deltaProcess := curProcess - prevProcess
			deltaProcessMarket := curProcessMarket - prevProcessMarket
			deltaCancel := curCancel - prevCancel
			deltaFetch := curFetch - prevFetch

			deltaMatchProcess := curMatchProcess - prevMatchProcess
			deltaMatchMarket := curMatchMarket - prevMatchMarket

			deltaLatencyProcess := curLatencyProcess - prevLatencyProcess
			deltaLatencyProcessMarket := curLatencyProcessMarket - prevLatencyProcessMarket
			deltaLatencyCancel := curLatencyCancel - prevLatencyCancel
			deltaLatencyFetch := curLatencyFetch - prevLatencyFetch

			// 计算平均耗时（毫秒）；为避免除零，若对应 QPS 为 0 则显示为 0
			avgLatProcessMs := float64(0)
			if deltaProcess > 0 {
				avgLatProcessMs = float64(deltaLatencyProcess) / float64(deltaProcess) / 1e6
			}
			avgLatProcessMarketMs := float64(0)
			if deltaProcessMarket > 0 {
				avgLatProcessMarketMs = float64(deltaLatencyProcessMarket) / float64(deltaProcessMarket) / 1e6
			}
			avgLatCancelMs := float64(0)
			if deltaCancel > 0 {
				avgLatCancelMs = float64(deltaLatencyCancel) / float64(deltaCancel) / 1e6
			}
			avgLatFetchMs := float64(0)
			if deltaFetch > 0 {
				avgLatFetchMs = float64(deltaLatencyFetch) / float64(deltaFetch) / 1e6
			}

			// 若所有方法在该秒内均无请求（且无撮合笔数变化），则跳过打印，避免刷屏的 0 值
			if (deltaProcess + deltaProcessMarket + deltaCancel + deltaFetch + deltaMatchProcess + deltaMatchMarket) > 0 {
				// 打印一行指标（可根据需要改为结构化日志）
				fmt.Printf(
					"[metrics] Process QPS=%d Avg=%.3fms Matches/s=%d | Market QPS=%d Avg=%.3fms Matches/s=%d | Cancel QPS=%d Avg=%.3fms | FetchBook QPS=%d Avg=%.3fms\n",
					deltaProcess, avgLatProcessMs, deltaMatchProcess,
					deltaProcessMarket, avgLatProcessMarketMs, deltaMatchMarket,
					deltaCancel, avgLatCancelMs,
					deltaFetch, avgLatFetchMs,
				)
			}

			// 更新快照
			prevProcess = curProcess
			prevProcessMarket = curProcessMarket
			prevCancel = curCancel
			prevFetch = curFetch
			prevMatchProcess = curMatchProcess
			prevMatchMarket = curMatchMarket
			prevLatencyProcess = curLatencyProcess
			prevLatencyProcessMarket = curLatencyProcessMarket
			prevLatencyCancel = curLatencyCancel
			prevLatencyFetch = curLatencyFetch
		}
	}()
}

// IncProcess 在限价单处理完成后调用，记录调用次数、耗时与撮合笔数
// 中文注释：matches 传入本次撮合返回的已处理订单数量
func IncProcess(start time.Time, matches int) {
	atomic.AddInt64(&processCalls, 1)
	atomic.AddInt64(&processLatencyNs, time.Since(start).Nanoseconds())
	if matches > 0 {
		atomic.AddInt64(&processMatches, int64(matches))
	}
}

// IncProcessMarket 在市价单处理完成后调用
func IncProcessMarket(start time.Time, matches int) {
	atomic.AddInt64(&processMarketCalls, 1)
	atomic.AddInt64(&processMarketLatencyNs, time.Since(start).Nanoseconds())
	if matches > 0 {
		atomic.AddInt64(&processMarketMatches, int64(matches))
	}
}

// IncCancel 在撤单处理完成后调用
func IncCancel(start time.Time) {
	atomic.AddInt64(&cancelCalls, 1)
	atomic.AddInt64(&cancelLatencyNs, time.Since(start).Nanoseconds())
}

// IncFetchBook 在查询订单簿处理完成后调用
func IncFetchBook(start time.Time) {
	atomic.AddInt64(&fetchBookCalls, 1)
	atomic.AddInt64(&fetchBookLatencyNs, time.Since(start).Nanoseconds())
}
