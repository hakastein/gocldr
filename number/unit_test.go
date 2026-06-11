package number_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hakastein/gocldr/number"
	_ "github.com/hakastein/gocldr/number/locales/all"
)

func ptrInt(i int) *int    { return &i }
func ptrBool(b bool) *bool { return &b }

// TestSpecialValues covers non-finite inputs, which Intl renders via locale
// symbols inside the style's pattern affixes (e.g. "NaN%", "$NaN"). Golden
// fixtures cannot encode NaN (JSON), so these cases are pinned here.
func TestSpecialValues(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		value  float64
		opts   number.Options
		want   string
	}{
		{name: "en NaN", locale: "en", value: math.NaN(), want: "NaN"},
		{name: "en +Inf", locale: "en", value: math.Inf(1), want: "∞"},
		{name: "en -Inf", locale: "en", value: math.Inf(-1), want: "-∞"},
		{name: "de NaN", locale: "de", value: math.NaN(), want: "NaN"},
		{name: "en NaN percent", locale: "en", value: math.NaN(), opts: number.Options{Style: "percent"}, want: "NaN%"},
		{name: "en NaN currency symbol", locale: "en", value: math.NaN(), opts: number.Options{Style: "currency", Currency: "USD"}, want: "$NaN"},
		{name: "en NaN currency name", locale: "en", value: math.NaN(), opts: number.Options{Style: "currency", Currency: "USD", CurrencyDisplay: "name"}, want: "NaN US dollars"},
		{name: "de NaN currency", locale: "de", value: math.NaN(), opts: number.Options{Style: "currency", Currency: "EUR"}, want: "NaN €"},
		{name: "sign-bit NaN stays unsigned", locale: "en", value: math.Copysign(math.NaN(), -1), opts: number.Options{Style: "percent"}, want: "NaN%"},
		{name: "en +Inf percent", locale: "en", value: math.Inf(1), opts: number.Options{Style: "percent"}, want: "∞%"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := number.Format(tc.locale, tc.value, tc.opts)
			assert.Equal(t, tc.want, got)
		})
	}
}

// negZero returns a true negative zero (-0.0), which a literal "-0" constant
// folds away to +0 in Go.
func negZero() float64 {
	z := 0.0
	return -z
}

// TestNegativeZero asserts the formatter preserves the sign of negative zero and
// of negatives that round to integer zero, matching Intl.NumberFormat, which
// renders "-0" / "-0%" (confirmed against Node, CLDR 46).
func TestNegativeZero(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		value  float64
		opts   number.Options
		want   string
	}{
		{name: "negative zero decimal", locale: "en", value: negZero(), opts: number.Options{}, want: "-0"},
		{name: "negative rounds to zero", locale: "en", value: -0.4, opts: number.Options{MaximumFractionDigits: ptrInt(0)}, want: "-0"},
		{name: "negative zero percent", locale: "en", value: negZero(), opts: number.Options{Style: "percent"}, want: "-0%"},
		{name: "negative percent rounds to zero", locale: "en", value: -0.004, opts: number.Options{Style: "percent", MaximumFractionDigits: ptrInt(0)}, want: "-0%"},
		// Sanity: the fraction-shown path already keeps the sign.
		{name: "negative zero with fraction shown", locale: "en", value: negZero(), opts: number.Options{MinimumFractionDigits: ptrInt(2)}, want: "-0.00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := number.Format(tc.locale, tc.value, tc.opts)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestLocaleFallback verifies the exact -> truncate -> root resolution chain.
func TestLocaleFallback(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{name: "unknown region falls back to en", locale: "en-XX", want: "1,234.5"},
		{name: "multi-subtag resolves like zh", locale: "zh-Hans-CN", want: "1,234.5"},
		{name: "case-insensitive language subtag", locale: "EN", want: "1,234.5"},
		{name: "underscore separator, region present", locale: "de_DE", want: "1.234,5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := number.Format(tc.locale, 1234.5, number.Options{})
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestCurrencyBasics confirms the standard (non-accounting) currency pattern is
// used by default and that negative values use the locale minus sign.
func TestCurrencyBasics(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		value  float64
		opts   number.Options
		want   string
	}{
		{name: "lowercase code", locale: "en", value: 1234.5, opts: number.Options{Style: "currency", Currency: "usd"}, want: "$1,234.50"},
		{name: "negative uses minus sign", locale: "en", value: -5, opts: number.Options{Style: "currency", Currency: "USD"}, want: "-$5.00"},
		{name: "JPY no fraction", locale: "en", value: 1234.5, opts: number.Options{Style: "currency", Currency: "JPY"}, want: "¥1,235"},
		{name: "unknown currency -> code", locale: "en", value: 1234.5, opts: number.Options{Style: "currency", Currency: "XYZ"}, want: "XYZ 1,234.50"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := number.Format(tc.locale, tc.value, tc.opts)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestOptions exercises a few representative option combinations.
func TestOptions(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		value  float64
		opts   number.Options
		want   string
	}{
		{name: "half away from zero", locale: "en", value: 2.5, opts: number.Options{MaximumFractionDigits: ptrInt(0)}, want: "3"},
		{name: "away from zero on negatives", locale: "en", value: -2.5, opts: number.Options{MaximumFractionDigits: ptrInt(0)}, want: "-3"},
		{name: "grouping disabled", locale: "en", value: 1234567, opts: number.Options{UseGrouping: ptrBool(false)}, want: "1234567"},
		{name: "minimum integer digits pad", locale: "en", value: 1, opts: number.Options{MinimumIntegerDigits: ptrInt(3)}, want: "001"},
		{name: "maximum significant digits", locale: "en", value: 12345.678, opts: number.Options{MaximumSignificantDigits: ptrInt(3)}, want: "12,300"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := number.Format(tc.locale, tc.value, tc.opts)
			assert.Equal(t, tc.want, got)
		})
	}
}
