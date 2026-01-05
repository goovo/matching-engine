package server

import (
"fmt"
"time"

"github.com/goovo/matching-engine/engine"
"github.com/goovo/matching-engine/util"
"github.com/shopspring/decimal"
)

// ProcessLimitOrder 处理限价单 (Internal Direct Call for RDMA)
func (e *Engine) ProcessLimitOrder(side engine.Side, symbol string, amount, price decimal.Decimal) {
	amt, _ := util.NewDecimalFromString(amount.String())
	prc, _ := util.NewDecimalFromString(price.String())
	
	order := engine.Order{
		ID:     fmt.Sprintf("%d", time.Now().UnixNano()), 
		Type:   side, 
		Amount: amt,
		Price:  prc,
	}
	
	e.mu.Lock()
	pairBook, ok := e.book[symbol]
	if !ok {
		pairBook = engine.NewOrderBook(nil)
		e.book[symbol] = pairBook
	}
	e.mu.Unlock()
	
	start := time.Now()
	ordersProcessed, _ := pairBook.Process(order)
	IncProcess(start, len(ordersProcessed))
}

// ProcessMarketOrder 处理市价单 (Internal Direct Call for RDMA)
func (e *Engine) ProcessMarketOrder(side engine.Side, symbol string, amount decimal.Decimal) {
	amt, _ := util.NewDecimalFromString(amount.String())

	order := engine.Order{
		ID:     fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:   side, 
		Amount: amt,
	}

	e.mu.Lock()
	pairBook, ok := e.book[symbol]
	if !ok {
		pairBook = engine.NewOrderBook(nil)
		e.book[symbol] = pairBook
	}
	e.mu.Unlock()

	start := time.Now()
	ordersProcessed, _ := pairBook.ProcessMarket(order)
	IncProcessMarket(start, len(ordersProcessed))
}
