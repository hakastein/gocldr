// Package data holds the resolved number locale-data type (LocaleData)
// together with the runtime registry that the generated per-locale packages
// populate.
//
// It lives under internal/ so LocaleData stays out of gocldr's public API while
// remaining importable by the generated gocldr/number/locales/* packages. Each
// of those packages registers its locale's data from an init() via Register;
// the number core then looks it up with Lookup. A program links only the locale
// packages it (blank-)imports, so the data it pulls in is just what it uses.
package data

// Symbols holds the locale's number symbols actually used by the formatter.
type Symbols struct {
	Decimal  string
	Group    string
	Minus    string
	Percent  string
	NaN      string
	Infinity string
}

// CurrencyDisplay holds the currency display data for one ISO code in one
// locale.
type CurrencyDisplay struct {
	Symbol string
	Narrow string
	Names  map[string]string // plural-count display names
}

// LocaleData is the fully-resolved, self-contained data for one locale. The
// concrete values are emitted into the per-locale packages by the generator in
// number/internal/gen.
type LocaleData struct {
	Sym           Symbols
	Decimal       string // standard decimal pattern
	Percent       string // standard percent pattern
	Currency      string // standard currency pattern
	MinGrouping   int
	Digits        string // numbering-system digit glyphs ("" => latn/ASCII)
	SpacingBefore string
	SpacingAfter  string
	UnitPatterns  map[string]string // currency unitPattern-count-* for name display
	// Currencies maps an ISO 4217 code to its FULLY-RESOLVED display
	// (inheritance pre-applied), so the runtime does no fallback walk.
	Currencies map[string]CurrencyDisplay
}

// registry holds the locale data registered by the generated per-locale
// packages, keyed by canonical CLDR locale tag.
var registry = map[string]*LocaleData{}

// Register records d under the given locale tag. Generated per-locale packages
// call it from their init().
func Register(locale string, d *LocaleData) { registry[locale] = d }

// Lookup returns the registered data for an exact locale key (no fallback).
func Lookup(key string) (*LocaleData, bool) { d, ok := registry[key]; return d, ok }
