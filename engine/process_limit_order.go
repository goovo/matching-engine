package engine

import (
	// "fmt"

	"github.com/goovo/binarytree"
	"github.com/goovo/matching-engine/util"
)

var decimalZero, _ = util.NewDecimalFromString("0.0")

// Process 执行限价单撮合流程
func (ob *OrderBook) Process(order Order) ([]*Order, *Order) {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	if order.Type == Buy {
		// return ob.processOrderB(order)
		return ob.commonProcess(order, ob.SellTree, ob.addBuyOrder, ob.removeSellNode)
	}
	// return ob.processOrderS(order)
	return ob.commonProcess(order, ob.BuyTree, ob.addSellOrder, ob.removeBuyNode)
}

func (ob *OrderBook) commonProcess(order Order, tree *binarytree.BinaryTree, add func(Order), remove func(float64) error) ([]*Order, *Order) {
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	if maxNode == nil {
		// fmt.Println("adding node pending", order.Price)
		add(order)
		return nil, nil
	}
	// fmt.Println("maxNode", maxNode.Key, maxNode.Data.(*OrderType).Tree.Root.Key)
	count := 0
	noMoreOrders := false
	var allOrdersProcessed []*Order
	var partialOrder *Order
	orderOriginalAmount := order.Amount.Clone() // Clone
	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		count++
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		if maxNode == nil || noMoreOrders {
			if order.Amount.Cmp(decimalZero) == 1 {
				// fmt.Println("adding sell node pending")
				add(order)
				break
			} else {
				break
			}
		}
		
		var ordersProcessed []*Order
		noMoreOrders, ordersProcessed, partialOrder = ob.processLimit(&order, partialOrder, maxNode.Data.(*OrderType).Tree, orderOriginalAmount) //, orderPrice)
		
		allOrdersProcessed = append(allOrdersProcessed, ordersProcessed...)

		if maxNode.Data.(*OrderType).Tree.Root == nil {
			remove(maxNode.Key)
		}
	}

	return allOrdersProcessed, partialOrder
}

func (ob *OrderBook) processLimit(order, partialOrder *Order, tree *binarytree.BinaryTree, orderOriginalAmount *util.StandardBigDecimal) (bool, []*Order, *Order) {
	orderPrice := order.Price.Float64()
	var maxNode *binarytree.BinaryNode
	if order.Type == Sell {
		maxNode = tree.Max()
	} else {
		maxNode = tree.Min()
	}
	noMoreOrders := false
	var ordersProcessed []*Order
	
	if maxNode == nil {
		return noMoreOrders, nil, nil
	}
	
	for maxNode == nil || order.Amount.Cmp(decimalZero) == 1 {
		if order.Type == Sell {
			maxNode = tree.Max()
		} else {
			maxNode = tree.Min()
		}
		
		if maxNode == nil || noMoreOrders {
			if order.Amount.Cmp(decimalZero) == 1 {
				partialOrder = NewOrder(order.ID, order.Type, order.Amount, order.Price)
				break
			} else {
				break
			}
		}
		if order.Type == Sell {
			if orderPrice > maxNode.Key {
				noMoreOrders = true
				return noMoreOrders, ordersProcessed, partialOrder
			}
		} else {
			if orderPrice < maxNode.Key {
				noMoreOrders = true
				return noMoreOrders, ordersProcessed, partialOrder
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
				// 使用原地修改
				nodeData.Volume.SubMut(order.Amount)

				ele.Amount.SubMut(order.Amount)

				partialOrder = NewOrder(ele.ID, ele.Type, ele.Amount, ele.Price)
				ordersProcessed = append(ordersProcessed, NewOrder(order.ID, order.Type, orderOriginalAmount, order.Price))

				order.Amount.SetZero() // 优化：原地置零
				
				noMoreOrders = true
				break
			} else if ele.Amount.Cmp(order.Amount) == 0 {
				// 必须先创建副本，因为 removeOrder 后 ele 指针可能失效或指向被复用的内存
				ordersProcessed = append(ordersProcessed, NewOrder(ele.ID, ele.Type, ele.Amount, ele.Price))
				ordersProcessed = append(ordersProcessed, NewOrder(order.ID, order.Type, orderOriginalAmount, order.Price))
				partialOrder = nil

				// 先删除 map 索引
				delete(ob.orders, ele.ID)
				
				// 再从链表移除 (会调用 Arena.Free)
				nodeData.removeOrder(ob.Arena, currIdx)
				
				order.Amount.SetZero()
				
				currIdx = nextIdx // Move to next
				break
			} else {
				ordersProcessed = append(ordersProcessed, NewOrder(ele.ID, ele.Type, ele.Amount, ele.Price))

				order.Amount.SubMut(ele.Amount)
				
				partialOrder = NewOrder(order.ID, order.Type, order.Amount, order.Price)

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
	return noMoreOrders, ordersProcessed, partialOrder
}
