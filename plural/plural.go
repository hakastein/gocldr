// Package plural provides CLDR plural-rule selection (cardinal and ordinal)
// for Go, generated directly from the Unicode CLDR data. It has zero external
// dependencies and is designed to match the behaviour of JavaScript's
// Intl.PluralRules exactly.
//
// The plural rule tables in tables_gen.go are produced by the generator in
// internal/gen; regenerate them with `make gen` (pinned Docker toolchain).
package plural

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/hakastein/gocldr/internal/decimal"
	"github.com/hakastein/gocldr/internal/locale"
)

// CLDR input paths come from $CLDR_DATA (set by the pinned gen image).
// Run via `make gen`; never on the host.
//go:generate go run ./internal/gen -out tables_gen.go
//go:generate node internal/gen/samples.js
//go:generate node internal/gen/intl.js

// Category is a CLDR plural category.
type Category string

// The six CLDR plural categories.
const (
	Zero  Category = "zero"
	One   Category = "one"
	Two   Category = "two"
	Few   Category = "few"
	Many  Category = "many"
	Other Category = "other"
)

// Operands holds the CLDR plural operands derived from a formatted number, as
// defined by UTS #35 (Language Plural Rules).
//
//	N: the absolute value of the source number.
//	I: the integer digits of the number.
//	V: the number of visible fraction digits, with trailing zeros.
//	W: the number of visible fraction digits, without trailing zeros.
//	F: the visible fraction digits, with trailing zeros, as an integer.
//	T: the visible fraction digits, without trailing zeros, as an integer.
//	C: the compact/scientific exponent value (UTS #35 operand c, also named e).
type Operands struct {
	N float64
	I int64
	V int
	W int
	F int64
	T int64
	C int
}

// modNf computes the CLDR modulo of a (possibly fractional) operand value by an
// integer modulus, as used by the generated rules for `n % m`. Per UTS #35 the
// result is n - m*floor(n/m), preserving any fractional part so that a
// subsequent integer value/range comparison only matches an integer-valued
// remainder.
func modNf(n float64, m int64) float64 {
	r := math.Mod(n, float64(m))
	if r < 0 {
		r += float64(m)
	}
	return r
}

// inRangeN reports whether the float operand n lies within the inclusive
// integer range [lo, hi]. Per UTS #35, a range in an `=`/`!=` relation matches
// a non-integer operand only when the operand is integer-valued, so a
// fractional n never matches an integer range.
func inRangeN(n float64, lo, hi int64) bool {
	if n != math.Trunc(n) {
		return false
	}
	return n >= float64(lo) && n <= float64(hi)
}

// rule is a generated predicate reporting whether the operands match a
// category's CLDR conditions.
type rule func(o Operands) bool

// ruleSet maps plural categories (in evaluation order) to predicates. The
// generator emits one ruleSet per distinct set of CLDR conditions and points
// every locale that shares those conditions at it.
type ruleSet struct {
	// cats lists the categories to test, in CLDR order. The last entry is
	// always Other and has no predicate; eval falls through to it.
	cats  []Category
	preds []rule
}

func (rs *ruleSet) eval(o Operands) Category {
	for i, p := range rs.preds {
		if p(o) {
			return rs.cats[i]
		}
	}
	return Other
}

// NewOperands computes the plural operands for the floating-point value n,
// honouring the requested fraction-digit formatting. The number is formatted
// with at least minFrac and at most maxFrac fraction digits (matching the
// minimumFractionDigits / maximumFractionDigits options of Intl.NumberFormat),
// and the operands are derived from that formatted representation.
//
// The compact exponent operand C is left at 0; use OperandsFromString or set
// the field directly if you need compact notation.
func NewOperands(n float64, minFrac, maxFrac int) Operands {
	if minFrac < 0 {
		minFrac = 0
	}
	if minFrac > 100 {
		minFrac = 100 // ECMA-402 fraction-digits ceiling
	}
	if maxFrac < minFrac {
		maxFrac = minFrac
	}
	if maxFrac > 100 {
		maxFrac = 100
	}
	neg := math.Signbit(n)
	abs := math.Abs(n)
	if math.IsInf(abs, 0) || math.IsNaN(abs) {
		return Operands{N: abs}
	}
	// ECMA-402's halfExpand rounding, shared with the number formatter via
	// internal/decimal.
	intPart, fracPart := decimal.RoundFixed(abs, maxFrac)
	s := intPart
	if fracPart = decimal.TrimFrac(fracPart, minFrac); fracPart != "" {
		s += "." + fracPart
	}
	if neg {
		s = "-" + s
	}
	// s is a plain decimal digit string, so OperandsFromString cannot fail.
	ops, _ := OperandsFromString(s)
	return ops
}

