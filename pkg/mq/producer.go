package mq

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer 封装 Kafka Writer，提供异步写入能力
type Producer struct {
	writer  *kafka.Writer
	msgChan chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewProducer 创建一个新的 Producer 实例
// brokers: Kafka 节点列表 (e.g. ["localhost:9092"])
// topic: 目标 Topic
func NewProducer(brokers []string, topic string) *Producer {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
		// 批处理配置，提高吞吐量
		BatchSize:  100,
		BatchTimeout: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Producer{
		writer:  w,
		msgChan: make(chan []byte, 100000), // 大容量缓冲，防止阻塞撮合引擎
		ctx:     ctx,
		cancel:  cancel,
	}

	// 启动后台 Worker
	go p.worker()

	return p
}

// AsyncWrite 异步发送消息
// 该方法是非阻塞的，除非 channel 已满
func (p *Producer) AsyncWrite(msg []byte) {
	select {
	case p.msgChan <- msg:
		// 成功入队
	default:
		// Channel 满，丢弃消息并打印错误，避免阻塞撮合引擎
		// 在生产环境中，这里应该有监控报警
		log.Println("Error: Kafka producer channel full, dropping message")
	}
}

// worker 后台消费 Channel 并写入 Kafka
func (p *Producer) worker() {
	for {
		select {
		case msg := <-p.msgChan:
			err := p.writer.WriteMessages(p.ctx, kafka.Message{
				Value: msg,
			})
			if err != nil {
				log.Printf("Error writing to kafka: %v\n", err)
			}
		case <-p.ctx.Done():
			if err := p.writer.Close(); err != nil {
				log.Printf("Error closing kafka writer: %v\n", err)
			}
			return
		}
	}
}

// Close 关闭 Producer
func (p *Producer) Close() {
	p.cancel()
}
