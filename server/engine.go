package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/goovo/matching-engine/engine"
	engineGrpc "github.com/goovo/matching-engine/engineGrpc"
	"github.com/goovo/matching-engine/util"
)

// Engine 引擎服务实现，维护每个交易对的订单簿
type Engine struct {
	book map[string]*engine.OrderBook
	mu   sync.RWMutex
}

// NewEngine 返回 Engine 实例
func NewEngine() *Engine {
	return &Engine{book: map[string]*engine.OrderBook{}}
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
	e.mu.Lock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
	} else {
		pairBook = engine.NewOrderBook()
		e.book[req.GetPair()] = pairBook
	}
	e.mu.Unlock()

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
	e.mu.Lock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
	} else {
		pairBook = engine.NewOrderBook()
		e.book[req.GetPair()] = pairBook
	}
	e.mu.Unlock()

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
	e.mu.Lock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
	} else {
		pairBook = engine.NewOrderBook()
		e.book[req.GetPair()] = pairBook
	}
	e.mu.Unlock()

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
	e.mu.Lock()
	if val, ok := e.book[req.GetPair()]; ok {
		pairBook = val
	} else {
		e.mu.Unlock()
		return nil, errors.New("Invalid pair")
	}
	e.mu.Unlock()

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
