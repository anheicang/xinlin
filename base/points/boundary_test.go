// 文件用途：验证最大金额、非法格式、最小 int64 展示及无效分母。

package points

import (
	"math"
	"testing"
)

// TestAmountBoundaries 验证最大金额、非法格式、最小 int64 展示及无效分母。
func TestAmountBoundaries(t *testing.T) {
	for _, input := range []string{".", ".1", "1.", "1.2.3", "999999999999999999999999", "+1", "1 2"} {
		if _, err := Parse(input); err == nil {
			t.Errorf("invalid value accepted: %q", input)
		}
	}
	if got, err := Parse(" 90071992547409.91 "); err != nil || got != MaxFen {
		t.Fatalf("max value=%d err=%v", got, err)
	}
	if Display(1234) != 12.34 || Format(math.MinInt64) != "-92233720368547758.08" {
		t.Fatal("amount display incorrect")
	}
	for _, denominator := range []int64{0, -1} {
		if Proportion(100, 1, denominator) != 0 {
			t.Fatal("invalid denominator accepted")
		}
	}
	if Proportion(100, 1, 3) != 33 || Proportion(100, 0, 3) != 0 {
		t.Fatal("proportion rounding incorrect")
	}
}
