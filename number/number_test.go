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
	Style                    string `json:"style"`
	Currency                 string `json:"currency"`
	CurrencyDisplay          string `json:"currencyDisplay"`
	UseGrouping              *bool  `json:"useGrouping"`
	MinimumIntegerDigits     *int   `json:"minimumIntegerDigits"`
	MinimumFractionDigits    *int   `json:"minimumFractionDigits"`
	MaximumFractionDigits    *int   `json:"maximumFractionDigits"`
	MinimumSignificantDigits *int   `json:"minimumSignificantDigits"`
	MaximumSignificantDigits *int   `json:"maximumSignificantDigits"`
}

func (r rawOptions) toOptions() number.Options {
	return number.Options{
		Style:                    r.Style,
		Currency:                 r.Currency,
		CurrencyDisplay:          r.CurrencyDisplay,
		UseGrouping:              r.UseGrouping,
		MinimumIntegerDigits:     r.MinimumIntegerDigits,
		MinimumFractionDigits:    r.MinimumFractionDigits,
		MaximumFractionDigits:    r.MaximumFractionDigits,
		MinimumSignificantDigits: r.MinimumSignificantDigits,
		MaximumSignificantDigits: r.MaximumSignificantDigits,
	}
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

// TestOutOfRangeOptionsClamp pins the clamping contract for digit-count
// options outside Intl's documented ranges (Intl throws a RangeError; this
// no-panic formatter clamps into range instead): the out-of-range bag must
// produce exactly the output of its explicitly clamped equivalent, and that
// output is asserted literally so a wrong clamp cannot slip through.
func TestOutOfRangeOptionsClamp(t *testing.T) {
	intp := func(v int) *int { return &v }
	tests := []struct {
		name    string
		raw     number.Options
		clamped number.Options
		want    string
	}{
		{
			name:    "minimumIntegerDigits 0 clamps to 1",
			raw:     number.Options{MinimumIntegerDigits: intp(0)},
			clamped: number.Options{MinimumIntegerDigits: intp(1)},
			want:    "1,234.5",
		},
		{
			name:    "minimumIntegerDigits -5 clamps to 1",
			raw:     number.Options{MinimumIntegerDigits: intp(-5)},
			clamped: number.Options{MinimumIntegerDigits: intp(1)},
			want:    "1,234.5",
		},
		{
			name:    "maximumFractionDigits 1000 clamps to 100",
			raw:     number.Options{MaximumFractionDigits: intp(1000)},
			clamped: number.Options{MaximumFractionDigits: intp(100)},
			want:    "1,234.5",
		},
		{
			name:    "minimumFractionDigits -1 clamps to 0",
			raw:     number.Options{MinimumFractionDigits: intp(-1)},
			clamped: number.Options{MinimumFractionDigits: intp(0)},
			want:    "1,234.5",
		},
		{
			name:    "significant digits 0 clamp to 1",
			raw:     number.Options{MinimumSignificantDigits: intp(0), MaximumSignificantDigits: intp(0)},
			clamped: number.Options{MinimumSignificantDigits: intp(1), MaximumSignificantDigits: intp(1)},
			want:    "1,000",
		},
		{
			name:    "maximumSignificantDigits 10000 clamps to 21",
			raw:     number.Options{MaximumSignificantDigits: intp(10000)},
			clamped: number.Options{MaximumSignificantDigits: intp(21)},
			want:    "1,234.5",
		},
		{
			name:    "maxSignificant below minSignificant is raised",
			raw:     number.Options{MinimumSignificantDigits: intp(5), MaximumSignificantDigits: intp(2)},
			clamped: number.Options{MinimumSignificantDigits: intp(5), MaximumSignificantDigits: intp(5)},
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
