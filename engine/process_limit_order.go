package engine

import (
"github.com/goovo/binarytree"
"github.com/goovo/matching-engine/util"
)

var decimalZero, _ = util.NewDecimalFromString("0.0")

// Process 执行限价单撮合流程
func (ob *OrderBook) Process(order Order) ([]Order, *Order) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	var processed []Order
	var partial *Order

	if order.Type == Buy {
		processed, partial = ob.commonProcess(order, ob.SellTree, ob.addBuyOrder, ob.removeSellNode)
	} else {
		processed, partial = ob.commonProcess(order, ob.BuyTree, ob.addSellOrder, ob.removeBuyNode)
	}
	return processed, partial
}

func (ob *OrderBook) commonProcess(order Order, tree *binarytree.BinaryTree, add func(Order), remove func(float64) error) ([]Order, *Order) {
	var maxNode *binarytree.BinaryNode
	var processed []Order
	
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	if maxNode == nil {
		add(order)
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
				add(order)
				return processed, &order 
			} else {
				break
			}
		}
		
		var subProcessed []Order
		noMoreOrders, subProcessed = ob.processLimit(&order, maxNode.Data.(*OrderType).Tree)
		processed = append(processed, subProcessed...)
		
		if maxNode.Data.(*OrderType).Tree.Root == nil {
			remove(maxNode.Key)
		}
	}
	return processed, nil
}

func (ob *OrderBook) processLimit(order *Order, tree *binarytree.BinaryTree) (bool, []Order) {
	orderPrice := order.Price.Float64()
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
		if order.Type == Sell {
			if orderPrice > maxNode.Key {
				noMoreOrders = true
				return noMoreOrders, processed
			}
		} else {
			if orderPrice < maxNode.Key {
				noMoreOrders = true
				return noMoreOrders, processed
			}
		}

		nodeData := maxNode.Data.(*OrderNode)
		currIdx := nodeData.Head

		for currIdx != NullIndex {
			ele := ob.Arena.Get(currIdx)
			nextIdx := ele.Next 
			
			if order.Type == Sell {
				if ele.Price.Cmp(order.Price) == -1 {
					noMoreOrders = true
					break
				}
			} else {
				if ele.Price.Cmp(order.Price) == 1 {
					noMoreOrders = true
					break
				}
			}

			if ele.Amount.Cmp(order.Amount) == 1 {
				// Case 1: Maker > Taker (Partial)
				nodeData.Volume.SubMut(order.Amount)
				ele.Amount.SubMut(order.Amount)

				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, order.Amount.Val)
				
				order.Amount.SetZero()
				noMoreOrders = true
				break
			} else if ele.Amount.Cmp(order.Amount) == 0 {
				// Case 2: Full
				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, ele.Amount.Val)

				delete(ob.orders, ele.ID)
				nodeData.removeOrder(ob.Arena, currIdx)
				
				order.Amount.SetZero()
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
