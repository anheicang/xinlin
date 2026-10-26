// Package points converts human-readable points (1 point = 1 yuan) to integer
// fen. Requests, database values and business arithmetic use fen; product
// response prices are converted back to decimal points at the output boundary.
package points

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const Scale int64 = 100

// MaxFen is exact in a JavaScript JSON number as well as a Go int64.
const MaxFen int64 = 9007199254740991

// Display converts integer fen to the decimal points used by product responses.
// Keep business arithmetic and request values in integer fen.
func Display(fen int64) float64 {
	return float64(fen) / float64(Scale)
}

// Parse accepts ordinary nonnegative decimal text, rejecting extra precision
// instead of silently rounding a price from an Excel cell.
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

func Format(fen int64) string {
	if fen < 0 {
		return fmt.Sprintf("-%d.%02d", -(fen / Scale), -(fen % Scale))
	}
	return fmt.Sprintf("%d.%02d", fen/Scale, fen%Scale)
}

// Proportion floors to a whole fen without overflowing the intermediate
// product. Use cumulative targets to conserve every fen across partial refunds.
func Proportion(fen, numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	result := new(big.Int).Mul(big.NewInt(fen), big.NewInt(numerator))
	return result.Quo(result, big.NewInt(denominator)).Int64()
}
