// 文件用途：完成金额/积分的整数分转换、格式化和比例计算。

package points

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Scale 表示一个显示单位对应的整数分数，即 1 元/积分 = 100 分。
const Scale int64 = 100

// MaxFen 是允许解析的最大整数分，取 JavaScript Number 可精确表示的最大整数。
const MaxFen int64 = 9007199254740991

// Display 把整数分转换成显示用的小数单位。
// 只用于输出边界，后续金额运算仍应使用整数分。
func Display(fen int64) float64 {
	return float64(fen) / float64(Scale)
}

// Parse 把非负、最多两位小数的文本转换成整数分。
// 拒绝科学计数法、负数、额外精度和超过 MaxFen 的值，不进行四舍五入。
func Parse(value string) (int64, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, errors.New("积分格式不正确")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 0 || len(fraction) > 2 {
			return 0, errors.New("积分最多两位小数")
		}
	}
	for _, c := range parts[0] + fraction {
		if c < '0' || c > '9' {
			return 0, errors.New("积分须为非负数")
		}
	}
	amount, err := strconv.ParseInt(parts[0]+fraction+strings.Repeat("0", 2-len(fraction)), 10, 64)
	if err != nil || amount > MaxFen {
		return 0, errors.New("积分超出范围")
	}
	return amount, nil
}

// Format 把整数分格式化为固定两位小数文本，支持负值展示。
func Format(fen int64) string {
	if fen < 0 {
		return fmt.Sprintf("-%d.%02d", -(fen / Scale), -(fen % Scale))
	}
	return fmt.Sprintf("%d.%02d", fen/Scale, fen%Scale)
}

// Proportion 计算 fen*numerator/denominator，避免中间乘法溢出。
// 结果向零截断；非负业务值等同于向下取整，分母非正数返回 0，调用方需保证最终结果可用 int64 表示。
func Proportion(fen, numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	result := new(big.Int).Mul(big.NewInt(fen), big.NewInt(numerator))
	return result.Quo(result, big.NewInt(denominator)).Int64()
}
