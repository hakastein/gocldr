// Package decimal provides the shared fixed-point decimal helpers used by the
// number and plural formatters: half-away-from-zero rounding on the shortest
// round-tripping decimal representation of a float64. Working on the shortest
// decimal (what JavaScript's Number->string yields) makes rounding decisions
// match Intl.NumberFormat, which rounds the Number's shortest decimal rather
// than its full binary expansion.
package decimal

import (
	"strconv"
	"strings"
)

// Split splits a fixed-notation decimal string into its integer and fraction
// digit parts (without the dot). A missing fraction yields "".
func Split(s string) (intPart, fracPart string) {
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		return s[:dot], s[dot+1:]
	}
	return s, ""
}

// Shortest returns the integer and fraction digit strings of the non-negative
// value abs using the shortest representation that round-trips through float64.
func Shortest(abs float64) (intPart, fracPart string) {
	return Split(strconv.FormatFloat(abs, 'f', -1, 64))
}

// RoundFixed rounds the non-negative value abs to exactly keepFrac fraction
// digits using half-away-from-zero, returning the integer and fraction digit
// strings. The fraction string has exactly keepFrac digits (callers trim to a
// minimum afterwards).
func RoundFixed(abs float64, keepFrac int) (intPart, fracPart string) {
	i, f := Shortest(abs)
	return roundDigits(i, f, keepFrac)
}

// TrimFrac trims trailing zeros from a digit string, keeping at least min
// digits.
func TrimFrac(frac string, min int) string {
	end := len(frac)
	for end > min && frac[end-1] == '0' {
		end--
	}
	return frac[:end]
}

// roundDigits rounds the decimal represented by intPart.fracPart to keep exactly
// keepFrac fraction digits, half away from zero. intPart must have at least one
// digit.
func roundDigits(intPart, fracPart string, keepFrac int) (string, string) {
	if len(fracPart) <= keepFrac {
		if keepFrac > 0 {
			fracPart += strings.Repeat("0", keepFrac-len(fracPart))
		} else {
			fracPart = ""
		}
		return NormInt(intPart), fracPart
	}

	kept := fracPart[:keepFrac]
	roundUp := fracPart[keepFrac] >= '5'

	combined := intPart + kept
	if roundUp {
		combined = Increment(combined)
	}
	if keepFrac == 0 {
		return NormInt(combined), ""
	}
	// combined is intPart (>=1 digit) plus exactly keepFrac kept digits, so it
	// always has more than keepFrac digits even after a carry grows it by one.
	newInt := combined[:len(combined)-keepFrac]
	newFrac := combined[len(combined)-keepFrac:]
	return NormInt(newInt), newFrac
}

// Increment adds 1 to a pure digit string, growing it on carry.
func Increment(d string) string {
	b := []byte(d)
	if len(b) == 0 {
		return "1"
	}
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] < '9' {
			b[i]++
			return string(b)
		}
		b[i] = '0'
	}
	return "1" + string(b)
}

// NormInt strips leading zeros from an integer digit string, leaving at least
// one digit.
func NormInt(s string) string {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return s
}
