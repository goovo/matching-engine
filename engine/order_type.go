package engine

import (
	"errors"

	"github.com/goovo/binarytree"
)

// OrderType 定义价格树的方向（买/卖）
type OrderType struct {
	Tree *binarytree.BinaryTree
	Type Side `json:"type"`
}

// NewOrderType 返回 OrderType 结构体
func NewOrderType(orderSide Side) *OrderType {
	bTree := binarytree.NewBinaryTree()
	bTree.ToggleSplay(true)
	return &OrderType{Tree: bTree, Type: orderSide}
}

// AddOrderInQueue 将订单加入该方向的价格树队列
func (ot *OrderType) AddOrderInQueue(arena *OrderArena, orderIdx IndexType) (*OrderNode, error) {
	order := arena.Get(orderIdx)
	if ot.Type != order.Type {
		return nil, errors.New("invalid order type")
	}
	orderNode := NewOrderNode()
	orderNode.addOrder(arena, orderIdx)
	
	orderPrice := order.Price.Float64()
	ot.Tree.Insert(orderPrice, orderNode)
	return orderNode, nil
}
