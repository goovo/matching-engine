package engine

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"sync"

	"github.com/goovo/binarytree"
	"github.com/goovo/matching-engine/util"
)

// OrderBook 订单簿类型
type OrderBook struct {
	BuyTree         *binarytree.BinaryTree
	SellTree        *binarytree.BinaryTree
	orderLimitRange int
	orders          map[string]IndexType // orderID -> Arena Index
	Arena           *OrderArena          // 内存管理器
	mutex           *sync.Mutex
	listener        MatchingListener     // 事件回调接口
}

// Book 订单簿序列化结构
type Book struct {
	Buys  []orderinfo `json:"buys"`
	Sells []orderinfo `json:"sells"`
}

type orderinfo struct {
	Price  *util.StandardBigDecimal `json:"price"`
	Amount *util.StandardBigDecimal `json:"amount"`
}

// MarshalJSON 实现 json.Marshaler 接口
func (ob *OrderBook) MarshalJSON() ([]byte, error) {
	buys := []orderinfo{}
	ob.BuyTree.Root.InOrderTraverse(func(i float64) {
		node := ob.BuyTree.Root.SearchSubTree(i)
		node.Data.(*OrderType).Tree.Root.InOrderTraverse(func(i float64) {
			var b orderinfo
			b.Price = util.NewDecimalFromFloat(i)
			subNode := node.Data.(*OrderType).Tree.Root.SearchSubTree(i)
			b.Amount = subNode.Data.(*OrderNode).Volume
			buys = append(buys, b)
		})
	})

	sells := []orderinfo{}
	ob.SellTree.Root.InOrderTraverse(func(i float64) {
		node := ob.SellTree.Root.SearchSubTree(i)
		node.Data.(*OrderType).Tree.Root.InOrderTraverse(func(i float64) {
			var b orderinfo
			b.Price = util.NewDecimalFromFloat(i)
			subNode := node.Data.(*OrderType).Tree.Root.SearchSubTree(i)
			b.Amount = subNode.Data.(*OrderNode).Volume
			sells = append(sells, b)
		})
	})

	return json.Marshal(
		&Book{
			Buys:  buys,
			Sells: sells,
		},
	)
}

// BookArray 订单簿二维数组结构
type BookArray struct {
	Buys  [][]string `json:"buys"`
	Sells [][]string `json:"sells"`
}

// GetOrders 返回价格与数量的二维数组（买盘倒序、卖盘正序）
func (ob *OrderBook) GetOrders(limit int64) *BookArray {
	ob.mutex.Lock()
	defer ob.mutex.Unlock()

	buys := [][]string{}
	ob.BuyTree.Root.InReverseOrderTraverse(func(i float64) {
		node := ob.BuyTree.Root.SearchSubTree(i)
		node.Data.(*OrderType).Tree.Root.InReverseOrderTraverse(func(i float64) {
			if int64(len(buys)) >= limit && limit != 0 {
				return
			}
			var b []string
			subNode := node.Data.(*OrderType).Tree.Root.SearchSubTree(i)
			if subNode != nil {
				price := strconv.FormatFloat(i, 'f', -1, 64)
				b = append(b, price)

				amount := subNode.Data.(*OrderNode).Volume
				b = append(b, amount.String())
				buys = append(buys, b)
			}
		})
	})

	sells := [][]string{}
	ob.SellTree.Root.InOrderTraverse(func(i float64) {
		node := ob.SellTree.Root.SearchSubTree(i)
		node.Data.(*OrderType).Tree.Root.InOrderTraverse(func(i float64) {
			if int64(len(sells)) >= limit && limit != 0 {
				return
			}
			var b []string
			subNode := node.Data.(*OrderType).Tree.Root.SearchSubTree(i)
			if subNode != nil {
				price := strconv.FormatFloat(i, 'f', -1, 64)
				b = append(b, price)

				amount := subNode.Data.(*OrderNode).Volume
				b = append(b, amount.String())
				sells = append(sells, b)
			}
		})
	})

	return &BookArray{
		Buys:  buys,
		Sells: sells,
	}
}

// String 实现 Stringer 接口
func (ob *OrderBook) String() string {
	result := ""
	var orderSideSell []string
	ob.SellTree.Root.InOrderTraverse(func(i float64) {
		node := ob.SellTree.Root.SearchSubTree(i)
		node.Data.(*OrderType).Tree.Root.InOrderTraverse(func(i float64) {
			res := strconv.FormatFloat(i, 'f', -1, 64) + " -> "
			subNode := node.Data.(*OrderType).Tree.Root.SearchSubTree(i)
			if subNode != nil {
				vol := subNode.Data.(*OrderNode).Volume.Float64()
				res += strconv.FormatFloat(vol, 'f', -1, 64) 
				orderSideSell = append(orderSideSell, res)
			}
		})
	})
	sells := ""
	for _, o := range orderSideSell {
		sells = o + "\n" + sells
	}
	result = sells + "------------------------------------------\n"

	var orderSideBuy []string
	ob.BuyTree.Root.InOrderTraverse(func(i float64) {
		node := ob.BuyTree.Root.SearchSubTree(i)
		node.Data.(*OrderType).Tree.Root.InOrderTraverse(func(i float64) {
			res := strconv.FormatFloat(i, 'f', -1, 64) + " -> "
			subNode := node.Data.(*OrderType).Tree.Root.SearchSubTree(i)
			if subNode != nil {
				vol := subNode.Data.(*OrderNode).Volume.Float64()
				res += strconv.FormatFloat(vol, 'f', -1, 64)
				orderSideBuy = append(orderSideBuy, res)
			}
		})
	})
	buys := ""
	for _, o := range orderSideBuy {
		buys = o + "\n" + buys
	}
	result += buys
	return result
}

