package plural_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/plural"
)

func TestOperandsFromString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want plural.Operands
	}{
		{name: "integer", in: "1", want: plural.Operands{N: 1, I: 1, V: 0, W: 0, F: 0, T: 0, C: 0}},
		{name: "one trailing zero", in: "1.0", want: plural.Operands{N: 1, I: 1, V: 1, W: 0, F: 0, T: 0, C: 0}},
		{name: "two fraction digits", in: "1.50", want: plural.Operands{N: 1.5, I: 1, V: 2, W: 1, F: 50, T: 5, C: 0}},
		{name: "three fraction digits", in: "1.230", want: plural.Operands{N: 1.23, I: 1, V: 3, W: 2, F: 230, T: 23, C: 0}},
		{name: "zero", in: "0", want: plural.Operands{N: 0, I: 0}},
		{name: "negative", in: "-7.5", want: plural.Operands{N: 7.5, I: 7, V: 1, W: 1, F: 5, T: 5}},
		{name: "million", in: "1000000", want: plural.Operands{N: 1000000, I: 1000000}},
		// compact exponent scales the value; c retains the exponent.
		{name: "compact exponent", in: "1c6", want: plural.Operands{N: 1000000, I: 1000000, C: 6}},
		{name: "compact with fraction", in: "1.0000001c6", want: plural.Operands{N: 1000000.1, I: 1000000, V: 1, W: 1, F: 1, T: 1, C: 6}},
		{name: "scientific exponent (e alias)", in: "1.2e6", want: plural.Operands{N: 1200000, I: 1200000, C: 6}},
		{name: "negative exponent shifts left", in: "1.5e-3", want: plural.Operands{N: 0.0015, I: 0, V: 4, W: 4, F: 15, T: 15, C: -3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := plural.OperandsFromString(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewOperands(t *testing.T) {
	tests := []struct {
		name       string
		n          float64
		minF, maxF int
		wantV      int
		wantW      int
		wantI      int64
		wantF      int64
		wantT      int64
	}{
		{name: "integer no fraction", n: 1, minF: 0, maxF: 0, wantV: 0, wantW: 0, wantI: 1, wantF: 0, wantT: 0},
		{name: "min fraction pads to 1.0", n: 1, minF: 1, maxF: 3, wantV: 1, wantW: 0, wantI: 1, wantF: 0, wantT: 0},
		{name: "1.5 trimmed", n: 1.5, minF: 0, maxF: 3, wantV: 1, wantW: 1, wantI: 1, wantF: 5, wantT: 5},
		{name: "1.5 padded to two", n: 1.5, minF: 2, maxF: 2, wantV: 2, wantW: 1, wantI: 1, wantF: 50, wantT: 5},
		{name: "2.0 with min fraction", n: 2.0, minF: 1, maxF: 1, wantV: 1, wantW: 0, wantI: 2, wantF: 0, wantT: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := plural.NewOperands(tc.n, tc.minF, tc.maxF)
			assert.Equal(t, tc.wantV, o.V, "V")
			assert.Equal(t, tc.wantW, o.W, "W")
			assert.Equal(t, tc.wantI, o.I, "I")
			assert.Equal(t, tc.wantF, o.F, "F")
			assert.Equal(t, tc.wantT, o.T, "T")
		})
	}
}

func TestLocaleFallback(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		n      float64
		want   plural.Category
	}{
		// en-US should fall back to en (one/other).
		{name: "en-US 1 -> one", locale: "en-US", n: 1, want: plural.One},
		{name: "en-US 2 -> other", locale: "en-US", n: 2, want: plural.Other},
		// pt-PT is region-specific: 1 -> one (i=1,v=0), but 0 -> other (unlike pt).
		{name: "pt-PT 1 -> one", locale: "pt-PT", n: 1, want: plural.One},
		{name: "pt-PT 0 -> other", locale: "pt-PT", n: 0, want: plural.Other},
		// pt: 0 and 1 are both one.
		{name: "pt 0 -> one", locale: "pt", n: 0, want: plural.One},
		// unknown locale -> root/other.
		{name: "unknown locale -> other", locale: "zz", n: 5, want: plural.Other},
		// underscore form normalises.
		{name: "pt_PT 0 -> other", locale: "pt_PT", n: 0, want: plural.Other},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := plural.CardinalFor(tc.locale, tc.n, 0, 0)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestNewOperandsHalfAwayRounding asserts NewOperands rounds half away from
// zero (ECMA-402/ICU halfExpand), not Go's default half-to-even, and that the
// rounding is decimal-string precise (so the plural category agrees with the
// number the formatter displays). Expected values are confirmed against Node's
// Intl.PluralRules / Intl.NumberFormat (CLDR 46).
func TestNewOperandsHalfAwayRounding(t *testing.T) {
	tests := []struct {
		name       string
		n          float64
		minF, maxF int
		// expected operands of the ROUNDED value (what Intl would display).
		wantI int64
		wantV int
		wantW int
		wantF int64
		wantT int64
		// English cardinal category for the same min/max.
		wantCat plural.Category
	}{
		// 0.5 rounded to 0 frac: half away from zero -> 1 (Go half-to-even -> 0).
		{name: "0.5 maxFrac0", n: 0.5, minF: 0, maxF: 0, wantI: 1, wantV: 0, wantW: 0, wantF: 0, wantT: 0, wantCat: plural.One},
		// 2.5 rounded to 0 frac -> 3 (Go half-to-even -> 2).
		{name: "2.5 maxFrac0", n: 2.5, minF: 0, maxF: 0, wantI: 3, wantV: 0, wantW: 0, wantF: 0, wantT: 0, wantCat: plural.Other},
		// 8.575 rounded to 2 frac: decimal-string half-away -> 8.58. Naive float
		// math (FormatFloat 'f' or abs*100) yields 8.57, disagreeing with Intl.
		{name: "8.575 maxFrac2 precision tie", n: 8.575, minF: 2, maxF: 2, wantI: 8, wantV: 2, wantW: 2, wantF: 58, wantT: 58, wantCat: plural.Other},
		// 1.255 rounded to 2 frac -> 1.26 (FormatFloat 'f' gives 1.25).
		{name: "1.255 maxFrac2 precision tie", n: 1.255, minF: 2, maxF: 2, wantI: 1, wantV: 2, wantW: 2, wantF: 26, wantT: 26, wantCat: plural.Other},
		// 0.015 rounded to 2 frac -> 0.02 (FormatFloat 'f' gives 0.01).
		{name: "0.015 maxFrac2 precision tie", n: 0.015, minF: 2, maxF: 2, wantI: 0, wantV: 2, wantW: 2, wantF: 2, wantT: 2, wantCat: plural.Other},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := plural.NewOperands(tc.n, tc.minF, tc.maxF)
			assert.Equal(t, tc.wantI, o.I, "I")
			assert.Equal(t, tc.wantV, o.V, "V")
			assert.Equal(t, tc.wantW, o.W, "W")
			assert.Equal(t, tc.wantF, o.F, "F")
			assert.Equal(t, tc.wantT, o.T, "T")
			assert.Equal(t, tc.wantCat, plural.CardinalFor("en", tc.n, tc.minF, tc.maxF), "cardinal en")
		})
	}
}

func TestRussianCardinal(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want plural.Category
	}{
		{name: "1 -> one", n: 1, want: plural.One},
		{name: "21 -> one", n: 21, want: plural.One},
		{name: "2 -> few", n: 2, want: plural.Few},
		{name: "3 -> few", n: 3, want: plural.Few},
		{name: "4 -> few", n: 4, want: plural.Few},
		{name: "5 -> many", n: 5, want: plural.Many},
		{name: "11 -> many", n: 11, want: plural.Many},
		{name: "12 -> many", n: 12, want: plural.Many},
		{name: "0 -> many", n: 0, want: plural.Many},
		{name: "100 -> many", n: 100, want: plural.Many},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := plural.CardinalFor("ru", float64(tc.n), 0, 0)
			assert.Equal(t, tc.want, got)
		})
	}
}
