// 文件用途：验证非负两位小数转整数分及常见非法金额输入；验证大整数比例计算和分批退款累计分值守恒。

package points

import "testing"

// TestParsePointsToFen 验证非负两位小数转整数分及常见非法金额输入。
func TestParsePointsToFen(t *testing.T) {
	for input, want := range map[string]int64{"0": 0, "0.01": 1, "0.29": 29, "1": 100, "12.34": 1234, "498": 49800, "0001.20": 120} {
		got, err := Parse(input)
		if err != nil || got != want {
			t.Fatalf("%s = %d, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "-1", "NaN", "Inf", "0.001", "1.0000001", "1e3", "90071992547409.92"} {
		if _, err := Parse(input); err == nil {
			t.Fatalf("invalid points accepted: %s", input)
		}
	}
	if Format(1234) != "12.34" || Format(-1) != "-0.01" {
		t.Fatal("display units incorrect")
	}
}

// TestProportionConservesFenWithoutIntermediateOverflow 验证大整数比例计算和分批退款累计分值守恒。
func TestProportionConservesFenWithoutIntermediateOverflow(t *testing.T) {
	if Proportion(900000000000, 900000000000, 900000000000) != 900000000000 {
		t.Fatal("overflow")
	}
	for _, amount := range []int64{1, 2, 1234, 6700} {
		var refunded int64
		for i := int64(1); i <= 3; i++ {
			refund := Proportion(amount, i, 3) - refunded
			if refund < 0 {
				t.Fatal("negative refund")
			}
			refunded += refund
		}
		if refunded != amount {
			t.Fatalf("refunded %d != %d", refunded, amount)
		}
	}
}
