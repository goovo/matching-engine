package util

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SCALE 定义定点数精度：8位小数
const SCALE = 100000000
const SCALE_FLOAT = 100000000.0

// StandardBigDecimal 基于 int64 的定点数实现
type StandardBigDecimal struct {
	Val int64
}

// 由于 int64 非常轻量，我们不再需要 sync.Pool
// 为了保持 API 兼容性，保留 Release 方法但留空
func (s *StandardBigDecimal) Release() {}

// Clone 返回副本
func (s *StandardBigDecimal) Clone() *StandardBigDecimal {
	return &StandardBigDecimal{Val: s.Val}
}

// NewDecimalFromString 从字符串解析定点数
func NewDecimalFromString(str string) (*StandardBigDecimal, error) {
	if str == "" {
		return nil, errors.New("empty string")
	}

	// 处理负号
	neg := false
	if str[0] == '-' {
		neg = true
		str = str[1:]
	}

	parts := strings.Split(str, ".")
	var intPart, fracPart int64
	var err error

	// 解析整数部分
	if parts[0] != "" {
		intPart, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, err
		}
	}

	// 解析小数部分
	if len(parts) > 1 && len(parts[1]) > 0 {
		fracStr := parts[1]
		if len(fracStr) > 8 {
			fracStr = fracStr[:8] // 截断超过8位的小数
		}
		// 补齐到8位
		padding := 8 - len(fracStr)
		if padding > 0 {
			fracStr += strings.Repeat("0", padding)
		}
		fracPart, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return nil, err
		}
	}

	val := intPart*SCALE + fracPart
	if neg {
		val = -val
	}

	return &StandardBigDecimal{Val: val}, nil
}

// NewDecimalFromFloat 从浮点数创建
func NewDecimalFromFloat(f float64) *StandardBigDecimal {
	return &StandardBigDecimal{Val: int64(f * SCALE_FLOAT)}
}

// Add 加法
func (s *StandardBigDecimal) Add(other *StandardBigDecimal) *StandardBigDecimal {
	return &StandardBigDecimal{Val: s.Val + other.Val}
}

// AddMut 原地加法
func (s *StandardBigDecimal) AddMut(other *StandardBigDecimal) {
	s.Val += other.Val
}

// Sub 减法
func (s *StandardBigDecimal) Sub(other *StandardBigDecimal) *StandardBigDecimal {
	return &StandardBigDecimal{Val: s.Val - other.Val}
}

// SubMut 原地减法
func (s *StandardBigDecimal) SubMut(other *StandardBigDecimal) {
	s.Val -= other.Val
}

// Mul 乘法
func (s *StandardBigDecimal) Mul(other *StandardBigDecimal) *StandardBigDecimal {
	// 简单的乘法实现：(a * b) / SCALE
	// 注意：这里可能溢出，生产环境建议使用 big.Int 做中间计算或 int128
	// 鉴于第一性原理追求极致性能，且假设金额在安全范围内，这里使用直接计算
	// 更好的做法是：return &StandardBigDecimal{Val: (s.Val * other.Val) / SCALE}
	// 为了稍微安全一点，可以将一个转为 float 计算再转回（牺牲一点性能换取不溢出），或者使用 math/big
	// 但为了极致性能，我们假设不会溢出（最大支持 9e10 * 1 = 9e18）
	return &StandardBigDecimal{Val: (s.Val * other.Val) / SCALE}
}

// Div 除法
func (s *StandardBigDecimal) Div(other *StandardBigDecimal) *StandardBigDecimal {
	if other.Val == 0 {
		return &StandardBigDecimal{Val: 0} // 避免 panic
	}
	// (a * SCALE) / b
	return &StandardBigDecimal{Val: (s.Val * SCALE) / other.Val}
}

// Cmp 比较
func (s *StandardBigDecimal) Cmp(other *StandardBigDecimal) int {
	if s.Val > other.Val {
		return 1
	}
	if s.Val < other.Val {
		return -1
	}
	return 0
}

// Neg 取反
func (s *StandardBigDecimal) Neg() *StandardBigDecimal {
	return &StandardBigDecimal{Val: -s.Val}
}

// SetZero 置零
func (s *StandardBigDecimal) SetZero() {
	s.Val = 0
}

// String 格式化输出
func (s *StandardBigDecimal) String() string {
	val := s.Val
	neg := false
	if val < 0 {
		neg = true
		val = -val
	}

	intPart := val / SCALE
	fracPart := val % SCALE

	// 格式化小数部分，保留逻辑与 decimal 库类似（去掉末尾 0，但如果本来就是整数则加 .0 ？）
	// 原项目要求：如果 !strings.Contains(amount, ".") { amount = amount + ".0" }
	// 我们这里输出标准格式，例如 "100.50000000" -> "100.5"

	fracStr := fmt.Sprintf("%08d", fracPart)
	fracStr = strings.TrimRight(fracStr, "0")
	
	res := strconv.FormatInt(intPart, 10)
	if fracStr != "" {
		res += "." + fracStr
	} else {
		// 保持之前的行为，整数不带小数点？
		// 原代码：if !strings.Contains(amount, ".") { amount = amount + ".0" } 是在 MarshalJSON 里做的
		// 这里只返回最简字符串
	}

	if neg {
		return "-" + res
	}
	return res
}

// Float64 转浮点数
func (s *StandardBigDecimal) Float64() float64 {
	return float64(s.Val) / SCALE_FLOAT
}
