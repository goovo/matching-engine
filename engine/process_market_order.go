package engine

import (
"github.com/goovo/binarytree"
)

// ProcessMarket 执行市价单撮合流程
func (ob *OrderBook) ProcessMarket(order Order) ([]Order, *Order) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()
	
	var processed []Order
	var partial *Order

	if order.Type == Buy {
		processed, partial = ob.commonProcessMarket(order, ob.SellTree, ob.addBuyOrder, ob.removeSellNode)
	} else {
		processed, partial = ob.commonProcessMarket(order, ob.BuyTree, ob.addSellOrder, ob.removeBuyNode)
	}
	return processed, partial
}

func (ob *OrderBook) commonProcessMarket(order Order, tree *binarytree.BinaryTree, add func(Order), remove func(float64) error) ([]Order, *Order) {
	var maxNode *binarytree.BinaryNode
	var processed []Order
	
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	if maxNode == nil {
		if order.ID != "" {
			ob.listener.OnOrderCancelled(order.ID)
		}
		return processed, nil
	}
	noMoreOrders := false
	
	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		if maxNode == nil || noMoreOrders {
			if order.Amount.Cmp(decimalZero) == 1 {
				ob.listener.OnOrderCancelled(order.ID)
				return processed, &order 
			}
			break
		}

		var subProcessed []Order
		noMoreOrders, subProcessed = ob.processLimitMarket(&order, maxNode.Data.(*OrderType).Tree)
		processed = append(processed, subProcessed...)

		if maxNode.Data.(*OrderType).Tree.Root == nil {
			remove(maxNode.Key)
		}
	}
	return processed, nil
}

func (ob *OrderBook) processLimitMarket(order *Order, tree *binarytree.BinaryTree) (bool, []Order) {
	var maxNode *binarytree.BinaryNode
	var processed []Order
	
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	noMoreOrders := false
	
	if maxNode == nil {
		return noMoreOrders, processed
	}
	
	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		if maxNode == nil || noMoreOrders {
			break
		}
		
		nodeData := maxNode.Data.(*OrderNode)
		currIdx := nodeData.Head
		
		for currIdx != NullIndex {
			ele := ob.Arena.Get(currIdx)
			nextIdx := ele.Next 
			
			if ele.Amount.Cmp(order.Amount) == 1 {
				// Case 1: Maker > Taker
				nodeData.Volume.SubMut(order.Amount)
				ele.Amount.SubMut(order.Amount)

				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, order.Amount.Val)

				order.Amount.SetZero()
				noMoreOrders = true
				break
			}
			if ele.Amount.Cmp(order.Amount) == 0 {
				// Case 2: Maker == Taker
				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, ele.Amount.Val)

				order.Amount.SetZero()
				delete(ob.orders, ele.ID)
				nodeData.removeOrder(ob.Arena, currIdx)
				currIdx = nextIdx
				break
			} else {
				// Case 3: Maker < Taker
				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, ele.Amount.Val)

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
	return noMoreOrders, processed
}