// NewOrderBook 返回新的订单簿
// listener: 事件回调接口，如果为 nil 则使用 NoOpListener
func NewOrderBook(listener MatchingListener) *OrderBook {
	bTree := binarytree.NewBinaryTree()
	sTree := binarytree.NewBinaryTree()
	bTree.ToggleSplay(true)
	sTree.ToggleSplay(true)

	if listener == nil {
		listener = &NoOpListener{}
	}

	return &OrderBook{
		BuyTree:         bTree,
		SellTree:        sTree,
		orderLimitRange: 200000000,
		orders:          make(map[string]IndexType),
		Arena:           NewOrderArena(100000), // 默认 10w 容量
		mutex:           &sync.Mutex{},
		listener:        listener,
	}
}

// addBuyOrder 将买单加入订单簿
func (ob *OrderBook) addBuyOrder(order Order) {
	// 分配 Arena 空间
	idx := ob.Arena.Alloc()
	storedOrder := ob.Arena.Get(idx)
	*storedOrder = order // Copy struct
	storedOrder.Next = NullIndex
	storedOrder.Prev = NullIndex
	storedOrder.Node = nil

	orderPrice := order.Price.Float64()
	startPoint := float64(int(math.Ceil(orderPrice)) / ob.orderLimitRange * ob.orderLimitRange)
	endPoint := startPoint + float64(ob.orderLimitRange)
	searchNodePrice := (startPoint + endPoint) / 2
	
	node := ob.BuyTree.Root.SearchSubTree(searchNodePrice)
	if node != nil {
		subTree := node.Data.(*OrderType)
		subTreeNode := subTree.Tree.Root.SearchSubTree(orderPrice)
		if subTreeNode != nil {
			subTreeNode.Data.(*OrderNode).addOrder(ob.Arena, idx)
		} else {
			_, _ = subTree.AddOrderInQueue(ob.Arena, idx)
		}
	} else {
		orderTypeObj := NewOrderType(order.Type)
		_, _ = orderTypeObj.AddOrderInQueue(ob.Arena, idx)
		ob.BuyTree.Insert(searchNodePrice, orderTypeObj)
	}
	ob.orders[order.ID] = idx
	
	// 触发 Maker 事件
	ob.listener.OnOrderAccepted(order.ID)
}

// addSellOrder 将卖单加入订单簿
func (ob *OrderBook) addSellOrder(order Order) {
	// 分配 Arena 空间
	idx := ob.Arena.Alloc()
	storedOrder := ob.Arena.Get(idx)
	*storedOrder = order // Copy struct
	storedOrder.Next = NullIndex
	storedOrder.Prev = NullIndex
	storedOrder.Node = nil

	orderPrice := order.Price.Float64()
	startPoint := float64(int(math.Ceil(orderPrice)) / ob.orderLimitRange * ob.orderLimitRange)
	endPoint := startPoint + float64(ob.orderLimitRange)
	searchNodePrice := (startPoint + endPoint) / 2
	
	node := ob.SellTree.Root.SearchSubTree(searchNodePrice)
	if node != nil {
		subTree := node.Data.(*OrderType)
		subTreeNode := subTree.Tree.Root.SearchSubTree(orderPrice)
		if subTreeNode != nil {
			subTreeNode.Data.(*OrderNode).addOrder(ob.Arena, idx)
		} else {
			_, _ = subTree.AddOrderInQueue(ob.Arena, idx)
		}
	} else {
		orderTypeObj := NewOrderType(order.Type)
		_, _ = orderTypeObj.AddOrderInQueue(ob.Arena, idx)
		ob.SellTree.Insert(searchNodePrice, orderTypeObj)
	}
	ob.orders[order.ID] = idx

	// 触发 Maker 事件
	ob.listener.OnOrderAccepted(order.ID)
}

func (ob *OrderBook) removeBuyNode(key float64) error {
	node := ob.BuyTree.Root.Remove(key)
	ob.BuyTree.Root = node
	return nil
}

func (ob *OrderBook) removeSellNode(key float64) error {
	node := ob.SellTree.Root.Remove(key)
	ob.SellTree.Root = node
	return nil
}

// removeOrder 从订单簿移除订单
func (ob *OrderBook) removeOrder(order *Order) error {
	orderPrice := order.Price.Float64()
	startPoint := float64(int(math.Ceil(orderPrice)) / ob.orderLimitRange * ob.orderLimitRange)
	endPoint := startPoint + float64(ob.orderLimitRange)
	searchNodePrice := (startPoint + endPoint) / 2
	
	var node *binarytree.BinaryNode
	if order.Type == Buy {
		node = ob.BuyTree.Root.SearchSubTree(searchNodePrice)
	} else {
		node = ob.SellTree.Root.SearchSubTree(searchNodePrice)
	}
	if node != nil {
		subTree := node.Data.(*OrderType)
		subTreeNode := subTree.Tree.Root.SearchSubTree(orderPrice)
		if subTreeNode != nil {
			if subTreeNode.Data.(*OrderNode).Count == 0 {
				n := subTree.Tree.Root.Remove(orderPrice)
				subTree.Tree.Root = n
			}
		} else {
			return errors.New("no Order found")
		}
	} else {
		return errors.New("no Order found")
	}
	return nil
}
