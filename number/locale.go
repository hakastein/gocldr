package number

import "strings"

// symbols holds the locale's number symbols actually used by the formatter.
type symbols struct {
	decimal  string
	group    string
	minus    string
	percent  string
	plus     string
	nan      string
	infinity string
}

// localeData is the fully-resolved data for one locale, assembled from the
// compact generated tables.
type localeData struct {
	locale        string
	sym           symbols
	decimal       string // standard decimal pattern
	percent       string // standard percent pattern
	currency      string // standard currency pattern
	minGrouping   int
	digits        string // numbering-system digit glyphs ("" => latn/ASCII)
	spacingBefore string
	spacingAfter  string
	unitPatterns  map[string]string // currency unitPattern-count-* for name display
}

// currencyInfo holds the resolved currency display data.
type currencyInfo struct {
	code   string
	symbol string
	narrow string
	digits int
	names  map[string]string // plural-count display names
}

// resolveLocale builds a localeData for the requested locale, following the
// CLDR fallback chain: exact -> parentLocale -> truncated subtags -> root.
func resolveLocale(locale string) *localeData {
	key := lookupLocaleKey(locale)
	e := localeTable[key]

	ld := &localeData{
		locale:       locale,
		minGrouping:  int(e.minGrouping),
		unitPatterns: map[string]string{},
	}
	ld.sym = symbols(symbolSets[e.symSet])
	ld.decimal = patternTable[e.decimalPat]
	ld.percent = patternTable[e.percentPat]
	ld.currency = patternTable[e.currencyPat]
	ld.spacingBefore = e.spacingBefore
	ld.spacingAfter = e.spacingAfter
	if e.digitSys != "" {
		ld.digits = numberingSystems[e.digitSys]
	}
	for k, v := range e.unitPatterns {
		ld.unitPatterns[k] = v
	}
	return ld
}

// lookupLocaleKey resolves a locale string to a key present in localeTable,
// applying the CLDR fallback chain.
func lookupLocaleKey(locale string) string {
	loc := canonicalLocaleTag(locale)
	seen := map[string]bool{}
	for loc != "" && !seen[loc] {
		seen[loc] = true
		if _, ok := localeTable[loc]; ok {
			return loc
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
	if _, ok := localeTable["root"]; ok {
		return "root"
	}
	return "en"
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

// resolveCurrency builds currencyInfo for the given ISO code in the locale.
func resolveCurrency(ld *localeData, code string) currencyInfo {
	code = strings.ToUpper(code)
	ci := currencyInfo{code: code, digits: defaultCurrencyDigits, names: map[string]string{}}
	if d, ok := currencyDigits[code]; ok {
		ci.digits = int(d)
	}
	// Locale-specific currency data, resolved through the fallback chain so a
	// currency missing in a region locale inherits from its parent.
	loc := canonicalLocaleTag(ld.locale)
	seen := map[string]bool{}
	for loc != "" && !seen[loc] {
		seen[loc] = true
		if cl, ok := currencyDisplayTable[loc]; ok {
			if idx, ok := cl[code]; ok {
				cd := currencyPool[idx]
				ci.symbol = cd.symbol
				ci.narrow = cd.narrow
				for k, v := range cd.names {
					ci.names[k] = v
				}
				break
			}
		}
		if p, ok := parentLocales[loc]; ok {
			loc = p
			continue
		}
		if i := strings.LastIndexByte(loc, '-'); i >= 0 {
			loc = loc[:i]
			continue
		}
		break
	}
	if ci.symbol == "" {
		ci.symbol = code
	}
	if ci.narrow == "" {
		ci.narrow = ci.symbol
	}
	return ci
}
