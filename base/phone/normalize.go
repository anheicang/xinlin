package phone

import (
	"strings"
	"unicode"
)

// Normalize removes whitespace from a phone number before validation, lookup or
// storage. It deliberately retains other characters for the caller to validate.
func Normalize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '\uFEFF' {
			return -1
		}
		return r
	}, value)
}

// NormalizeSearch supports a phone number in a mixed name/phone search without
// removing meaningful spaces from names or other account identifiers.
func NormalizeSearch(value string) string {
	compact := Normalize(value)
	if compact != "" && strings.IndexFunc(compact, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		return compact
	}
	return strings.TrimSpace(value)
}
