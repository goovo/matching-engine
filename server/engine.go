package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goovo/matching-engine/engine"
	engineGrpc "github.com/goovo/matching-engine/engineGrpc"
	"github.com/goovo/matching-engine/pkg/mq"
	"github.com/goovo/matching-engine/util"
)

// Engine 引擎服务实现，维护每个交易对的订单簿
type Engine struct {
	book      map[string]*engine.OrderBook
	mu        sync.RWMutex
	producer  *mq.Producer
	pairCount int64 // 活跃交易对数量 (atomic)
}

// NewEngine 返回 Engine 实例
func NewEngine() *Engine {
	// 简单的 .env 解析逻辑 (兼容 YAML 风格)
	kafkaHost := "localhost"
	kafkaPort := "9092"
	kafkaTopic := "market_events"

	if content, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "host:") {
				kafkaHost = strings.TrimSpace(strings.TrimPrefix(line, "host:"))
			} else if strings.HasPrefix(line, "port:") {
				kafkaPort = strings.TrimSpace(strings.TrimPrefix(line, "port:"))
			} else if strings.HasPrefix(line, "topic:") {
				kafkaTopic = strings.TrimSpace(strings.TrimPrefix(line, "topic:"))
			}
		}
	}

	brokers := []string{fmt.Sprintf("%s:%s", kafkaHost, kafkaPort)}
	producer := mq.NewProducer(brokers, kafkaTopic)
	fmt.Printf("Kafka Producer initialized: %v, topic: %s\n", brokers, kafkaTopic)

	return &Engine{
		book:     map[string]*engine.OrderBook{},
		producer: producer,
	}
}

// Process 实现 EngineServer 接口：处理限价单
func (e *Engine) Process(ctx context.Context, req *engineGrpc.Order) (*engineGrpc.OutputOrders, error) {
	start := time.Now() // 中文注释：记录方法开始时间用于统计耗时
	bigZero, _ := util.NewDecimalFromString("0.0")
	orderString := fmt.Sprintf("{\"id\":\"%s\", \"type\": \"%s\", \"amount\": \"%s\", \"price\": \"%s\" }", req.GetID(), req.GetType().String(), req.GetAmount(), req.GetPrice())

	var order engine.Order
	// 解析消息体
	// fmt.Println("Orderstring =: ", orderString)
	err := order.FromJSON([]byte(orderString))
	if err != nil {
		fmt.Println("JSON Parse Error =: ", err)
		return nil, err
	}

	if order.Amount.Cmp(bigZero) == 0 || order.Price.Cmp(bigZero) == 0 {
		fmt.Println("Invalid JSON")
		return nil, errors.New("Invalid JSON")
	}

	if req.GetPair() == "" {
		fmt.Println("Invalid pair")
		return nil, errors.New("Invalid pair")
	}

	var pairBook *engine.OrderBook
	e.mu.RLock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
		e.mu.RUnlock()
	} else {
		e.mu.RUnlock()
		e.mu.Lock()
		// Double check
		if val, ok := e.book[req.GetPair()]; ok {
			pairBook = val
		} else {
			// 注入 Kafka 监听器
			listener := NewKafkaMatchingListener(req.GetPair(), e.producer)
			pairBook = engine.NewOrderBook(listener)
			e.book[req.GetPair()] = pairBook
			atomic.AddInt64(&e.pairCount, 1)
		}
		e.mu.Unlock()
	}

	ordersProcessed, partialOrder := pairBook.Process(order)
	// 中文注释：统计限价撮合的成交笔数与耗时
	IncProcess(start, len(ordersProcessed))

	ordersProcessedString, err := json.Marshal(ordersProcessed)

	// if order.Type.String() == "sell" {
	// fmt.Println("pair:", req.GetPair())
	// fmt.Println(pairBook)
	// }

	if err != nil {
		fmt.Println("Marshal error", err)
		return nil, err
	}

	if partialOrder != nil {
		var partialOrderString []byte
		partialOrderString, err = json.Marshal(partialOrder)
		if err != nil {
			fmt.Println("partialOrderString Marshal error", err)
			return nil, err
		}
		return &engineGrpc.OutputOrders{OrdersProcessed: string(ordersProcessedString), PartialOrder: string(partialOrderString)}, nil
	}
	return &engineGrpc.OutputOrders{OrdersProcessed: string(ordersProcessedString), PartialOrder: "null"}, nil
}

