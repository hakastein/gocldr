package number

import (
	"strconv"
	"strings"
)

// decimal is a sign-less decimal value represented as a digit string plus the
// position of the decimal point. digits holds all significant digits (no
// leading zeros except a single "0" for zero); exp is the power of ten of the
// last digit. The value is digits * 10^exp interpreted with the point implied.
//
// We work on the shortest round-tripping decimal representation of the float64
// (what JavaScript's Number->string yields, and what Intl rounds), so that
// rounding decisions match Intl.NumberFormat, which rounds the Number's
// shortest decimal, not its full binary expansion.

// shortestDecimal returns the integer-digit and fraction-digit strings of abs
// using the shortest representation that round-trips through float64.
func shortestDecimal(abs float64) (intDigits, fracDigits string) {
	s := strconv.FormatFloat(abs, 'f', -1, 64)
	return splitDot(s)
}

// roundFixed rounds the non-negative value abs to exactly maxFrac fraction
// digits using half-expand (round half away from zero), then returns the
// integer and fraction digit strings. The fraction string has exactly maxFrac
// digits (callers trim to minFrac afterwards).
func roundFixed(abs float64, maxFrac int) (string, string) {
	intPart, fracPart := shortestDecimal(abs)
	return roundDigits(intPart, fracPart, maxFrac)
}

// roundDigits rounds the decimal represented by intPart.fracPart to keep exactly
// keepFrac fraction digits, half away from zero.
func roundDigits(intPart, fracPart string, keepFrac int) (string, string) {
	if len(fracPart) <= keepFrac {
		// Pad with zeros to reach keepFrac.
		if keepFrac > 0 {
			fracPart += strings.Repeat("0", keepFrac-len(fracPart))
		} else {
			fracPart = ""
		}
		return normInt(intPart), fracPart
	}

	// We need to drop the digits beyond keepFrac, rounding on the first dropped.
	kept := fracPart[:keepFrac]
	roundUp := fracPart[keepFrac] >= '5'

	combined := intPart + kept // all kept digits, integer + fraction
	if roundUp {
		combined = incrementDigits(combined)
	}
	// combined may have grown by one digit (carry). Split back: the rightmost
	// keepFrac digits are the fraction.
	if keepFrac == 0 {
		return normInt(combined), ""
	}
	if len(combined) < keepFrac {
		combined = strings.Repeat("0", keepFrac-len(combined)) + combined
	}
	newInt := combined[:len(combined)-keepFrac]
	newFrac := combined[len(combined)-keepFrac:]
	return normInt(newInt), newFrac
}

// incrementDigits adds 1 to a pure digit string, growing it on carry.
func incrementDigits(d string) string {
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

// normInt strips leading zeros from an integer digit string, leaving at least
// one digit.
func normInt(s string) string {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return s
}
