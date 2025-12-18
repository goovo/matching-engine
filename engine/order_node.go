package engine

import (
	"encoding/json"
	"sync"

	"github.com/goovo/matching-engine/util"
)

// OrderNode 价格节点，包含订单队列与聚合成交量
type OrderNode struct {
	Head   IndexType                `json:"-"`
	Tail   IndexType                `json:"-"`
	Count  int                      `json:"count"`
	Volume *util.StandardBigDecimal `json:"volume"`
}

var orderNodePool = sync.Pool{
	New: func() interface{} {
		return &OrderNode{}
	},
}

// GetOrderNode 从池中获取 OrderNode 对象
func GetOrderNode() *OrderNode {
	return orderNodePool.Get().(*OrderNode)
}

// Release 将 OrderNode 对象放回池中
func (on *OrderNode) Release() {
	// StandardBigDecimal 无需 Release
	on.Head = NullIndex
	on.Tail = NullIndex
	on.Count = 0
	on.Volume = nil
	orderNodePool.Put(on)
}

// NewOrderNode 返回新的 OrderNode 结构体
func NewOrderNode() *OrderNode {
	on := GetOrderNode()
	on.Head = NullIndex
	on.Tail = NullIndex
	on.Count = 0
	// 确保 Volume 也是从池中获取 (现在不需要了，直接创建)
	on.Volume = util.NewDecimalFromFloat(0.0)
	return on
}

// addOrder 将订单加入节点（链表尾部追加）
func (on *OrderNode) addOrder(arena *OrderArena, orderIdx IndexType) {
	order := arena.Get(orderIdx)
	order.Node = on
	
	// 使用原地更新
	on.Volume.AddMut(order.Amount)

	if on.Tail == NullIndex {
		on.Head = orderIdx
		on.Tail = orderIdx
		order.Prev = NullIndex
		order.Next = NullIndex
	} else {
		// on.Tail.Next = orderIdx
		tailOrder := arena.Get(on.Tail)
		tailOrder.Next = orderIdx
		
		order.Prev = on.Tail
		order.Next = NullIndex
		on.Tail = orderIdx
	}
	on.Count++
}

// updateVolume 更新节点聚合成交量
func (on *OrderNode) updateVolume(value *util.StandardBigDecimal) {
	on.Volume.AddMut(value)
}

// removeOrder 从节点中移除指定订单（O(1) 操作）
func (on *OrderNode) removeOrder(arena *OrderArena, orderIdx IndexType) {
	order := arena.Get(orderIdx)
	
	// 直接减去订单金额，避免创建负数临时对象
	on.Volume.SubMut(order.Amount)

	if order.Prev != NullIndex {
		// order.prev.next = order.next
		prevOrder := arena.Get(order.Prev)
		prevOrder.Next = order.Next
	} else {
		on.Head = order.Next
	}

	if order.Next != NullIndex {
		// order.next.prev = order.prev
		nextOrder := arena.Get(order.Next)
		nextOrder.Prev = order.Prev
	} else {
		on.Tail = order.Prev
	}

	order.Prev = NullIndex
	order.Next = NullIndex
	order.Node = nil
	on.Count--
	
	// 回收 Index 到 Arena
	arena.Free(orderIdx)
}

// ToJSONWithArena 辅助序列化方法
func (on *OrderNode) ToJSONWithArena(arena *OrderArena) ([]byte, error) {
	orders := make([]*Order, 0, on.Count)
	currIdx := on.Head
	for currIdx != NullIndex {
		orders = append(orders, arena.Get(currIdx))
		currIdx = arena.Get(currIdx).Next
	}

	type Alias OrderNode
	return json.Marshal(&struct {
		Orders []*Order `json:"orders"`
		*Alias
	}{
		Orders: orders,
		Alias:  (*Alias)(on),
	})
}
