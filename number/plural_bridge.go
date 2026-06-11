package number

import (
	"math"

	"github.com/hakastein/gocldr/plural"
)

// PluralOperands returns the CLDR plural operands for value as it would be
// FORMATTED for the given locale and options, so plural selection always agrees
// with the rendered number. It resolves the same integer/fraction/significant
// digit constraints and half-away-from-zero rounding that Format uses, then
// derives the operands (i/v/w/f/t/n) from the resulting digit strings.
//
// Style is deliberately ignored: Intl.PluralRules has no "style" option (it
// never multiplies a percent by 100, nor applies currency fraction defaults),
// so the decimal-style digit defaults (min 0, max 3 fraction digits) apply
// unless overridden by explicit fraction/significant-digit options. The
// resulting operands match Intl.PluralRules for the same option bag.
func PluralOperands(locale string, value float64, opts Options) plural.Operands {
	// Resolve digit constraints using decimal-style defaults (see doc comment).
	rs := resolveRounding("decimal", &opts, currencyInfo{})
	if !rs.useSig {
		return plural.NewOperands(value, rs.minFr, rs.maxFr)
	}

	abs := math.Abs(value)
	if math.IsInf(abs, 0) || math.IsNaN(abs) {
		return plural.Operands{N: abs}
	}
	return operandsFromDigits(formatMagnitude(abs, rs))
}

// operandsFromDigits derives plural operands from displayed integer and
// fraction digit strings; formatter-produced digit strings always parse.
func operandsFromDigits(intDigits, fracDigits string) plural.Operands {
	s := intDigits
	if fracDigits != "" {
		s += "." + fracDigits
	}
	ops, _ := plural.OperandsFromString(s)
	return ops
}

// CardinalCategory returns the locale's cardinal plural category for value as it
// would be formatted under opts, matching Intl.PluralRules (cardinal).
func CardinalCategory(locale string, value float64, opts Options) string {
	return string(plural.Cardinal(locale, PluralOperands(locale, value, opts)))
}

// OrdinalCategory returns the locale's ordinal plural category for value as it
// would be formatted under opts, matching Intl.PluralRules (ordinal).
func OrdinalCategory(locale string, value float64, opts Options) string {
	return string(plural.Ordinal(locale, PluralOperands(locale, value, opts)))
}

// pluralCategoryForDigits returns the CLDR cardinal plural category for a number
// whose displayed integer and fraction digit strings are given. Intl derives
// plural operands from the digits actually shown (so "1.00" has v=2 and yields
// the "other" category in many locales, not "one"). Used for
// currencyDisplay:"name" pattern selection.
func pluralCategoryForDigits(locale, intDigits, fracDigits string) string {
	return string(plural.Cardinal(locale, operandsFromDigits(intDigits, fracDigits)))
}
