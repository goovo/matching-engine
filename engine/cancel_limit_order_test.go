package engine

import (
	"testing"
)

func TestCancelOrder(t *testing.T) {
	var tests = []struct {
		input *Order
	}{
		{NewOrder("b1", Buy, DecimalBig("5.0"), DecimalBig("7000.0"))},
		{NewOrder("b2", Buy, DecimalBig("10.0"), DecimalBig("6000.0"))},
		{NewOrder("b3", Buy, DecimalBig("11.0"), DecimalBig("7000.0"))},
		{NewOrder("b4", Buy, DecimalBig("1.0"), DecimalBig("7000.0"))},
		{NewOrder("s1", Sell, DecimalBig("5.0"), DecimalBig("8000.0"))},
		{NewOrder("s2", Sell, DecimalBig("10.0"), DecimalBig("9000.0"))},
		{NewOrder("s3", Sell, DecimalBig("11.0"), DecimalBig("9000.0"))},
		{NewOrder("s4", Sell, DecimalBig("1.0"), DecimalBig("7500.0"))},
	}
	ob := NewOrderBook()

	for _, tt := range tests {
		if tt.input.Type == Buy {
			ob.addBuyOrder(*tt.input)
		} else {
			ob.addSellOrder(*tt.input)
		}
	}

	idx := ob.orders[tests[4].input.ID]
	orderInArena := ob.Arena.Get(idx)
	on := orderInArena.Node

	order := ob.CancelOrder("s1")

	currIdx := on.Head
	found := false
	for currIdx != NullIndex {
		currOrder := ob.Arena.Get(currIdx)
		if currOrder.ID == tests[4].input.ID {
			found = true
			break
		}
		currIdx = currOrder.Next
	}

	if found {
		t.Fatal("Order is not removed from the OrderNode")
	}

	if order == nil {
		t.Fatal("Order is not removed")
	}

	err := ob.removeOrder(order)
	if err == nil {
		t.Fatal("Order is not removed from Tree of Orderbook")
	}

	if _, ok := ob.orders[order.ID]; ok {
		t.Fatal("Order is not removed from \"orders\" of Orderbook")
	}
}
