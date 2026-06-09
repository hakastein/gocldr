package number

import (
	"strings"

	"github.com/hakastein/gocldr/number/internal/data"
)

// currencyInfo holds the resolved currency display data returned by
// resolveCurrency. It is an internal helper shape sourced from the registry's
// fully-resolved per-locale Currencies map.
type currencyInfo struct {
	code   string
	symbol string
	narrow string
	digits int
	names  map[string]string // plural-count display names
}

// rootData is the built-in CLDR-root fallback used when nothing in a locale's
// fallback chain is registered. It keeps Format from producing empty output for
// an unknown locale (degrading to plain ASCII grouped decimals instead).
var rootData = &data.LocaleData{
	Sym:          data.Symbols{Decimal: ".", Group: ",", Minus: "-", Percent: "%", Plus: "+", NaN: "NaN", Infinity: "∞"},
	Decimal:      "#,##0.###",
	Percent:      "#,##0%",
	Currency:     "¤#,##0.00",
	MinGrouping:  1,
	UnitPatterns: map[string]string{},
	Currencies:   map[string]data.CurrencyDisplay{},
}

// resolveLocale returns the registered data for the requested locale, following
// the CLDR fallback chain: exact -> parentLocale -> truncated subtags -> root.
// On a total miss (nothing in the chain registered) it returns rootData so
// output is never empty.
func resolveLocale(locale string) *data.LocaleData {
	loc := canonicalLocaleTag(locale)
	seen := map[string]bool{}
	for loc != "" && !seen[loc] {
		seen[loc] = true
		if d, ok := data.Lookup(loc); ok {
			return d
		}
		// parentLocale override.
		if p, ok := parentLocales[loc]; ok {
			loc = p
			continue
		}
		// Truncate trailing subtag.
		if i := strings.LastIndexByte(loc, '-'); i >= 0 {
			loc = loc[:i]
			continue
		}
		break
	}
	if d, ok := data.Lookup("root"); ok {
		return d
	}
	return rootData
}

// canonicalLocaleTag normalises a BCP-47 / CLDR tag for table lookup.
func canonicalLocaleTag(loc string) string {
	loc = strings.ReplaceAll(loc, "_", "-")
	parts := strings.Split(loc, "-")
	for i, p := range parts {
		switch {
		case i == 0:
			parts[i] = strings.ToLower(p)
		case len(p) == 2:
			parts[i] = strings.ToUpper(p)
		case len(p) == 4:
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "-")
}

// resolveCurrency builds currencyInfo for the given ISO code in the locale. The
// per-locale Currencies map is already fully resolved (inheritance pre-applied
// by the generator), so there is no runtime fallback walk.
func resolveCurrency(ld *data.LocaleData, code string) currencyInfo {
	code = strings.ToUpper(code)
	ci := currencyInfo{code: code, digits: defaultCurrencyDigits, names: map[string]string{}}
	if d, ok := currencyDigits[code]; ok {
		ci.digits = int(d)
	}
	if cd, ok := ld.Currencies[code]; ok {
		ci.symbol = cd.Symbol
		ci.narrow = cd.Narrow
		for k, v := range cd.Names {
			ci.names[k] = v
		}
	}
	if ci.symbol == "" {
		ci.symbol = code
	}
	if ci.narrow == "" {
		ci.narrow = ci.symbol
	}
	return ci
}
