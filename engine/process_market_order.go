package engine

import (
	"github.com/goovo/binarytree"
	"github.com/goovo/matching-engine/util"
)

// ProcessMarket 执行市价单撮合流程
func (ob *OrderBook) ProcessMarket(order Order) ([]*Order, *Order) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	if order.Type == Buy {
		// return ob.processOrderB(order)
		return ob.commonProcessMarket(order, ob.SellTree, ob.addBuyOrder, ob.removeSellNode)
	}
	// return ob.processOrderS(order)
	return ob.commonProcessMarket(order, ob.BuyTree, ob.addSellOrder, ob.removeBuyNode)
}

func (ob *OrderBook) commonProcessMarket(order Order, tree *binarytree.BinaryTree, add func(Order), remove func(float64) error) ([]*Order, *Order) {
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	if maxNode == nil {
		// add(order)
		return nil, nil
	}
	count := 0
	noMoreOrders := false
	var allOrdersProcessed []*Order
	var partialOrder *Order
	orderOriginalAmount := order.Amount.Clone()
	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		count++
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		if maxNode == nil || noMoreOrders {
			if order.Amount.Cmp(decimalZero) == 1 {
				allOrdersProcessed = append(allOrdersProcessed, NewOrder(order.ID, order.Type, orderOriginalAmount, decimalZero))
			}
			break
		}

		// var t []Trade
		var ordersProcessed []*Order
		noMoreOrders, ordersProcessed, partialOrder = ob.processLimitMarket(&order, maxNode.Data.(*OrderType).Tree, orderOriginalAmount) //, orderPrice)
		allOrdersProcessed = append(allOrdersProcessed, ordersProcessed...)
		// trades = append(trades, t...)

		if maxNode.Data.(*OrderType).Tree.Root == nil {
			// node := remove(maxNode.Key)
			// // node := ob.removeBuyNode(maxNode.Key)
			// tree.Root = node
			remove(maxNode.Key)
		}
	}

	// return trades, allOrdersProcessed, partialOrder
	return allOrdersProcessed, partialOrder
}

func (ob *OrderBook) processLimitMarket(order *Order, tree *binarytree.BinaryTree, orderOriginalAmount *util.StandardBigDecimal) (bool, []*Order, *Order) {
	// orderPrice, _ := order.Price.Float64()
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	noMoreOrders := false
	var ordersProcessed []*Order
	var partialOrder *Order
	if maxNode == nil {
		// return trades, noMoreOrders, nil, nil
		return noMoreOrders, nil, nil
	}
	// countAdd := 0.0
	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		if maxNode == nil || noMoreOrders {
			break
		}
		
		nodeData := maxNode.Data.(*OrderNode) //([]*Order)
		currIdx := nodeData.Head
		
		for currIdx != NullIndex {
			ele := ob.Arena.Get(currIdx)
			nextIdx := ele.Next // Save next
			
			if ele.Amount.Cmp(order.Amount) == 1 {
				// 使用原地修改
				nodeData.Volume.SubMut(order.Amount)

				ele.Amount.SubMut(order.Amount)

				partialOrder = NewOrder(ele.ID, ele.Type, ele.Amount, ele.Price)
				ordersProcessed = append(ordersProcessed, NewOrder(order.ID, order.Type, orderOriginalAmount, decimalZero))

				order.Amount.SetZero()
				noMoreOrders = true
				break
			}
			if ele.Amount.Cmp(order.Amount) == 0 {
				ordersProcessed = append(ordersProcessed, NewOrder(ele.ID, ele.Type, ele.Amount, ele.Price))
				ordersProcessed = append(ordersProcessed, NewOrder(order.ID, order.Type, orderOriginalAmount, decimalZero))

				order.Amount.SetZero()
				
				delete(ob.orders, ele.ID)
				
				nodeData.removeOrder(ob.Arena, currIdx)

				currIdx = nextIdx
				break
			} else {
				ordersProcessed = append(ordersProcessed, NewOrder(ele.ID, ele.Type, ele.Amount, ele.Price))

				order.Amount.SubMut(ele.Amount)
				
				delete(ob.orders, ele.ID)
				
				nodeData.removeOrder(ob.Arena, currIdx)
			}
			currIdx = nextIdx
		}

		if nodeData.Count == 0 {
			node := tree.Root.Remove(maxNode.Key) 
			tree.Root = node
			nodeData.Release()
		}
	}
	return noMoreOrders, ordersProcessed, partialOrder
}
