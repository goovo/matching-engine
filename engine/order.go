package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
 
	"github.com/goovo/matching-engine/util"
)

// Order 描述订单的结构体
type Order struct {
	Amount *util.StandardBigDecimal `json:"amount"` // validate:"gt=0"`
	Price  *util.StandardBigDecimal `json:"price"`  // validate:"gt=0"`
	ID     string                   `json:"id"`     // validate:"required"`
	Type   Side                     `json:"type"`   //  validate:"side_validate"`
}

// func sideValidation(fl validator.FieldLevel) bool {
// 	if fl.Field().Interface() != Buy && fl.Field().Interface() != Sell {
// 		return false
// 	}
// 	return true
// }

// NewOrder 返回 *Order
func NewOrder(id string, orderType Side, amount, price *util.StandardBigDecimal) *Order {
	o := &Order{ID: id, Type: orderType, Amount: amount, Price: price}
	return o
}

// FromJSON 从 JSON 字符串创建 Order 结构体
func (order *Order) FromJSON(msg []byte) error {
	err := json.Unmarshal(msg, order)
	if err != nil {
		return err
	}
	return nil
}

// ToJSON 返回订单的 JSON 字符串
func (order *Order) ToJSON() ([]byte, error) {
	str, err := json.Marshal(order)
	return str, err
}

// String 实现 Stringer 接口
func (order *Order) String() string {
	amount := order.Amount.Float64()
	price := order.Price.Float64()

	return fmt.Sprintf("\"%s\":\n\tside: %v\n\tquantity: %s\n\tprice: %s\n", order.ID, order.Type, strconv.FormatFloat(amount, 'f', -1, 64), strconv.FormatFloat(price, 'f', -1, 64))
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (order *Order) UnmarshalJSON(data []byte) error {
	obj := struct {
		Type   Side   `json:"type"`   // validate:"side_validate"`
		ID     string `json:"id"`     // validate:"required"`
		Amount string `json:"amount"` // validate:"required"`
		Price  string `json:"price"`  // validate:"required"`
	}{}

	if err := json.Unmarshal(data, &obj); err != nil {
		fmt.Println("Damn errr", err)
		return err
	}

	if obj.ID == "" {
		return errors.New("ID is not present")
	}
	if obj.Type == "" {
		return errors.New("invalid order type")
	}

	var err error
	order.Price, err = util.NewDecimalFromString(obj.Price) //.Quantize(8)
	if err != nil {
		fmt.Println("price", order.Price, err.Error())
		return errors.New("invalid order price")
	}
	order.Amount, err = util.NewDecimalFromString(obj.Amount) //.Quantize(8)
	if err != nil {
		return errors.New("invalid order amount")
	}

	order.Type = obj.Type
	order.ID = obj.ID

	price := order.Price.Float64()
	if price <= 0 {
		return errors.New("Order price should be greater than zero")
	}
	amount := order.Amount.Float64()
	if amount <= 0 {
		return errors.New("Order amount should be greater than zero")
	}
	return nil
}

// MarshalJSON 实现 json.Marshaler 接口
func (order *Order) MarshalJSON() ([]byte, error) {
	// 保证整数格式的数量与价格在 JSON 中以 .0 结尾，满足序列化预期
	amount := order.Amount.String()
	if !strings.Contains(amount, ".") {
		amount = amount + ".0"
	}
	price := order.Price.String()
	if !strings.Contains(price, ".") {
		price = price + ".0"
	}
	return json.Marshal(
		&struct {
			Type   string `json:"type"`
			ID     string `json:"id"`
			Amount string `json:"amount"`
			Price  string `json:"price"`
		}{
			Type:   order.Type.String(),
			ID:     order.ID,
			Amount: amount,
			Price:  price,
		},
	)
}
