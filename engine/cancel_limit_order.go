package engine

// CancelOrder 从订单簿移除该订单并返回被移除的订单
func (ob *OrderBook) CancelOrder(id string) *Order {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	idx, ok := ob.orders[id]
	if !ok {
		return nil
	}

	orderInArena := ob.Arena.Get(idx)
	// 创建副本返回
	retOrder := NewOrder(orderInArena.ID, orderInArena.Type, orderInArena.Amount.Clone(), orderInArena.Price.Clone())

	if orderInArena.Node != nil {
		node := orderInArena.Node
		// 注意：removeOrder 会调用 Arena.Free(idx)，但数据在当前锁范围内依然可读（尚未被覆盖）
		// 不过为了更安全，应该先从 Tree 移除（如果需要），再从 Node 移除？
		// 不，只有 Node 空了才从 Tree 移除。
		// 先获取必要信息
		// nodeCount := node.Count - 1 // 预测
		// node.removeOrder(ob.Arena, idx)
		
		// 或者：
		node.removeOrder(ob.Arena, idx)
		if node.Count == 0 {
			ob.removeOrder(orderInArena)
		}
	}

	delete(ob.orders, id)
	return retOrder
}
