package engine

import (
	"github.com/goovo/binarytree"
)

// ProcessMarket 执行市价单撮合流程
func (ob *OrderBook) ProcessMarket(order Order) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	if order.Type == Buy {
		ob.commonProcessMarket(order, ob.SellTree, ob.addBuyOrder, ob.removeSellNode)
	} else {
		ob.commonProcessMarket(order, ob.BuyTree, ob.addSellOrder, ob.removeBuyNode)
	}
}

func (ob *OrderBook) commonProcessMarket(order Order, tree *binarytree.BinaryTree, add func(Order), remove func(float64) error) {
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	if maxNode == nil {
		// 市价单如果不匹配，直接丢弃或取消（IOC/FOK）
		// 这里假设是 IOC (Immediate or Cancel)，未成交部分取消
		if order.ID != "" {
			// 触发取消事件（剩余全部取消）
			ob.listener.OnOrderCancelled(order.ID)
		}
		return
	}
	count := 0
	noMoreOrders := false
	
	// orderOriginalAmount := order.Amount.Clone()

	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		count++
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		if maxNode == nil || noMoreOrders {
			if order.Amount.Cmp(decimalZero) == 1 {
				// 市价单未完全成交，剩余部分取消
				ob.listener.OnOrderCancelled(order.ID)
			}
			break
		}

		noMoreOrders = ob.processLimitMarket(&order, maxNode.Data.(*OrderType).Tree)

		if maxNode.Data.(*OrderType).Tree.Root == nil {
			remove(maxNode.Key)
		}
	}
}

func (ob *OrderBook) processLimitMarket(order *Order, tree *binarytree.BinaryTree) bool {
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	noMoreOrders := false
	
	if maxNode == nil {
		return noMoreOrders
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
			nextIdx := ele.Next // Save next
			
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
	return noMoreOrders
}
