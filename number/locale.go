package number

import (
	"strings"

	"github.com/hakastein/gocldr/internal/locale"
	"github.com/hakastein/gocldr/number/internal/data"
)

// currencyInfo holds the resolved currency display data returned by
// resolveCurrency.
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
	Sym:          data.Symbols{Decimal: ".", Group: ",", Minus: "-", Percent: "%", NaN: "NaN", Infinity: "∞"},
	Decimal:      "#,##0.###",
	Percent:      "#,##0%",
	Currency:     "¤#,##0.00",
	MinGrouping:  1,
	UnitPatterns: map[string]string{},
	Currencies:   map[string]data.CurrencyDisplay{},
}

// resolveLocale returns the registered data for the requested locale, following
// the CLDR fallback chain: exact -> parentLocale -> truncated subtags.
// On a total miss (nothing in the chain registered) it returns rootData so
// output is never empty.
func resolveLocale(loc string) *data.LocaleData {
	if tag, ok := locale.Resolve(loc, parentLocales, registered); ok {
		d, _ := data.Lookup(tag)
		return d
	}
	return rootData
}

func registered(tag string) bool {
	_, ok := data.Lookup(tag)
	return ok
}

// defaultCurrencyDigits is the CLDR DEFAULT fraction-digit count for currencies
// not listed in the currencyData fractions table.
const defaultCurrencyDigits = 2

// resolveCurrency builds currencyInfo for the given ISO code in the locale.
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
