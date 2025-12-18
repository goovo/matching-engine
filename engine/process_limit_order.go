package engine

import (
	// "fmt"

	"github.com/goovo/binarytree"
	"github.com/goovo/matching-engine/util"
)

var decimalZero, _ = util.NewDecimalFromString("0.0")

// Process 执行限价单撮合流程
func (ob *OrderBook) Process(order Order) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	if order.Type == Buy {
		// return ob.processOrderB(order)
		ob.commonProcess(order, ob.SellTree, ob.addBuyOrder, ob.removeSellNode)
	} else {
		// return ob.processOrderS(order)
		ob.commonProcess(order, ob.BuyTree, ob.addSellOrder, ob.removeBuyNode)
	}
}

func (ob *OrderBook) commonProcess(order Order, tree *binarytree.BinaryTree, add func(Order), remove func(float64) error) {
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	if maxNode == nil {
		add(order)
		return
	}

	count := 0
	noMoreOrders := false
	
	// 不需要 Clone 了，因为不需要返回 partialOrder
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
				add(order)
				break
			} else {
				break
			}
		}
		
		noMoreOrders = ob.processLimit(&order, maxNode.Data.(*OrderType).Tree)
		
		if maxNode.Data.(*OrderType).Tree.Root == nil {
			remove(maxNode.Key)
		}
	}
}

func (ob *OrderBook) processLimit(order *Order, tree *binarytree.BinaryTree) bool {
	orderPrice := order.Price.Float64()
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
			// 这里不需要处理 break，外层循环会处理
			break
		}
		if order.Type == Sell {
			if orderPrice > maxNode.Key {
				noMoreOrders = true
				return noMoreOrders
			}
		} else {
			if orderPrice < maxNode.Key {
				noMoreOrders = true
				return noMoreOrders
			}
		}

		nodeData := maxNode.Data.(*OrderNode)
		currIdx := nodeData.Head

		for currIdx != NullIndex {
			ele := ob.Arena.Get(currIdx)
			nextIdx := ele.Next // Save next
			
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
				// Case 1: Maker 量 > Taker 量 (部分成交)
				// 使用原地修改
				nodeData.Volume.SubMut(order.Amount)
				ele.Amount.SubMut(order.Amount)

				// 触发成交事件
				// Maker: ele, Taker: order
				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, order.Amount.Val)

				order.Amount.SetZero() // 优化：原地置零
				
				noMoreOrders = true
				break
			} else if ele.Amount.Cmp(order.Amount) == 0 {
				// Case 2: Maker 量 == Taker 量 (完全成交)
				
				// 触发成交事件
				ob.listener.OnTrade(ele.ID, order.ID, ele.Type, ele.Price.Val, ele.Amount.Val)

				// 先删除 map 索引
				delete(ob.orders, ele.ID)
				
				// 再从链表移除 (会调用 Arena.Free)
				nodeData.removeOrder(ob.Arena, currIdx)
				
				order.Amount.SetZero()
				
				currIdx = nextIdx // Move to next
				break
			} else {
				// Case 3: Maker 量 < Taker 量 (Maker 吃光，Taker 还有剩)
				
				// 触发成交事件
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
			nodeData.Release() // 回收空的 OrderNode
		}
	}
	return noMoreOrders
}
