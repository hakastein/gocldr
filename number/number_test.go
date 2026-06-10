package number_test

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
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
	if len(r.UseGrouping) > 0 {
		var b bool
		if err := json.Unmarshal(r.UseGrouping, &b); err == nil {
			o.UseGrouping = &b
		} else {
			var s string
			if err := json.Unmarshal(r.UseGrouping, &s); err == nil {
				o.UseGroupingMode = s
			}
		}
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
// golden matrix and reports the overall match rate plus a breakdown of
// divergences by (locale, style) bucket.
func TestIntlMatch(t *testing.T) {
	fs := loadFixtures(t)
	total := len(fs)
	matched := 0

	type bucket struct {
		count  int
		sample [3]string
	}
	buckets := map[string]*bucket{}

	for _, f := range fs {
		var ro rawOptions
		require.NoError(t, json.Unmarshal(f.Options, &ro), "parse options")
		got := number.Format(f.Locale, f.Value, ro.toOptions())
		// Per-row assertion: every divergence is reported, not just bucketed.
		assert.Equalf(t, f.Expected, got, "Format(%q, %v, %s)", f.Locale, f.Value, f.Options)
		if got == f.Expected {
			matched++
			continue
		}
		style := ro.Style
		if style == "" {
			style = "decimal"
		}
		key := f.Locale + "/" + style
		if ro.Style == "currency" {
			key += "/" + ro.Currency + "/" + nonEmpty(ro.CurrencyDisplay, "symbol")
		}
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
		}
		if b.count < 3 {
			b.sample[b.count] = sampleStr(f, got)
		}
		b.count++
	}

	rate := float64(matched) / float64(total) * 100
	t.Logf("Intl match rate: %d/%d = %.2f%%", matched, total, rate)

	if len(buckets) > 0 {
		keys := make([]string, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if buckets[keys[i]].count != buckets[keys[j]].count {
				return buckets[keys[i]].count > buckets[keys[j]].count
			}
			return keys[i] < keys[j]
		})
		t.Logf("divergence buckets (%d):", len(buckets))
		for _, k := range keys {
			b := buckets[k]
			samples := b.sample[:]
			var ss []string
			for _, s := range samples {
				if s != "" {
					ss = append(ss, s)
				}
			}
			t.Logf("  %-40s x%-4d  %s", k, b.count, strings.Join(ss, " | "))
		}
	}

	// Gate: require a very high match rate. The generation pipeline is pinned to
	// CLDR 46 (the Go generators reading the cldr-data JSON and Node's full-ICU
	// that dumps these fixtures see the same CLDR release), so tables and
	// fixtures agree by construction and parity is currently 100%. The threshold
	// stays a hair below 100% as a floor: it leaves a small margin for benign
	// future data drift without silently masking an algorithm regression.
	assert.GreaterOrEqualf(t, rate, 99.5, "Intl match rate %.2f%% below 99.5%% threshold", rate)
}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func sampleStr(f fixture, got string) string {
	return jsonQuote(f.Value) + " want=" + quote(f.Expected) + " got=" + quote(got)
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case ' ':
			b.WriteString("<NBSP>")
		case ' ':
			b.WriteString("<NNBSP>")
		case '‎':
			b.WriteString("<LRM>")
		case '‏':
			b.WriteString("<RLM>")
		case '؜':
			b.WriteString("<ALM>")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func jsonQuote(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestUnknownLocaleDegradesToRoot verifies that a locale with nothing in its
// fallback chain registered (here "zz", which truncates to root and has no real
// CLDR data) degrades to the built-in root data — plain ASCII grouped decimals —
// rather than producing empty output or panicking.
func TestUnknownLocaleDegradesToRoot(t *testing.T) {
	got := number.Format("zz", 1234.5, number.Options{})
	assert.Equal(t, "1,234.5", got)
}

// TestOutOfRangeOptionsDoNotPanic verifies digit-count options outside Intl's
// documented ranges are clamped rather than panicking. Intl throws a RangeError;
// this no-panic formatter clamps into range instead.
func TestOutOfRangeOptionsDoNotPanic(t *testing.T) {
	intp := func(v int) *int { return &v }
	cases := []number.Options{
		{MinimumIntegerDigits: intp(0)},
		{MinimumIntegerDigits: intp(-5)},
		{MaximumFractionDigits: intp(1000)},
		{MinimumFractionDigits: intp(-1)},
		{MinimumSignificantDigits: intp(0), MaximumSignificantDigits: intp(0)},
		{MaximumSignificantDigits: intp(10000)},
	}
	for _, opts := range cases {
		assert.NotPanics(t, func() {
			_ = number.Format("en", 1234.5, opts)
		})
	}
}