// Cancel 实现 EngineServer 接口：撤单
func (e *Engine) Cancel(ctx context.Context, req *engineGrpc.Order) (*engineGrpc.Order, error) {
	start := time.Now() // 中文注释：记录方法开始时间用于统计耗时
	order := &engine.Order{ID: req.GetID()}

	if order.ID == "" {
		fmt.Println("Invalid JSON")
		return nil, errors.New("Invalid JSON")
	}

	if req.GetPair() == "" {
		fmt.Println("Invalid pair")
		return nil, errors.New("Invalid pair")
	}

	var pairBook *engine.OrderBook
	e.mu.RLock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
		e.mu.RUnlock()
	} else {
		e.mu.RUnlock()
		e.mu.Lock()
		// Double check
		if val, ok := e.book[req.GetPair()]; ok {
			pairBook = val
		} else {
			// 注入 Kafka 监听器
			listener := NewKafkaMatchingListener(req.GetPair(), e.producer)
			pairBook = engine.NewOrderBook(listener)
			e.book[req.GetPair()] = pairBook
			atomic.AddInt64(&e.pairCount, 1)
		}
		e.mu.Unlock()
	}

	order = pairBook.CancelOrder(order.ID)

	// fmt.Println("pair:", req.GetPair())
	// fmt.Println(pairBook)

	if order == nil {
		return nil, errors.New("NoOrderPresent")
	}

	orderEngine := &engineGrpc.Order{}

	orderEngine.ID = order.ID
	orderEngine.Amount = order.Amount.String()
	orderEngine.Price = order.Price.String()
	orderEngine.Type = engineGrpc.Side(engineGrpc.Side_value[order.Type.String()])

	// 中文注释：统计撤单的耗时
	IncCancel(start)

	return orderEngine, nil
}

// ProcessMarket 实现 EngineServer 接口：处理市价单
func (e *Engine) ProcessMarket(ctx context.Context, req *engineGrpc.Order) (*engineGrpc.OutputOrders, error) {
	start := time.Now() // 中文注释：记录方法开始时间用于统计耗时
	bigZero, _ := util.NewDecimalFromString("0.0")
	orderString := fmt.Sprintf("{\"id\":\"%s\", \"type\": \"%s\", \"amount\": \"%s\", \"price\": \"%s\" }", req.GetID(), req.GetType().String(), req.GetAmount(), req.GetPrice())

	var order engine.Order
	// 解析消息体
	// fmt.Println("Orderstring =: ", orderString)
	err := order.FromJSON([]byte(orderString))
	if err != nil {
		fmt.Println("JSON Parse Error =: ", err)
		return nil, err
	}

	if order.Amount.Cmp(bigZero) == 0 {
		fmt.Println("Invalid JSON")
		return nil, errors.New("Invalid JSON")
	}

	if req.GetPair() == "" {
		fmt.Println("Invalid pair")
		return nil, errors.New("Invalid pair")
	}

	var pairBook *engine.OrderBook
	e.mu.RLock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
		e.mu.RUnlock()
	} else {
		e.mu.RUnlock()
		e.mu.Lock()
		// Double check
		if val, ok := e.book[req.GetPair()]; ok {
			pairBook = val
		} else {
			// 注入 Kafka 监听器
			listener := NewKafkaMatchingListener(req.GetPair(), e.producer)
			pairBook = engine.NewOrderBook(listener)
			e.book[req.GetPair()] = pairBook
			atomic.AddInt64(&e.pairCount, 1)
		}
		e.mu.Unlock()
	}

	ordersProcessed, partialOrder := pairBook.ProcessMarket(order)
	// 中文注释：统计市价撮合的成交笔数与耗时
	IncProcessMarket(start, len(ordersProcessed))

	ordersProcessedString, err := json.Marshal(ordersProcessed)

	// if order.Type.String() == "sell" {
	// fmt.Println("pair:", req.GetPair())
	// fmt.Println(pairBook)
	// }

	if err != nil {
		return nil, err
	}

	if partialOrder != nil {
		var partialOrderString []byte
		partialOrderString, err = json.Marshal(partialOrder)
		return &engineGrpc.OutputOrders{OrdersProcessed: string(ordersProcessedString), PartialOrder: string(partialOrderString)}, nil
	}
	return &engineGrpc.OutputOrders{OrdersProcessed: string(ordersProcessedString), PartialOrder: "null"}, nil
}

