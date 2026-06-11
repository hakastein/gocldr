// Command gen reads the Unicode CLDR JSON data (cldr-numbers-full and
// cldr-core) and emits the number locale data. It covers every locale present
// in cldr-numbers-full (~710).
//
// The output is split:
//
//   - The small shared core table is written to -out (tables_gen.go,
//     package number): currencyDigits and parentLocales.
//   - One self-registering package per locale at locales/<tag>/data_gen.go
//     (package locale), carrying that locale's FULLY-RESOLVED, de-interned
//     data.LocaleData (symbols, patterns, currency display with CLDR
//     inheritance pre-applied). A program links only the locales it imports.
//   - locales/all/all_gen.go (package all) blank-imports every per-locale
//     package for callers that want the full set.
//
// CLDR input paths derive from $CLDR_DATA (set by the pinned gen image; run
// via `make gen`, never on the host).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hakastein/gocldr/internal/cldr"
	"github.com/hakastein/gocldr/internal/locale"
)

func main() {
	out := flag.String("out", "tables_gen.go", "output file")
	flag.Parse()

	g := &generator{cldr: cldr.MustDataDir()}
	g.run(*out)
}

type generator struct {
	cldr string

	localeEntries map[string]*localeEntry

	numberingSystems map[string]string
	currencyDigits   map[string]int
	parentLocales    map[string]string

	// currency display: locale -> code -> own (un-inherited) display record.
	currencyDisplay map[string]map[string]displayCurrency
}

// localeEntry is the fully-resolved, de-interned per-locale record assembled
// from CLDR. The currency display map is filled in by resolveCurrencies after
// all locales have been read.
type localeEntry struct {
	sym                         symbolSet
	decimalPat                  string
	percentPat                  string
	currencyPat                 string
	minGrouping                 int
	digits                      string // numbering-system digit glyphs ("" => latn)
	spacingBefore, spacingAfter string
	unitPatterns                map[string]string
	currencies                  map[string]displayCurrency // fully resolved
}

type symbolSet struct {
	decimal, group, minus, percent, nan, infinity string
}

type displayCurrency struct {
	symbol, narrow string
	names          map[string]string
}

func (g *generator) run(out string) {
	g.localeEntries = map[string]*localeEntry{}
	g.currencyDisplay = map[string]map[string]displayCurrency{}

	g.numberingSystems = cldr.LoadNumberingSystems(filepath.Join(g.cldr, "cldr-core", "supplemental", "numberingSystems.json"))
	g.loadCurrencyDigits()
	g.parentLocales = cldr.LoadParentLocales(filepath.Join(g.cldr, "cldr-core", "supplemental", "parentLocales.json"))
	g.loadLocales()
	g.resolveCurrencies()

	g.emit(out)
}

func (g *generator) loadCurrencyDigits() {
	path := filepath.Join(g.cldr, "cldr-core", "supplemental", "currencyData.json")
	var doc struct {
		Supplemental struct {
			CurrencyData struct {
				Fractions map[string]struct {
					Digits string `json:"_digits"`
				} `json:"fractions"`
			} `json:"currencyData"`
		} `json:"supplemental"`
	}
	cldr.MustJSON(path, &doc)
	g.currencyDigits = map[string]int{}
	for code, fr := range doc.Supplemental.CurrencyData.Fractions {
		if code == "DEFAULT" || fr.Digits == "" {
			continue
		}
		if d, err := strconv.Atoi(fr.Digits); err == nil {
			g.currencyDigits[code] = d
		}
	}
}

func (g *generator) loadLocales() {
	mainDir := filepath.Join(g.cldr, "cldr-numbers-full", "main")
	entries, err := os.ReadDir(mainDir)
	if err != nil {
		log.Fatalf("gen: read main dir: %v", err)
	}
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		loc := de.Name()
		g.loadLocale(loc)
	}
}

