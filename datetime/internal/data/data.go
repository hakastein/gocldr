// Package data holds the datetime locale-data type (LocaleData) together with
// the runtime registry that the generated per-locale packages populate.
//
// It lives under internal/ so LocaleData stays out of gocldr's public API while
// remaining importable by the generated gocldr/datetime/locales/* packages. Each
// of those packages registers its locale's data from an init() via Register; the
// datetime core then looks it up with Lookup. A program links only the locale
// packages it (blank-)imports, so the data it pulls in is just what it uses.
package data

// LocaleData holds the resolved CLDR symbol tables and patterns for one locale.
// The concrete values are emitted into the per-locale packages by the generator
// in datetime/internal/gen.
type LocaleData struct {
	MonthsFormat map[string][]string
	MonthsStand  map[string][]string
	DaysFormat   map[string][]string
	DaysStand    map[string][]string
	QuartersFmt  map[string][]string
	QuartersStd  map[string][]string

	DayPeriodsFmt map[string]map[string]string
	DayPeriodsStd map[string]map[string]string
	Eras          map[string][]string

	DateFormats map[string]string
	TimeFormats map[string]string
	DateTime    map[string]string
	AtTime      map[string]string
	Available   map[string]string

	NumberingSystem string
	Zones           map[string]string

	// DayPeriodRules maps a flexible day-period key (morning1, afternoon1,
	// evening1, night1, noon, midnight) to its time range. For a half-open
	// range [from,before) the value is {from, before}; for an exact-point rule
	// (_at, used by noon/midnight) from==before==the point. Times are minutes
	// since 00:00 (0..1440). A range may wrap past midnight (from > before).
	DayPeriodRules map[string][2]int

	// MetazoneNames maps a CLDR metazone id (e.g. "America_Eastern") to its
	// localized names, keyed "<width>.<type>" where width is long|short and
	// type is generic|standard|daylight (e.g. "long.daylight"). Only the
	// sub-keys present in CLDR are emitted.
	MetazoneNames map[string]map[string]string

	// ZoneOverrides maps a CLDR zone id (legacy IANA key, '/'-joined, e.g.
	// "Europe/London") to per-zone name overrides, same "<width>.<type>" keys
	// as MetazoneNames. These take priority over the metazone names.
	ZoneOverrides map[string]map[string]string

	// ExemplarCities maps a CLDR zone id to its localized exemplar city
	// (feeds the regionFormat location fallback for generic names).
	ExemplarCities map[string]string

	// TerritoryNames maps a territory code (e.g. "GB") to its localized display
	// name, limited to the country-representative territories. Used by the
	// generic-location format ("United Kingdom Time").
	TerritoryNames map[string]string
}

// registry holds the locale data registered by the generated per-locale
// packages, keyed by canonical CLDR locale tag.
var registry = map[string]*LocaleData{}

// Register records d under the given locale tag. Generated per-locale packages
// call it from their init().
func Register(locale string, d *LocaleData) { registry[locale] = d }

// Lookup returns the registered data for an exact locale key (no fallback).
func Lookup(key string) (*LocaleData, bool) { d, ok := registry[key]; return d, ok }
