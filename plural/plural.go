// Package plural provides CLDR plural-rule selection (cardinal and ordinal)
// for Go, generated directly from the Unicode CLDR data. It has zero external
// dependencies and is designed to match the behaviour of JavaScript's
// Intl.PluralRules exactly.
//
// The plural rule tables in tables_gen.go are produced by the generator in
// internal/gen. To regenerate them, run:
//
//	go generate ./plural/...
//
// See internal/gen/main.go for how to repoint the generator at a different
// copy of the CLDR data.
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
//go:generate go run ./internal/gen/main.go -out tables_gen.go
//go:generate node internal/gen/samples.js testdata/cldr_samples.json
//go:generate node internal/gen/intl.js testdata/intl_plurals.json

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

// rule is the signature of a generated per-rule-set predicate. It receives the
// operands and reports whether the operands match the rule for a given
// category. Each generated rule set maps categories to such predicates.
type rule func(o Operands) bool

// ruleSet maps plural categories (in evaluation order) to predicates. The
// generator emits one ruleSet per distinct set of CLDR conditions and points
// every locale that shares those conditions at it.
type ruleSet struct {
	// cats lists the categories to test, in CLDR order. The last entry is
	// always Other and its predicate always returns true.
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
	if maxFrac < minFrac {
		maxFrac = minFrac
	}
	neg := math.Signbit(n)
	abs := math.Abs(n)
	if math.IsInf(abs, 0) || math.IsNaN(abs) {
		return Operands{N: abs}
	}
	// Round to maxFrac fraction digits half-away-from-zero on the shortest
	// round-tripping decimal (ECMA-402's halfExpand; shared with the number
	// formatter via internal/decimal), then trim trailing zeros down to minFrac
	// for the canonical visible representation.
	s := roundHalfAway(abs, maxFrac)
	s = trimToMinFrac(s, minFrac)
	if neg {
		s = "-" + s
	}
	ops, err := OperandsFromString(s)
	if err != nil {
		// The freshly built decimal string can only fail to parse when its
		// integer or fraction part exceeds int64 (|n| >= 2^63). Operands I/F
		// cannot represent such a magnitude; N still drives rule selection and
		// every locale categorises numbers that large as "other", so fall back
		// to N alone.
		return Operands{N: abs}
	}
	return ops
}

// roundHalfAway rounds the non-negative value abs to exactly maxFrac fraction
// digits (half-away-from-zero) and returns the fixed-notation decimal string;
// callers trim trailing zeros down to minFrac afterwards.
func roundHalfAway(abs float64, maxFrac int) string {
	intPart, fracPart := decimal.RoundFixed(abs, maxFrac)
	if fracPart == "" {
		return intPart
	}
	return intPart + "." + fracPart
}

// trimToMinFrac removes trailing fractional zeros from a decimal string until
// only minFrac fraction digits remain (never removing the digits required by
// minFrac). If the string has no fraction part, minFrac zeros are appended.
func trimToMinFrac(s string, minFrac int) string {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		if minFrac == 0 {
			return s
		}
		return s + "." + strings.Repeat("0", minFrac)
	}
	frac := s[dot+1:]
	// Trim trailing zeros but keep at least minFrac digits.
	end := len(frac)
	for end > minFrac && frac[end-1] == '0' {
		end--
	}
	frac = frac[:end]
	if frac == "" {
		return s[:dot]
	}
	return s[:dot] + "." + frac
}

// OperandsFromString computes the plural operands from a canonical decimal
// string. The string is authoritative for the fraction-digit operands
// (V, W, F, T): the number of digits after the decimal point determines V/W
// and their integer values determine F/T, exactly as written.
//
// The accepted syntax is an optional leading '-' (or '+'), one or more integer
// digits, an optional '.' with one or more fraction digits, and an optional
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
	// Split off the compact exponent suffix.
	var compact int
	if idx := strings.IndexAny(str, "ceCE"); idx >= 0 {
		expPart := str[idx+1:]
		str = str[:idx]
		e, err := strconv.Atoi(expPart)
		if err != nil {
			return Operands{}, errors.New("plural: invalid compact exponent in " + strconv.Quote(s))
		}
		compact = e
	}
	intPart := str
	fracPart := ""
	if dot := strings.IndexByte(str, '.'); dot >= 0 {
		intPart = str[:dot]
		fracPart = str[dot+1:]
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
	// I: integer value of the integer digits.
	i64, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return Operands{}, errors.New("plural: integer part overflow in " + strconv.Quote(s))
	}
	ops.I = i64

	// V/F use the fraction digits as written (with trailing zeros).
	ops.V = len(fracPart)
	if fracPart != "" {
		f64, err := strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			return Operands{}, errors.New("plural: fraction part overflow in " + strconv.Quote(s))
		}
		ops.F = f64
	}

	// W/T strip trailing zeros from the fraction digits. trimmed is a prefix of
	// fracPart, which already parsed into F above, so it cannot overflow here.
	trimmed := strings.TrimRight(fracPart, "0")
	ops.W = len(trimmed)
	if trimmed != "" {
		t64, _ := strconv.ParseInt(trimmed, 10, 64)
		ops.T = t64
	}

	// N: absolute numeric value (integer part plus fraction). Both digit
	// strings were validated above and the integer part fits int64, so the
	// composed literal (Go accepts a bare trailing '.') always parses.
	ops.N, _ = strconv.ParseFloat(intPart+"."+fracPart, 64)
	ops.C = compact
	return ops, nil
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
	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}
	return intPart, fracPart
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
var otherOnly = &ruleSet{cats: []Category{Other}, preds: []rule{func(Operands) bool { return true }}}

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