func (g *generator) loadLocale(loc string) {
	numPath := filepath.Join(g.cldr, "cldr-numbers-full", "main", loc, "numbers.json")
	var top struct {
		Main map[string]struct {
			Numbers map[string]json.RawMessage `json:"numbers"`
		} `json:"main"`
	}
	if !cldr.LoadJSON(numPath, &top) {
		return
	}
	numbers := top.Main[loc].Numbers
	if numbers == nil {
		return
	}

	defaultNS, symRaw := cldr.ResolveNumberDefaults(loc, numbers)
	if symRaw == nil {
		log.Fatalf("gen: %s: no symbols block for %s or latn", loc, defaultNS)
	}
	minGroup := 1
	if mg := jsonString(numbers["minimumGroupingDigits"]); mg != "" {
		if v, err := strconv.Atoi(mg); err == nil {
			minGroup = v
		}
	}

	ss := symbolSet{
		decimal:  symRaw["decimal"],
		group:    symRaw["group"],
		minus:    symRaw["minusSign"],
		percent:  symRaw["percentSign"],
		nan:      symRaw["nan"],
		infinity: symRaw["infinity"],
	}

	// Patterns for the default numbering system (fall back to latn).
	decStd := pattern(numbers, "decimalFormats-numberSystem-"+defaultNS, "decimalFormats-numberSystem-latn", "standard")
	pctStd := pattern(numbers, "percentFormats-numberSystem-"+defaultNS, "percentFormats-numberSystem-latn", "standard")
	curStd := pattern(numbers, "currencyFormats-numberSystem-"+defaultNS, "currencyFormats-numberSystem-latn", "standard")
	if decStd == "" || pctStd == "" || curStd == "" {
		log.Fatalf("gen: %s: missing standard decimal/percent/currency pattern", loc)
	}

	// Currency spacing + unit patterns from currencyFormats.
	spacingBefore, spacingAfter, unitPats := currencyExtras(numbers, defaultNS)

	// Resolve the numbering-system digit glyphs at generation time, so the
	// per-locale record carries them directly (no runtime numbering-system
	// lookup). latn => "" (ASCII digits, no substitution).
	digits := ""
	if defaultNS != "latn" {
		if glyphs, ok := g.numberingSystems[defaultNS]; ok {
			digits = glyphs
		}
	}

	g.localeEntries[loc] = &localeEntry{
		sym:           ss,
		decimalPat:    decStd,
		percentPat:    pctStd,
		currencyPat:   curStd,
		minGrouping:   minGroup,
		digits:        digits,
		spacingBefore: spacingBefore,
		spacingAfter:  spacingAfter,
		unitPatterns:  unitPats,
	}

	// Currency display data (own, un-inherited).
	g.loadCurrencyDisplay(loc)
}

