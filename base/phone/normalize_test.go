// 文件用途：验证手机号 Unicode 空白清理与非空白字符保留；验证纯数字搜索去空白，而姓名等普通文本保留内部空格。

package phone

import "testing"

// TestNormalize 验证手机号 Unicode 空白清理与非空白字符保留。
func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"151 9009 5877", "15190095877"},
		{"\t151\u30009009\u00a05877\r\n", "15190095877"},
		{"151\u202f9009\u00855877\ufeff", "15190095877"},
		{"151-9009-5877", "151-9009-5877"},
		{"151a90095877", "151a90095877"},
		{" \t\n", ""},
		{"+86 15190095877", "+8615190095877"},
	} {
		if got := Normalize(tc.input); got != tc.want {
			t.Errorf("Normalize(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}

// TestNormalizeSearch 验证纯数字搜索去空白，而姓名等普通文本保留内部空格。
func TestNormalizeSearch(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"151 9009 5877", "15190095877"},
		{" 张 三 ", "张 三"},
		{" account name ", "account name"},
	} {
		if got := NormalizeSearch(tc.input); got != tc.want {
			t.Errorf("NormalizeSearch(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}
