package engine

import (
	"errors"
 
	"github.com/Pantelwar/binarytree"
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
func (ot *OrderType) AddOrderInQueue(order Order) (*OrderNode, error) {
	if ot.Type != order.Type {
		return nil, errors.New("invalid order type")
	}
	orderNode := NewOrderNode()
	orderNode.Orders = append(orderNode.Orders, &order)
	orderNode.Volume = order.Amount
	orderPrice := order.Price.Float64()
	ot.Tree.Insert(orderPrice, orderNode)
	return orderNode, nil
}
