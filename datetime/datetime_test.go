package datetime_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/datetime"
)

// jsCase mirrors one entry of testdata/intl_dates.json.
type jsCase struct {
	Locale string         `json:"locale"`
	MS     int64          `json:"ms"`
	Tag    string         `json:"tag"`
	Opts   map[string]any `json:"opts"`
	Value  string         `json:"value"`
}

func toOptions(m map[string]any) datetime.Options {
	var o datetime.Options
	str := func(k string) string {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	o.Weekday = str("weekday")
	o.Era = str("era")
	o.Year = str("year")
	o.Month = str("month")
	o.Day = str("day")
	o.Hour = str("hour")
	o.Minute = str("minute")
	o.Second = str("second")
	o.TimeZoneName = str("timeZoneName")
	o.DateStyle = str("dateStyle")
	o.TimeStyle = str("timeStyle")
	o.DayPeriod = str("dayPeriod")
	o.Calendar = str("calendar")
	o.NumberingSystem = str("numberingSystem")
	// Use the fixture's own timeZone (the generator now stores the merged opts,
	// including timeZone). Default to UTC when absent so legacy fixtures and the
	// UTC-based instants still resolve correctly.
	o.TimeZone = str("timeZone")
	if o.TimeZone == "" {
		o.TimeZone = "UTC"
	}
	if v, ok := m["hour12"]; ok {
		if b, ok := v.(bool); ok {
			o.Hour12 = &b
		}
	}
	if v, ok := m["fractionalSecondDigits"]; ok {
		if f, ok := v.(float64); ok {
			n := int(f)
			o.FractionalSecondDigits = &n
		}
	}
	return o
}

// category groups a case tag into a reporting bucket.
func category(tag string) string {
	switch {
	case strings.HasPrefix(tag, "dateStyle:"):
		return "dateStyle"
	case strings.HasPrefix(tag, "timeStyle:"):
		return "timeStyle"
	case strings.HasPrefix(tag, "both:"):
		return "both(date+time)"
	case strings.HasPrefix(tag, "comp:"):
		return "component"
	case strings.HasPrefix(tag, "dp:"):
		return "dayPeriod"
	case strings.HasPrefix(tag, "zone:"):
		return "zone"
	}
	return "other"
}

func loadCases(t *testing.T) []jsCase {
	t.Helper()
	data, err := os.ReadFile("testdata/intl_dates.json")
	require.NoError(t, err, "read fixtures")
	var cases []jsCase
	require.NoError(t, json.Unmarshal(data, &cases), "parse fixtures")
	require.NotEmpty(t, cases, "no fixtures loaded")
	return cases
}

// TestIntlMatch reports the per-category and per-tag match rates against
// JavaScript's Intl.DateTimeFormat and prints sample divergences. It is a
// diagnostic breakdown; the gating assertions live in TestThresholds. The
// overall rate is intentionally below 100% because two matrix locales, fa and
// th, default to non-Gregorian calendars (Persian, Buddhist) that this package
// does not implement (for those, the named-zone NAMES still match Intl exactly;
// only the numeric hour padding differs). `make gen` pins both the Go tables
// and the Node golden fixtures to the same CLDR release (CLDR 46), so there is
// no CLDR-version skew.
func TestIntlMatch(t *testing.T) {
	cases := loadCases(t)

	type stat struct{ pass, total int }
	byCat := map[string]*stat{}
	byTag := map[string]*stat{}
	// divergence buckets: category -> sample mismatches
	type miss struct{ locale, tag, got, want string }
	var misses []miss

	for _, c := range cases {
		opts := toOptions(c.Opts)
		tm := time.UnixMilli(c.MS).UTC()
		got := datetime.Format(c.Locale, tm, opts)

		cat := category(c.Tag)
		if byCat[cat] == nil {
			byCat[cat] = &stat{}
		}
		if byTag[c.Tag] == nil {
			byTag[c.Tag] = &stat{}
		}
		byCat[cat].total++
		byTag[c.Tag].total++
		if got == c.Value {
			byCat[cat].pass++
			byTag[c.Tag].pass++
		} else {
			misses = append(misses, miss{c.Locale, c.Tag, got, c.Value})
		}
	}

	// Report per category.
	var cats []string
	for k := range byCat {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	totalPass, totalAll := 0, 0
	t.Log("=== Intl match rate by category ===")
	for _, cat := range cats {
		s := byCat[cat]
		totalPass += s.pass
		totalAll += s.total
		t.Logf("  %-16s %5d/%-5d  %.1f%%", cat, s.pass, s.total, 100*float64(s.pass)/float64(s.total))
	}
	t.Logf("  %-16s %5d/%-5d  %.1f%%", "OVERALL", totalPass, totalAll, 100*float64(totalPass)/float64(totalAll))

	// Per-tag breakdown (helps locate weak component skeletons).
	var tags []string
	for k := range byTag {
		tags = append(tags, k)
	}
	sort.Strings(tags)
	t.Log("=== Intl match rate by tag ===")
	for _, tag := range tags {
		s := byTag[tag]
		if s.pass == s.total {
			continue
		}
		t.Logf("  %-22s %4d/%-4d  %.1f%%", tag, s.pass, s.total, 100*float64(s.pass)/float64(s.total))
	}

	// Sample divergences, one per (locale,tag) bucket (cap output).
	t.Log("=== sample divergences (first per locale+tag) ===")
	seen := map[string]bool{}
	shown := 0
	for _, m := range misses {
		key := m.locale + "|" + m.tag
		if seen[key] {
			continue
		}
		seen[key] = true
		t.Logf("  [%s %s] got=%q want=%q", m.locale, m.tag, m.got, m.want)
		shown++
		if shown > 80 {
			break
		}
	}
}

// TestThresholds asserts a minimum match rate so regressions fail the build.
// Style-based formats come straight from CLDR patterns and must match almost
// perfectly; component best-fit is allowed a documented lower bar.
func TestThresholds(t *testing.T) {
	cases := loadCases(t)
	type stat struct{ pass, total int }
	byCat := map[string]*stat{}
	for _, c := range cases {
		opts := toOptions(c.Opts)
		tm := time.UnixMilli(c.MS).UTC()
		got := datetime.Format(c.Locale, tm, opts)
		cat := category(c.Tag)
		if byCat[cat] == nil {
			byCat[cat] = &stat{}
		}
		byCat[cat].total++
		if got == c.Value {
			byCat[cat].pass++
		}
	}
	// Thresholds are set below the measured rates so genuine regressions fail.
	// `make gen` pins both the Go tables and the Node golden fixtures to the same
	// CLDR release (CLDR 46), so there is no CLDR-version skew between them. The
	// only genuine residual is that two matrix locales (fa, th) default to
	// non-Gregorian calendars (Persian, Buddhist) which this package does not
	// implement. Excluding fa/th the overall match rate is ~99%.
	thresholds := map[string]float64{
		"dateStyle":       0.90,
		"timeStyle":       0.99,
		"both(date+time)": 0.90,
		"component":       0.90,
		// dayPeriod: flexible day-period selection from CLDR dayPeriodRules.
		// Measured ~98.9%; the only residual is th (a Buddhist-calendar locale
		// this package does not implement, which also drops its "Hm" literal).
		"dayPeriod": 0.97,
		// zone: named non-UTC zones — specific (short/long), generic
		// (short/long, incl. the country/exemplar-city LOCATION format) and the
		// numeric GMT-offset forms. Excluding the two non-Gregorian-calendar
		// locales (fa, th) the match rate is 100%; the measured overall is ~97.6%.
		// The sole residual is that fa/th default to Persian/Buddhist calendars
		// (not implemented here): the zone NAMES match exactly, only the numeric
		// hour padding ("04:07" vs "4:07") differs for those two locales. The
		// threshold sits just below the measured rate so regressions fail.
		"zone": 0.95,
	}
	for cat, min := range thresholds {
		t.Run(cat, func(t *testing.T) {
			s := byCat[cat]
			require.NotNilf(t, s, "category %q missing from fixtures", cat)
			rate := float64(s.pass) / float64(s.total)
			assert.GreaterOrEqualf(t, rate, min,
				"category %q match rate %.3f below threshold %.3f (%d/%d)",
				cat, rate, min, s.pass, s.total)
		})
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// TestYearZeroAndEra covers the proleptic/astronomical year mapping: Go's year 0
// is 1 BCE, year -1 is 2 BCE. CLDR/ICU print the absolute era year, so year 0
// must render as "1" (and "1 BC" with a short era), not "0".
func TestYearZeroAndEra(t *testing.T) {
	tests := []struct {
		name string
		year int
		opts datetime.Options
		want string
	}{
		{"year0 numeric", 0, datetime.Options{Year: "numeric", TimeZone: "UTC"}, "1"},
		{"year0 short era", 0, datetime.Options{Year: "numeric", Era: "short", TimeZone: "UTC"}, "1 BC"},
		{"yearMinus1 numeric", -1, datetime.Options{Year: "numeric", TimeZone: "UTC"}, "2"},
		{"yearMinus1 short era", -1, datetime.Options{Year: "numeric", Era: "short", TimeZone: "UTC"}, "2 BC"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := time.Date(tc.year, 1, 1, 0, 0, 0, 0, time.UTC)
			assert.Equal(t, tc.want, datetime.Format("en", tm, tc.opts))
		})
	}
}

// TestWeekdayNarrowStandalone covers the standalone (c/e) narrow weekday width.
// A bare weekday:"narrow" request synthesizes a standalone "ccccc" pattern,
// which must render the single-letter narrow name (Intl: Wednesday -> "W"), not
// the short form.
func TestWeekdayNarrowStandalone(t *testing.T) {
	tm := time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC) // Wednesday
	tests := []struct {
		name string
		opts datetime.Options
		want string
	}{
		{"narrow", datetime.Options{Weekday: "narrow", TimeZone: "UTC"}, "W"},
		{"short", datetime.Options{Weekday: "short", TimeZone: "UTC"}, "Wed"},
		{"long", datetime.Options{Weekday: "long", TimeZone: "UTC"}, "Wednesday"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, datetime.Format("en", tm, tc.opts))
		})
	}
}