func pattern(numbers map[string]json.RawMessage, key, fallbackKey, field string) string {
	get := func(k string) string {
		if numbers[k] == nil {
			return ""
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(numbers[k], &m); err != nil {
			return ""
		}
		return jsonString(m[field])
	}
	if v := get(key); v != "" {
		return v
	}
	return get(fallbackKey)
}

func currencyExtras(numbers map[string]json.RawMessage, ns string) (string, string, map[string]string) {
	key := "currencyFormats-numberSystem-" + ns
	if numbers[key] == nil {
		key = "currencyFormats-numberSystem-latn"
	}
	if numbers[key] == nil {
		return "", "", nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(numbers[key], &m); err != nil {
		return "", "", nil
	}
	var before, after string
	if m["currencySpacing"] != nil {
		var sp struct {
			Before struct {
				InsertBetween string `json:"insertBetween"`
			} `json:"beforeCurrency"`
			After struct {
				InsertBetween string `json:"insertBetween"`
			} `json:"afterCurrency"`
		}
		if err := json.Unmarshal(m["currencySpacing"], &sp); err == nil {
			before = sp.Before.InsertBetween
			after = sp.After.InsertBetween
		}
	}
	unitPats := map[string]string{}
	for k, v := range m {
		if strings.HasPrefix(k, "unitPattern-count-") {
			cat := strings.TrimPrefix(k, "unitPattern-count-")
			unitPats[cat] = jsonString(v)
		}
	}
	return before, after, unitPats
}

func (g *generator) loadCurrencyDisplay(loc string) {
	path := filepath.Join(g.cldr, "cldr-numbers-full", "main", loc, "currencies.json")
	var top struct {
		Main map[string]struct {
			Numbers struct {
				Currencies map[string]map[string]string `json:"currencies"`
			} `json:"numbers"`
		} `json:"main"`
	}
	if !cldr.LoadJSON(path, &top) {
		return
	}
	curs := top.Main[loc].Numbers.Currencies
	if len(curs) == 0 {
		return
	}

	out := map[string]displayCurrency{}
	for code, fields := range curs {
		dc := displayCurrency{
			symbol: fields["symbol"],
			narrow: fields["symbol-alt-narrow"],
			names:  map[string]string{},
		}
		for k, v := range fields {
			if strings.HasPrefix(k, "displayName-count-") {
				dc.names[strings.TrimPrefix(k, "displayName-count-")] = v
			}
		}
		// Fallback name: plain displayName under "other" if no count keys.
		if len(dc.names) == 0 {
			if dn := fields["displayName"]; dn != "" {
				dc.names["other"] = dn
			}
		}
		// Only store if there is something locale-specific.
		if dc.symbol != "" || dc.narrow != "" || len(dc.names) > 0 {
			out[code] = dc
		}
	}
	if len(out) > 0 {
		g.currencyDisplay[loc] = out
	}
}

// resolveCurrencies fills each locale's fully-resolved Currencies map by walking
// the CLDR fallback chain (exact -> parentLocale -> truncate) and taking, for
// every currency code that appears anywhere in the chain, the FIRST display
// record found (first-hit-wins per code), pre-applied once at gen time.
func (g *generator) resolveCurrencies() {
	for loc, entry := range g.localeEntries {
		resolved := map[string]displayCurrency{}
		locale.Resolve(loc, g.parentLocales, func(cur string) bool {
			for code, dc := range g.currencyDisplay[cur] {
				if _, exists := resolved[code]; !exists {
					resolved[code] = dc
				}
			}
			return false
		})
		if len(resolved) > 0 {
			entry.currencies = resolved
		}
	}
}

func (g *generator) emit(out string) {
	// ---- core shared table (-out, package number) ----
	var b bytes.Buffer
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	p("// Code generated by internal/gen; DO NOT EDIT.\n\n")
	p("package number\n\n")

	// currency digits.
	p("// currencyDigits maps an ISO 4217 code to its CLDR default fraction digits.\n")
	p("// Codes absent here use defaultCurrencyDigits (2).\n")
	p("var currencyDigits = map[string]int8{\n")
	for _, k := range cldr.SortedKeys(g.currencyDigits) {
		p("\t%s: %d,\n", q(k), g.currencyDigits[k])
	}
	p("}\n\n")

	// parent locales.
	p("// parentLocales is the CLDR parentLocale override map for fallback.\n")
	p("var parentLocales = map[string]string{\n")
	for _, k := range cldr.SortedKeys(g.parentLocales) {
		p("\t%s: %s,\n", q(k), q(g.parentLocales[k]))
	}
	p("}\n")

	if err := cldr.WriteFormatted(out, b.Bytes()); err != nil {
		log.Fatal(err)
	}

	// The per-locale packages live under locales/, resolved relative to the
	// directory holding -out (i.e. the number package dir, which is also the
	// //go:generate working directory).
	locKeys := cldr.SortedKeys(g.localeEntries)
	cldr.EmitLocalePackages(filepath.Join(filepath.Dir(out), "locales"), "number",
		"Unicode CLDR cldr-numbers-full / cldr-core.", locKeys,
		func(buf *bytes.Buffer, tag string) { writeLocaleData(buf, g.localeEntries[tag]) })

	log.Printf("gen: wrote %s + %d locales/<tag>/data_gen.go + locales/all: currencyDigits=%d parentLocales=%d",
		out, len(locKeys), len(g.currencyDigits), len(g.parentLocales))
}

// writeLocaleData emits a composite literal for the resolved per-locale record
// (the part after "&data.LocaleData"), matching the data.LocaleData field set.
func writeLocaleData(buf *bytes.Buffer, e *localeEntry) {
	buf.WriteString("{")
	buf.WriteString(fmt.Sprintf("Sym: data.Symbols{Decimal: %s, Group: %s, Minus: %s, Percent: %s, NaN: %s, Infinity: %s}, ",
		q(e.sym.decimal), q(e.sym.group), q(e.sym.minus), q(e.sym.percent), q(e.sym.nan), q(e.sym.infinity)))
	buf.WriteString(fmt.Sprintf("Decimal: %s, ", q(e.decimalPat)))
	buf.WriteString(fmt.Sprintf("Percent: %s, ", q(e.percentPat)))
	buf.WriteString(fmt.Sprintf("Currency: %s, ", q(e.currencyPat)))
	buf.WriteString(fmt.Sprintf("MinGrouping: %d, ", e.minGrouping))
	if e.digits != "" {
		buf.WriteString(fmt.Sprintf("Digits: %s, ", q(e.digits)))
	}
	if e.spacingBefore != "" {
		buf.WriteString(fmt.Sprintf("SpacingBefore: %s, ", q(e.spacingBefore)))
	}
	if e.spacingAfter != "" {
		buf.WriteString(fmt.Sprintf("SpacingAfter: %s, ", q(e.spacingAfter)))
	}
	cldr.WriteStrMap(buf, "UnitPatterns", e.unitPatterns)
	writeCurrencies(buf, e.currencies)
	buf.WriteString("}")
}

func writeCurrencies(buf *bytes.Buffer, m map[string]displayCurrency) {
	if len(m) == 0 {
		return
	}
	buf.WriteString("Currencies: map[string]data.CurrencyDisplay{")
	for _, c := range cldr.SortedKeys(m) {
		dc := m[c]
		buf.WriteString(fmt.Sprintf("%s: {", q(c)))
		buf.WriteString(fmt.Sprintf("Symbol: %s, Narrow: %s", q(dc.symbol), q(dc.narrow)))
		if len(dc.names) > 0 {
			buf.WriteString(", Names: map[string]string{")
			for _, nk := range cldr.SortedKeys(dc.names) {
				buf.WriteString(fmt.Sprintf("%s: %s, ", q(nk), q(dc.names[nk])))
			}
			buf.WriteString("}")
		}
		buf.WriteString("}, ")
	}
	buf.WriteString("}")
}

// ---- helpers ----

func jsonString(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func q(s string) string { return strconv.Quote(s) }
