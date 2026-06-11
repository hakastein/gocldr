// Package digits substitutes numbering-system digit glyphs into formatted text.
package digits

import "strings"

// Substitute maps ASCII digits 0-9 in s to the numbering system's digit glyphs.
// digits is empty (no substitution) or exactly ten glyphs (a generated-table
// invariant); a Latin set returns s unchanged.
func Substitute(s string, digits []rune) string {
	if len(digits) == 0 || (digits[0] == '0' && digits[9] == '9') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(digits[r-'0'])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