// TestTimeStyleHourPad covers padding the numeric hour to two digits when a
// style path forces the locale's non-preferred clock (en defaults to 12h; with
// hour12:false the short timeStyle must render "09:07", matching Intl).
func TestTimeStyleHourPad(t *testing.T) {
	tm := time.Date(2021, 1, 6, 9, 7, 0, 0, time.UTC)
	tests := []struct {
		name string
		opts datetime.Options
		want string
	}{
		{"short forced 24h", datetime.Options{TimeStyle: "short", Hour12: boolPtr(false), TimeZone: "UTC"}, "09:07"},
		{"short default", datetime.Options{TimeStyle: "short", TimeZone: "UTC"}, "9:07 AM"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, datetime.Format("en", tm, tc.opts))
		})
	}
}

// TestFractionalSecondDigits covers fractionalSecondDigits, which must inject a
// "." plus the requested S-run after the seconds field (Intl: 12:30:45.123 PM
// for 3 digits, .1 for 1 digit).
func TestFractionalSecondDigits(t *testing.T) {
	tm := time.Date(2021, 1, 6, 12, 30, 45, 123000000, time.UTC)
	tests := []struct {
		name string
		opts datetime.Options
		want string
	}{
		{"3 digits", datetime.Options{Hour: "numeric", Minute: "numeric", Second: "numeric", FractionalSecondDigits: intPtr(3), TimeZone: "UTC"}, "12:30:45.123 PM"},
		{"1 digit", datetime.Options{Hour: "numeric", Minute: "numeric", Second: "numeric", FractionalSecondDigits: intPtr(1), TimeZone: "UTC"}, "12:30:45.1 PM"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, datetime.Format("en", tm, tc.opts))
		})
	}
}