// FetchBook 实现 EngineServer 接口：查询订单簿
func (e *Engine) FetchBook(ctx context.Context, req *engineGrpc.BookInput) (*engineGrpc.BookOutput, error) {
	start := time.Now() // 中文注释：记录方法开始时间用于统计耗时
	if req.GetPair() == "" {
		fmt.Println("Invalid pair")
		return nil, errors.New("Invalid pair")
	}

	var pairBook *engine.OrderBook
	e.mu.RLock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
		e.mu.RUnlock()
	} else {
		e.mu.RUnlock()
		return nil, errors.New("Invalid pair")
	}

	// fmt.Println(pairBook)
	book := pairBook.GetOrders(req.GetLimit())

	result := &engineGrpc.BookOutput{Buys: []*engineGrpc.BookArray{}, Sells: []*engineGrpc.BookArray{}}

	for _, buy := range book.Buys {
		arr := &engineGrpc.BookArray{PriceAmount: []string{}}

		bodyBytes, err := json.Marshal(buy)
		if err != nil {
			fmt.Println("1", err)
			return &engineGrpc.BookOutput{Buys: []*engineGrpc.BookArray{}, Sells: []*engineGrpc.BookArray{}}, nil
		}

		err = json.Unmarshal(bodyBytes, &arr.PriceAmount)
		if err != nil {
			fmt.Println("2", err)
			return &engineGrpc.BookOutput{Buys: []*engineGrpc.BookArray{}, Sells: []*engineGrpc.BookArray{}}, nil
		}

		result.Buys = append(result.Buys, arr)
	}

	for _, sell := range book.Sells {
		arr := &engineGrpc.BookArray{PriceAmount: []string{}}

		bodyBytes, err := json.Marshal(sell)
		if err != nil {
			fmt.Println("json.Marshal Error", err)
			return &engineGrpc.BookOutput{Buys: []*engineGrpc.BookArray{}, Sells: []*engineGrpc.BookArray{}}, nil
		}

		err = json.Unmarshal(bodyBytes, &arr.PriceAmount)
		if err != nil {
			fmt.Println("json.Unmarshal Error", err)
			return &engineGrpc.BookOutput{Buys: []*engineGrpc.BookArray{}, Sells: []*engineGrpc.BookArray{}}, nil
		}

		result.Sells = append(result.Sells, arr)
	}
	// 中文注释：统计查询订单簿的耗时
	IncFetchBook(start)
	return result, nil
}

// GetOrderBookCount 实现 EngineServer 接口：获取交易对数量
func (e *Engine) GetOrderBookCount(ctx context.Context, req *engineGrpc.Empty) (*engineGrpc.CountOutput, error) {
	count := atomic.LoadInt64(&e.pairCount)
	return &engineGrpc.CountOutput{Count: count}, nil
}

// DelistPair 实现 EngineServer 接口：下架交易对
func (e *Engine) DelistPair(ctx context.Context, req *engineGrpc.DelistInput) (*engineGrpc.DelistOutput, error) {
	pair := req.GetPair()
	e.mu.Lock()
	ob, ok := e.book[pair]
	if !ok {
		e.mu.Unlock()
		return nil, errors.New("Pair not found")
	}
	delete(e.book, pair)
	atomic.AddInt64(&e.pairCount, -1)
	e.mu.Unlock()

	// 锁外执行：获取并取消所有订单
	orders := ob.CancelAllOrders()

	// 转换格式
	var grpcOrders []*engineGrpc.Order
	for _, o := range orders {
		var side engineGrpc.Side
		if o.Type == engine.Buy {
			side = engineGrpc.Side_buy
		} else {
			side = engineGrpc.Side_sell
		}

		grpcOrders = append(grpcOrders, &engineGrpc.Order{
			ID:     o.ID,
			Type:   side,
			Amount: o.Amount.String(),
			Price:  o.Price.String(),
			Pair:   pair,
		})
	}

	return &engineGrpc.DelistOutput{Orders: grpcOrders}, nil
}