// OperandsFromString computes the plural operands from a canonical decimal
// string. The string is authoritative for the fraction-digit operands
// (V, W, F, T): the number of digits after the decimal point determines V/W
// and their integer values determine F/T, exactly as written. Digit runs
// longer than int64 can hold contribute their last 18 digits to I/F/T
// (ICU's behaviour for huge values); V/W always count every digit.
//
// The accepted syntax is an optional leading '-' (or '+'), integer digits, an
// optional '.' with fraction digits — at least one digit must be present
// overall, so ".5" and "5." are accepted but "." is not — and an optional
// exponent suffix 'c' or 'e' followed by a signed integer (e.g. "1.5",
// "1000000", "1.2c6"). The exponent scales the value by shifting the decimal
// point and is also reported as operand C, per UTS #35 (so "1.2c6" yields
// N=1200000, C=6).
func OperandsFromString(s string) (Operands, error) {
	if s == "" {
		return Operands{}, errors.New("plural: empty number string")
	}
	str := s
	if str[0] == '+' || str[0] == '-' {
		str = str[1:] // operands are defined on the absolute value
	}
	var compact int
	if idx := strings.IndexAny(str, "ceCE"); idx >= 0 {
		expPart := str[idx+1:]
		str = str[:idx]
		e, err := strconv.Atoi(expPart)
		if err != nil {
			return Operands{}, errors.New("plural: invalid compact exponent in " + strconv.Quote(s))
		}
		if e > 400 || e < -400 {
			return Operands{}, errors.New("plural: exponent out of range in " + strconv.Quote(s))
		}
		compact = e
	}
	intPart, fracPart := decimal.Split(str)
	if intPart == "" && fracPart == "" {
		return Operands{}, errors.New("plural: no digits in " + strconv.Quote(s))
	}
	if intPart == "" {
		intPart = "0"
	}
	if !allDigits(intPart) || !allDigits(fracPart) {
		return Operands{}, errors.New("plural: invalid number string " + strconv.Quote(s))
	}

	// Apply the exponent by shifting the decimal point, as required by UTS #35:
	// the operands i/v/f/t/n are computed from the scaled value while operand
	// c/e retains the exponent. Compact decimals use a positive exponent;
	// scientific notation may use a negative one.
	intPart, fracPart = shiftPoint(intPart, fracPart, compact)

	var ops Operands
	ops.I = digitsValue(intPart)
	ops.V = len(fracPart)
	ops.F = digitsValue(fracPart)
	trimmed := strings.TrimRight(fracPart, "0")
	ops.W = len(trimmed)
	ops.T = digitsValue(trimmed)

	// N: absolute numeric value. The composed literal always parses (Go
	// accepts a bare trailing '.'); magnitudes beyond float64 saturate to +Inf.
	ops.N, _ = strconv.ParseFloat(intPart+"."+fracPart, 64)
	ops.C = compact
	return ops, nil
}

// digitsValue converts a digit string to its integer value modulo 1e18,
// matching how ICU derives the i/f/t operands when the digits exceed int64.
func digitsValue(digits string) int64 {
	if len(digits) > 18 {
		digits = digits[len(digits)-18:]
	}
	v, _ := strconv.ParseInt(digits, 10, 64)
	return v
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// shiftPoint moves the decimal point of intPart.fracPart by `by` places — right
// when positive, left when negative — as applying an exponent of `by` requires,
// and returns the normalised integer and fraction digit strings.
func shiftPoint(intPart, fracPart string, by int) (string, string) {
	switch {
	case by > 0:
		for by > 0 && fracPart != "" {
			intPart += fracPart[:1]
			fracPart = fracPart[1:]
			by--
		}
		if by > 0 {
			intPart += strings.Repeat("0", by)
		}
	case by < 0:
		for by < 0 && intPart != "" {
			fracPart = intPart[len(intPart)-1:] + fracPart
			intPart = intPart[:len(intPart)-1]
			by++
		}
		if by < 0 {
			fracPart = strings.Repeat("0", -by) + fracPart
		}
	}
	return decimal.NormInt(intPart), fracPart
}

// lookup resolves a locale against a table by trying the exact (canonicalised)
// tag and then progressively stripping trailing subtags. The walk deliberately
// passes nil parentLocale overrides: Intl.PluralRules resolves by truncation
// only (ICU selects pt's rules for pt-AO, not pt-PT's). A total miss yields
// the universal "everything is Other" rule set, which is also how CLDR's
// catch-all "und" entry behaves.
func lookup(table map[string]*ruleSet, loc string) *ruleSet {
	if tag, ok := locale.Resolve(loc, nil, func(t string) bool { _, ok := table[t]; return ok }); ok {
		return table[tag]
	}
	return otherOnly
}

// otherOnly is the universal fallback rule set: everything is Other.
var otherOnly = &ruleSet{cats: []Category{Other}}

// Cardinal returns the cardinal plural category for the given operands in the
// given locale.
func Cardinal(locale string, ops Operands) Category {
	return lookup(cardinalRules, locale).eval(ops)
}

// Ordinal returns the ordinal plural category for the given operands in the
// given locale.
func Ordinal(locale string, ops Operands) Category {
	return lookup(ordinalRules, locale).eval(ops)
}

// CardinalFor is a convenience wrapper that computes the operands for n with
// the given fraction-digit formatting and returns its cardinal category.
func CardinalFor(locale string, n float64, minFrac, maxFrac int) Category {
	return Cardinal(locale, NewOperands(n, minFrac, maxFrac))
}

// OrdinalFor is a convenience wrapper that computes the operands for n with the
// given fraction-digit formatting and returns its ordinal category.
func OrdinalFor(locale string, n float64, minFrac, maxFrac int) Category {
	return Ordinal(locale, NewOperands(n, minFrac, maxFrac))
}