// TestZoneOffsetRendering covers the pure-math GMT offset formats. shortOffset
// (O) drops ":mm" when minutes are zero and never zero-pads the hour (GMT-4,
// GMT+5:30); longOffset (OOOO) always pads to GMT±HH:mm. The requested zone
// width must be honored rather than collapsing to the matched candidate's own
// zone letter. Name-based widths (z/v) for non-UTC zones are data-blocked.
func TestZoneOffsetRendering(t *testing.T) {
	base := datetime.Options{Hour: "numeric", Minute: "2-digit", Second: "2-digit"}
	tests := []struct {
		name   string
		offset int // seconds east of UTC
		tzName string
		want   string
	}{
		{"shortOffset -4", -4 * 3600, "shortOffset", "12:07:08 PM GMT-4"},
		{"longOffset -4", -4 * 3600, "longOffset", "12:07:08 PM GMT-04:00"},
		{"shortOffset +5:30", 5*3600 + 30*60, "shortOffset", "12:07:08 PM GMT+5:30"},
		{"longOffset +5:30", 5*3600 + 30*60, "longOffset", "12:07:08 PM GMT+05:30"},
		{"shortOffset +11", 11 * 3600, "shortOffset", "12:07:08 PM GMT+11"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc := time.FixedZone("X", tc.offset)
			tm := time.Date(2021, 7, 6, 12, 7, 8, 0, loc)
			o := base
			o.TimeZoneName = tc.tzName
			assert.Equal(t, tc.want, datetime.Format("en", tm, o))
		})
	}
}

