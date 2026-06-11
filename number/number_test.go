package number_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/number"
	_ "github.com/hakastein/gocldr/number/locales/all"
)

type fixture struct {
	Locale   string          `json:"locale"`
	Value    float64         `json:"value"`
	Options  json.RawMessage `json:"options"`
	Expected string          `json:"expected"`
}

type rawOptions struct {
	Style           string `json:"style"`
	Currency        string `json:"currency"`
	CurrencyDisplay string `json:"currencyDisplay"`
	// useGrouping arrives either as a JSON bool (the legacy Intl form) or as
	// an ES2023 string ("always"/"auto"/"min2"/legacy "true"/"false").
	UseGrouping              json.RawMessage `json:"useGrouping"`
	MinimumIntegerDigits     *int            `json:"minimumIntegerDigits"`
	MinimumFractionDigits    *int            `json:"minimumFractionDigits"`
	MaximumFractionDigits    *int            `json:"maximumFractionDigits"`
	MinimumSignificantDigits *int            `json:"minimumSignificantDigits"`
	MaximumSignificantDigits *int            `json:"maximumSignificantDigits"`
}

func (r rawOptions) toOptions() number.Options {
	o := number.Options{
		Style:                    r.Style,
		Currency:                 r.Currency,
		CurrencyDisplay:          r.CurrencyDisplay,
		MinimumIntegerDigits:     r.MinimumIntegerDigits,
		MinimumFractionDigits:    r.MinimumFractionDigits,
		MaximumFractionDigits:    r.MaximumFractionDigits,
		MinimumSignificantDigits: r.MinimumSignificantDigits,
		MaximumSignificantDigits: r.MaximumSignificantDigits,
	}
	if len(r.UseGrouping) == 0 {
		return o
	}
	var b bool
	if json.Unmarshal(r.UseGrouping, &b) == nil {
		o.UseGrouping = &b
		return o
	}
	var s string
	if json.Unmarshal(r.UseGrouping, &s) == nil {
		o.UseGroupingMode = s
	}
	return o
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/intl_numbers.json")
	require.NoError(t, err, "read fixtures")
	var fs []fixture
	require.NoError(t, json.Unmarshal(raw, &fs), "parse fixtures")
	require.NotEmpty(t, fs, "no fixtures loaded")
	return fs
}

// TestIntlMatch asserts Go Format output equals Intl.NumberFormat over the full
// golden matrix.
func TestIntlMatch(t *testing.T) {
	for _, f := range loadFixtures(t) {
		var ro rawOptions
		require.NoError(t, json.Unmarshal(f.Options, &ro), "parse options")
		got := number.Format(f.Locale, f.Value, ro.toOptions())
		assert.Equalf(t, f.Expected, got, "Format(%q, %v, %s)", f.Locale, f.Value, f.Options)
	}
}

// TestUnknownLocaleDegradesToRoot verifies that a locale with nothing in its
// fallback chain registered (here "zz", which truncates to root and has no real
// CLDR data) degrades to the built-in root data — plain ASCII grouped decimals —
// rather than producing empty output or panicking.
func TestUnknownLocaleDegradesToRoot(t *testing.T) {
	got := number.Format("zz", 1234.5, number.Options{})
	assert.Equal(t, "1,234.5", got)
}

// TestOutOfRangeOptionsClamp pins the clamping contract for digit-count
// options outside Intl's documented ranges (Intl throws a RangeError; this
// no-panic formatter clamps into range instead): the out-of-range bag must
// produce exactly the output of its explicitly clamped equivalent, and that
// output is asserted literally so a wrong clamp cannot slip through.
func TestOutOfRangeOptionsClamp(t *testing.T) {
	tests := []struct {
		name    string
		raw     number.Options
		clamped number.Options
		want    string
	}{
		{
			name:    "minimumIntegerDigits 0 clamps to 1",
			raw:     number.Options{MinimumIntegerDigits: ptrInt(0)},
			clamped: number.Options{MinimumIntegerDigits: ptrInt(1)},
			want:    "1,234.5",
		},
		{
			name:    "minimumIntegerDigits -5 clamps to 1",
			raw:     number.Options{MinimumIntegerDigits: ptrInt(-5)},
			clamped: number.Options{MinimumIntegerDigits: ptrInt(1)},
			want:    "1,234.5",
		},
		{
			name:    "maximumFractionDigits 1000 clamps to 100",
			raw:     number.Options{MaximumFractionDigits: ptrInt(1000)},
			clamped: number.Options{MaximumFractionDigits: ptrInt(100)},
			want:    "1,234.5",
		},
		{
			name:    "minimumFractionDigits -1 clamps to 0",
			raw:     number.Options{MinimumFractionDigits: ptrInt(-1)},
			clamped: number.Options{MinimumFractionDigits: ptrInt(0)},
			want:    "1,234.5",
		},
		{
			name:    "significant digits 0 clamp to 1",
			raw:     number.Options{MinimumSignificantDigits: ptrInt(0), MaximumSignificantDigits: ptrInt(0)},
			clamped: number.Options{MinimumSignificantDigits: ptrInt(1), MaximumSignificantDigits: ptrInt(1)},
			want:    "1,000",
		},
		{
			name:    "maximumSignificantDigits 10000 clamps to 21",
			raw:     number.Options{MaximumSignificantDigits: ptrInt(10000)},
			clamped: number.Options{MaximumSignificantDigits: ptrInt(21)},
			want:    "1,234.5",
		},
		{
			name:    "maxSignificant below minSignificant is raised",
			raw:     number.Options{MinimumSignificantDigits: ptrInt(5), MaximumSignificantDigits: ptrInt(2)},
			clamped: number.Options{MinimumSignificantDigits: ptrInt(5), MaximumSignificantDigits: ptrInt(5)},
			want:    "1,234.5",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := number.Format("en", 1234.5, tc.raw)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, number.Format("en", 1234.5, tc.clamped), got,
				"out-of-range options must behave exactly like their clamped equivalent")
		})
	}
}
