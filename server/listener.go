package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/goovo/matching-engine/engine"
	"github.com/goovo/matching-engine/pkg/mq"
	"github.com/goovo/matching-engine/util"
)

// TradeEvent 定义发送到 Kafka 的成交事件结构
type TradeEvent struct {
	EventType    string `json:"event_type"`
	TradeID      string `json:"trade_id"`
	Symbol       string `json:"symbol"`
	MakerOrderID string `json:"maker_order_id"`
	TakerOrderID string `json:"taker_order_id"`
	Side         string `json:"side"` // Maker 的方向
	Price        string `json:"price"`
	Amount       string `json:"amount"`
	Timestamp    int64  `json:"timestamp"`
}

// KafkaMatchingListener 实现 engine.MatchingListener 接口
type KafkaMatchingListener struct {
	pair     string
	producer *mq.Producer
}

// NewKafkaMatchingListener 创建一个新的监听器
func NewKafkaMatchingListener(pair string, producer *mq.Producer) *KafkaMatchingListener {
	return &KafkaMatchingListener{
		pair:     pair,
		producer: producer,
	}
}

// OnTrade 当发生撮合时触发
func (l *KafkaMatchingListener) OnTrade(makerOrderID, takerOrderID string, side engine.Side, price, amount int64) {
	// 将 int64 定点数转换为标准字符串格式 (1e8 scale)
	priceStr := (&util.StandardBigDecimal{Val: price}).String()
	amountStr := (&util.StandardBigDecimal{Val: amount}).String()

	// 生成唯一 Trade ID
	tradeID := fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), makerOrderID, takerOrderID)

	event := TradeEvent{
		EventType:    "trade",
		TradeID:      tradeID,
		Symbol:       l.pair,
		MakerOrderID: makerOrderID,
		TakerOrderID: takerOrderID,
		Side:         side.String(),
		Price:        priceStr,
		Amount:       amountStr,
		Timestamp:    time.Now().UnixMilli(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling trade event: %v\n", err)
		return
	}

	if l.producer != nil {
		l.producer.AsyncWrite(data)
	}
}

// OnOrderCancelled 当订单被取消时触发 (可选实现)
func (l *KafkaMatchingListener) OnOrderCancelled(orderID string) {
	// 暂不处理
}

// OnOrderAccepted 当订单成功进入订单簿时触发 (可选实现)
func (l *KafkaMatchingListener) OnOrderAccepted(orderID string) {
	// 暂不处理
}