// TestForced12hPeriodPlacement covers forcing a 12-hour clock onto a locale
// that defaults to a 24-hour clock. The day period must be placed where the
// locale's native 12-hour format puts it (zh/ja place it BEFORE the hour) and
// keep that format's hour width, rather than textually splicing " a" after the
// hour.
func TestForced12hPeriodPlacement(t *testing.T) {
	tm := time.Date(2021, 1, 6, 15, 7, 30, 0, time.UTC)
	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{"zh", "zh", "下午03:07:30"},
		{"ja", "ja", "午後3:07:30"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := datetime.Format(tc.locale, tm, datetime.Options{TimeStyle: "medium", Hour12: boolPtr(true), TimeZone: "UTC"})
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestFlexibleDayPeriod covers the flexible day-period ("B" field) selected via
// the dayPeriod option. The en rules map hours to morning1/afternoon1/
// evening1/night1, with noon only at the exact 12:00 instant. With only an hour
// requested the period is evaluated at hour precision (so 12:30 -> noon), and
// with a 24-hour locale + an hour the period is dropped entirely (de "13 Uhr").
func TestFlexibleDayPeriod(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		h, m   int
		opts   datetime.Options
		want   string
	}{
		{"en night", "en", 1, 30, datetime.Options{Hour: "numeric", DayPeriod: "long", TimeZone: "UTC"}, "1 at night"},
		{"en morning", "en", 6, 30, datetime.Options{Hour: "numeric", DayPeriod: "long", TimeZone: "UTC"}, "6 in the morning"},
		{"en noon-hour", "en", 12, 30, datetime.Options{Hour: "numeric", DayPeriod: "long", TimeZone: "UTC"}, "12 noon"},
		{"en afternoon", "en", 13, 30, datetime.Options{Hour: "numeric", DayPeriod: "long", TimeZone: "UTC"}, "1 in the afternoon"},
		{"en evening", "en", 18, 30, datetime.Options{Hour: "numeric", DayPeriod: "long", TimeZone: "UTC"}, "6 in the evening"},
		{"en noon-narrow", "en", 12, 0, datetime.Options{Hour: "numeric", DayPeriod: "narrow", TimeZone: "UTC"}, "12 n"},
		{"en alone", "en", 13, 30, datetime.Options{DayPeriod: "long", TimeZone: "UTC"}, "in the afternoon"},
		{"de alone", "de", 13, 30, datetime.Options{DayPeriod: "long", TimeZone: "UTC"}, "nachmittags"},
		{"de hour drops period", "de", 13, 30, datetime.Options{Hour: "numeric", DayPeriod: "long", TimeZone: "UTC"}, "13 Uhr"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := time.Date(2021, 1, 1, tc.h, tc.m, 0, 0, time.UTC)
			assert.Equal(t, tc.want, datetime.Format(tc.locale, tm, tc.opts))
		})
	}
}

// TestNoonExactInstant covers that noon is selected only at the exact 12:00:00
// instant (zero minutes/seconds) when seconds are displayed; one second past
// noon resolves to the afternoon period instead.
func TestNoonExactInstant(t *testing.T) {
	base := datetime.Options{Hour: "numeric", Minute: "2-digit", Second: "2-digit", DayPeriod: "long", TimeZone: "UTC"}
	noon := time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "12:00:00 noon", datetime.Format("en", noon, base))
	past := time.Date(2021, 1, 1, 12, 0, 1, 0, time.UTC)
	assert.Equal(t, "12:00:01 in the afternoon", datetime.Format("en", past, base))
}

