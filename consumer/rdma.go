package consumer

import (
"encoding/binary"
"fmt"
"log"
"time"

"github.com/goovo/matching-engine/engine"
"github.com/goovo/matching-engine/pkg/queue"
"github.com/goovo/matching-engine/pkg/shm"
"github.com/goovo/matching-engine/server"
"github.com/shopspring/decimal"
)

const SHMPath = "/tmp/order_shm"
const SHMSize = 64 * 1024 * 1024 // 64MB

func StartRDMAConsumer(eng *server.Engine) {
	mf, err := shm.OpenOrCreate(SHMPath, SHMSize)
	if err != nil {
		log.Fatalf("Failed to open SHM: %v", err)
	}

	rb := queue.NewSHMRingBuffer(mf.Data, 1024*1024)

	fmt.Println("RDMA Consumer Started. Polling shared memory...")

	go func() {
		pollCount := 0
		for {
			slot := rb.Poll()
			if slot != nil {
				data := slot[:]
				
				orderID := binary.BigEndian.Uint64(data[8:])
				// marketIDHash := binary.BigEndian.Uint64(data[16:])
				priceRaw := binary.BigEndian.Uint64(data[24:])
				amountRaw := binary.BigEndian.Uint64(data[32:])
				flags := binary.BigEndian.Uint16(data[48:])
				
				side := engine.Buy
				if flags&1 != 0 {
					side = engine.Sell
				}
				
				isMarketOrder := false
				if flags&2 != 0 {
					isMarketOrder = true
				}
				
				price := decimal.NewFromInt(int64(priceRaw)).Div(decimal.NewFromInt(100000000))
				amount := decimal.NewFromInt(int64(amountRaw)).Div(decimal.NewFromInt(100000000))
				
				symbol := "ETH-USD" 
				
				fmt.Printf("RDMA Received: OID=%d Side=%v Price=%s Amt=%s\n", orderID, side, price, amount)
				
				if !isMarketOrder {
					eng.ProcessLimitOrder(side, symbol, amount, price)
				} else {
					eng.ProcessMarketOrder(side, symbol, amount)
				}

				slot.SetStatus(4) 
				
				pollCount = 0
			} else {
				pollCount++
				if pollCount > 1000 {
					time.Sleep(1 * time.Microsecond)
					pollCount = 0
				}
			}
		}
	}()
}
