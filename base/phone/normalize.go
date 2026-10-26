// 文件用途：清理手机号空白，并区分纯数字查询与普通姓名查询。

package phone

import (
	"strings"
	"unicode"
)

// Normalize 移除 Unicode 空白和 BOM，供手机号校验、查询和入库前使用。
// 保留加号、连字符等其他字符，号码合法性由调用方校验。
func Normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\uFEFF' {
			return -1
		}
		return r
	}, value)
}

// NormalizeSearch 规范化混合姓名/手机号搜索值。
// 去空白后为纯数字时使用紧凑值，否则只去首尾空白，保留姓名内部空格。
func NormalizeSearch(value string) string {
	compact := Normalize(value)
	if compact != "" && strings.IndexFunc(compact, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		return compact
	}
	return strings.TrimSpace(value)
}