// TestNamedZones covers the named non-UTC time zones: specific (short/long)
// names selected by DST, metazone generic names, and the generic-LOCATION
// format (country or exemplar city via regionFormat). Verified against Node's
// Intl.DateTimeFormat.
func TestNamedZones(t *testing.T) {
	load := func(n string) *time.Location { l, _ := time.LoadLocation(n); return l }
	// Summer (Jul) and winter (Jan) instants exercise DST selection.
	summer := time.Date(2021, 7, 1, 15, 4, 0, 0, time.UTC)
	winter := time.Date(2021, 1, 5, 15, 4, 0, 0, time.UTC)
	tests := []struct {
		name   string
		locale string
		zone   string
		tzName string
		when   time.Time
		want   string
	}{
		{"NY short daylight", "en", "America/New_York", "short", summer, "11:04 AM EDT"},
		{"NY short standard", "en", "America/New_York", "short", winter, "10:04 AM EST"},
		{"NY long daylight", "en", "America/New_York", "long", summer, "11:04 AM Eastern Daylight Time"},
		{"NY long standard", "en", "America/New_York", "long", winter, "10:04 AM Eastern Standard Time"},
		{"NY shortGeneric metazone", "en", "America/New_York", "shortGeneric", summer, "11:04 AM ET"},
		{"NY longGeneric metazone", "en", "America/New_York", "longGeneric", summer, "11:04 AM Eastern Time"},
		{"London long daylight", "en", "Europe/London", "long", summer, "4:04 PM British Summer Time"},
		{"London long standard", "en", "Europe/London", "long", winter, "3:04 PM Greenwich Mean Time"},
		{"Shanghai long", "en", "Asia/Shanghai", "long", summer, "11:04 PM China Standard Time"},
		{"Shanghai shortGeneric metazone", "en", "Asia/Shanghai", "shortGeneric", summer, "11:04 PM China Time"},
		{"Shanghai longGeneric metazone", "en", "Asia/Shanghai", "longGeneric", summer, "11:04 PM China Standard Time"},
		{"Kolkata long", "en", "Asia/Kolkata", "long", summer, "8:34 PM India Standard Time"},
		{"Sydney long daylight", "en", "Australia/Sydney", "long", winter, "2:04 AM Australian Eastern Daylight Time"},
		{"Sydney longGeneric metazone", "en", "Australia/Sydney", "longGeneric", summer, "1:04 AM Eastern Australia Time"},

		// Generic-LOCATION format (rule 3): COUNTRY name when the zone's territory
		// has a single zone (GB, IN) or is its primaryZone (CN -> Shanghai).
		{"London shortGeneric country", "en", "Europe/London", "shortGeneric", winter, "3:04 PM United Kingdom Time"},
		{"London longGeneric country", "en", "Europe/London", "longGeneric", winter, "3:04 PM United Kingdom Time"},
		{"Kolkata shortGeneric country", "en", "Asia/Kolkata", "shortGeneric", summer, "8:34 PM India Time"},
		{"Shanghai shortGeneric primary not city", "en", "Asia/Shanghai", "shortGeneric", winter, "11:04 PM China Time"},

		// Generic-LOCATION format: EXEMPLAR CITY when the territory is multi-zone
		// and the zone is not primary (US, AU). The city is derived from the zone
		// id and substituted into the per-locale regionFormat.
		{"Sydney shortGeneric city", "en", "Australia/Sydney", "shortGeneric", summer, "1:04 AM Sydney Time"},
		{"NY shortGeneric city (de)", "de", "America/New_York", "shortGeneric", summer, "11:04 New York (Ortszeit)"},
		{"NY shortGeneric city (fr)", "fr", "America/New_York", "shortGeneric", summer, "11:04 heure : New York"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tm := tc.when.In(load(tc.zone))
			o := datetime.Options{Hour: "numeric", Minute: "2-digit", TimeZone: tc.zone, TimeZoneName: tc.tzName}
			assert.Equal(t, tc.want, datetime.Format(tc.locale, tm, o))
		})
	}
}

// ExampleFormat shows the public API.
func ExampleFormat() {
	t := time.Date(2021, 1, 5, 15, 4, 5, 0, time.UTC)
	fmt.Println(datetime.Format("en", t, datetime.Options{DateStyle: "long", TimeStyle: "short", TimeZone: "UTC"}))
	fmt.Println(datetime.Format("fr", t, datetime.Options{DateStyle: "full", TimeStyle: "medium", TimeZone: "UTC"}))
	// Output:
	// January 5, 2021 at 3:04 PM
	// mardi 5 janvier 2021 à 15:04:05
}
