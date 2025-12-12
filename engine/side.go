package engine

import (
	"encoding/json"
	"reflect"
)

// Side 订单方向
type Side string

// 卖单（asks）或买单（bids）
const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

// MarshalJSON 实现 json.Marshaler 接口
func (s Side) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON 实现 JSON 反序列化接口
func (s *Side) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"buy"`:
		*s = Buy
	case `"sell"`:
		*s = Sell
	default:
		return &json.UnsupportedValueError{
			Value: reflect.New(reflect.TypeOf(data)),
			Str:   string(data),
		}
	}

	return nil
}

// String 实现 Stringer 接口
func (s Side) String() string {
	if s == Buy {
		return "buy"
	} else if s == Sell {
		return "sell"
	}
	return ""
}
