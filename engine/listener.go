package engine

// MatchingListener 定义撮合引擎的事件回调接口
// 实现该接口以接收撮合结果。
// 注意：实现必须高效且非阻塞，建议直接写入 RingBuffer 或 Channel。
type MatchingListener interface {
	// OnTrade 当发生撮合时触发
	// price 和 amount 是 int64 格式的定点数 (Scale=1e8)
	OnTrade(makerOrderID, takerOrderID string, side Side, price, amount int64)

	// OnOrderCancelled 当订单被取消时触发
	OnOrderCancelled(orderID string)

	// OnOrderAccepted 当订单成功进入订单簿（Maker）时触发
	OnOrderAccepted(orderID string)
}

// NoOpListener 空实现，用于默认情况
type NoOpListener struct{}

func (l *NoOpListener) OnTrade(makerID, takerID string, side Side, price, amount int64) {}
func (l *NoOpListener) OnOrderCancelled(id string) {}
func (l *NoOpListener) OnOrderAccepted(id string) {}
