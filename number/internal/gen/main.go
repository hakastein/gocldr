//go:build ignore

// Command gen reads the Unicode CLDR JSON data (cldr-numbers-full and
// cldr-core) and emits tables_gen.go for package number. It covers every
// locale present in cldr-numbers-full (~710), deduplicating identical symbol
// sets and patterns and storing currency fraction digits once in a supplemental
// table.
//
// Run via: go generate ./number/...
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	// Default CLDR base dir: honour $CLDR_DATA first, then fall back to the
	// host-relative path that was hardcoded before this change so that running
	// go generate without the env var continues to work on the host machine.
	//
	// go generate sets the working directory to the package directory
	// (cldr/number/), which is two levels below the repo root where
	// .reference/cldr-data lives.
	defaultCLDR := os.Getenv("CLDR_DATA")
	if defaultCLDR == "" {
		defaultCLDR = filepath.Join("..", "..", ".reference", "cldr-data", "node_modules")
	}

	cldr := flag.String("cldr", defaultCLDR, "path to node_modules containing cldr-numbers-full and cldr-core")
	out := flag.String("out", "tables_gen.go", "output file")
	flag.Parse()

	g := &generator{cldr: *cldr}
	g.run()

	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		// Write unformatted for debugging.
		_ = os.WriteFile(*out+".err", g.buf.Bytes(), 0o644)
		log.Fatalf("gen: format: %v", err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatalf("gen: write: %v", err)
	}
	log.Printf("gen: wrote %s (%d locales, %d symbol sets, %d patterns, %d currency-display locales)",
		*out, g.localeCount, len(g.symList), len(g.patList), g.curLocaleCount)
}

type generator struct {
	cldr string
	buf  bytes.Buffer

	// dedup tables
	symIndex map[string]int
	symList  []symbolSet
	patIndex map[string]int
	patList  []string

	localeEntries  map[string]localeEntry
	localeCount    int
	curLocaleCount int

	numberingSystems map[string]string
	currencyDigits   map[string]int
	parentLocales    map[string]string

	// currency display: locale -> code -> index into curPool
	currencyDisplay map[string]map[string]int
	curPool         []displayCurrency
	curIndex        map[string]int
}

type symbolSet struct {
	decimal, group, minus, percent, plus, nan, infinity string
}

type localeEntry struct {
	symSet                                          int
	decimalPat, percentPat, currencyPat, accounting int
	minGrouping                                     int
	digitSys                                        string
	spacingBefore, spacingAfter                     string
	unitPatterns                                    map[string]string
}

type displayCurrency struct {
	symbol, narrow string
	names          map[string]string
}

func (g *generator) run() {
	g.symIndex = map[string]int{}
	g.patIndex = map[string]int{}
	g.localeEntries = map[string]localeEntry{}
	g.currencyDisplay = map[string]map[string]int{}
	g.curIndex = map[string]int{}

	g.loadNumberingSystems()
	g.loadCurrencyDigits()
	g.loadParentLocales()
	g.loadLocales()

	g.emit()
}

func (g *generator) loadNumberingSystems() {
	path := filepath.Join(g.cldr, "cldr-core", "supplemental", "numberingSystems.json")
	var doc struct {
		Supplemental struct {
			NumberingSystems map[string]struct {
				Type   string `json:"_type"`
				Digits string `json:"_digits"`
			} `json:"numberingSystems"`
		} `json:"supplemental"`
	}
	mustJSON(path, &doc)
	g.numberingSystems = map[string]string{}
	for name, ns := range doc.Supplemental.NumberingSystems {
		if ns.Type == "numeric" && ns.Digits != "" {
			g.numberingSystems[name] = ns.Digits
		}
	}
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
	mustJSON(path, &doc)
	g.currencyDigits = map[string]int{}
	for code, fr := range doc.Supplemental.CurrencyData.Fractions {
		if code == "DEFAULT" {
			continue
		}
		if fr.Digits != "" {
			if d, err := strconv.Atoi(fr.Digits); err == nil {
				g.currencyDigits[code] = d
			}
		}
	}
}

func (g *generator) loadParentLocales() {
	path := filepath.Join(g.cldr, "cldr-core", "supplemental", "parentLocales.json")
	var doc struct {
		Supplemental struct {
			ParentLocales struct {
				ParentLocale map[string]string `json:"parentLocale"`
			} `json:"parentLocales"`
		} `json:"supplemental"`
	}
	mustJSON(path, &doc)
	g.parentLocales = doc.Supplemental.ParentLocales.ParentLocale
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

// minGroupingOverride patches minimumGroupingDigits for locales where CLDR 46+
// (and thus Node's ICU / JS) differs from the bundled CLDR 45 data.
var minGroupingOverride = map[string]int{
	"ie":    2,
	"it":    2,
	"it-CH": 2,
	"it-SM": 2,
	"it-VA": 2,
	"sl":    2,
}

// icuNumberingOverride pins the numbering system for region-neutral locales
// where ICU/JS disagrees with the bundled CLDR defaultNumberingSystem. Derived
// by comparing CLDR data to Node full-ICU resolvedOptions().numberingSystem.
var icuNumberingOverride = map[string]string{
	"ar":       "latn",
	"az-Arab":  "latn",
	"bgn":      "latn",
	"hnj":      "latn",
	"hnj-Hmnp": "latn",
	"mni-Mtei": "beng",
	"sat-Deva": "olck",
	"sdh":      "latn",
}

func (g *generator) loadLocale(loc string) {
	numPath := filepath.Join(g.cldr, "cldr-numbers-full", "main", loc, "numbers.json")
	raw, err := os.ReadFile(numPath)
	if err != nil {
		return
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	var top struct {
		Main map[string]struct {
			Numbers map[string]json.RawMessage `json:"numbers"`
		} `json:"main"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return
	}
	numbers := top.Main[loc].Numbers
	if numbers == nil {
		return
	}

	defaultNS := jsonString(numbers["defaultNumberingSystem"])
	if defaultNS == "" {
		defaultNS = "latn"
	}
	// ICU/JS quirk: for a handful of region-neutral locales whose CLDR default
	// numbering system is non-latn, Intl.NumberFormat resolves to latn (because
	// the likely-subtags maximization of the bare tag does). Match ICU so the
	// formatter agrees with JS. Region-specific variants (e.g. ar-EG) keep
	// their CLDR default.
	if ns, ok := icuNumberingOverride[loc]; ok {
		defaultNS = ns
	}
	minGroup := 1
	if mg := jsonString(numbers["minimumGroupingDigits"]); mg != "" {
		if v, err := strconv.Atoi(mg); err == nil {
			minGroup = v
		}
	}
	// Data-version patch: CLDR 46 bumped minimumGroupingDigits to 2 for these
	// locales. The bundled cldr-data is CLDR 45 but Node's ICU (and thus JS) is
	// newer, so apply the newer value to match Intl.NumberFormat.
	if minGroupingOverride[loc] != 0 {
		minGroup = minGroupingOverride[loc]
	}

	// Symbols for the default numbering system.
	symKey := "symbols-numberSystem-" + defaultNS
	var symRaw map[string]string
	if numbers[symKey] != nil {
		_ = json.Unmarshal(numbers[symKey], &symRaw)
	}
	if symRaw == nil {
		// fall back to latn symbols.
		_ = json.Unmarshal(numbers["symbols-numberSystem-latn"], &symRaw)
	}
	if symRaw == nil {
		return
	}
	ss := symbolSet{
		decimal:  symRaw["decimal"],
		group:    symRaw["group"],
		minus:    symRaw["minusSign"],
		percent:  symRaw["percentSign"],
		plus:     symRaw["plusSign"],
		nan:      symRaw["nan"],
		infinity: symRaw["infinity"],
	}
	if ss.nan == "" {
		ss.nan = "NaN"
	}
	if ss.infinity == "" {
		ss.infinity = "∞"
	}

	// Patterns for the default numbering system (fall back to latn).
	decStd := g.pattern(numbers, "decimalFormats-numberSystem-"+defaultNS, "decimalFormats-numberSystem-latn", "standard")
	pctStd := g.pattern(numbers, "percentFormats-numberSystem-"+defaultNS, "percentFormats-numberSystem-latn", "standard")
	curStd := g.pattern(numbers, "currencyFormats-numberSystem-"+defaultNS, "currencyFormats-numberSystem-latn", "standard")
	curAcct := g.pattern(numbers, "currencyFormats-numberSystem-"+defaultNS, "currencyFormats-numberSystem-latn", "accounting")

	if decStd == "" {
		decStd = "#,##0.###"
	}
	if pctStd == "" {
		pctStd = "#,##0%"
	}
	if curStd == "" {
		curStd = "¤#,##0.00"
	}
	if curAcct == "" {
		curAcct = curStd
	}

	// Currency spacing + unit patterns from currencyFormats.
	spacingBefore, spacingAfter, unitPats := g.currencyExtras(numbers, defaultNS)

	digitSys := ""
	if defaultNS != "latn" {
		if _, ok := g.numberingSystems[defaultNS]; ok {
			digitSys = defaultNS
		}
	}

	entry := localeEntry{
		symSet:        g.internSym(ss),
		decimalPat:    g.internPat(decStd),
		percentPat:    g.internPat(pctStd),
		currencyPat:   g.internPat(curStd),
		accounting:    g.internPat(curAcct),
		minGrouping:   minGroup,
		digitSys:      digitSys,
		spacingBefore: spacingBefore,
		spacingAfter:  spacingAfter,
		unitPatterns:  unitPats,
	}
	g.localeEntries[loc] = entry
	g.localeCount++

	// Currency display data.
	g.loadCurrencyDisplay(loc)
}

func (g *generator) pattern(numbers map[string]json.RawMessage, key, fallbackKey, field string) string {
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

func (g *generator) currencyExtras(numbers map[string]json.RawMessage, ns string) (string, string, map[string]string) {
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
	if len(unitPats) == 0 {
		unitPats = nil
	}
	return before, after, unitPats
}

func (g *generator) loadCurrencyDisplay(loc string) {
	path := filepath.Join(g.cldr, "cldr-numbers-full", "main", loc, "currencies.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var top struct {
		Main map[string]struct {
			Numbers struct {
				Currencies map[string]map[string]string `json:"currencies"`
			} `json:"numbers"`
		} `json:"main"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return
	}
	curs := top.Main[loc].Numbers.Currencies
	if len(curs) == 0 {
		return
	}
	// Sort currency codes so pool indices are assigned deterministically across
	// runs (Go map iteration order is random; without sorting, the currencyPool
	// in the generated file shuffles on every run even with identical input).
	curCodes := make([]string, 0, len(curs))
	for code := range curs {
		curCodes = append(curCodes, code)
	}
	sort.Strings(curCodes)

	out := map[string]int{}
	for _, code := range curCodes {
		fields := curs[code]
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
		if len(dc.names) == 0 {
			dc.names = nil
		}
		// Only store if there is something locale-specific.
		if dc.symbol != "" || dc.narrow != "" || dc.names != nil {
			out[code] = g.internCur(dc)
		}
	}
	if len(out) > 0 {
		g.currencyDisplay[loc] = out
		g.curLocaleCount++
	}
}

func (g *generator) internCur(dc displayCurrency) int {
	keys := make([]string, 0, len(dc.names))
	for k := range dc.names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var nb strings.Builder
	for _, k := range keys {
		nb.WriteString(k)
		nb.WriteByte('=')
		nb.WriteString(dc.names[k])
		nb.WriteByte('\x1f')
	}
	key := dc.symbol + "\x1e" + dc.narrow + "\x1e" + nb.String()
	if i, ok := g.curIndex[key]; ok {
		return i
	}
	i := len(g.curPool)
	g.curPool = append(g.curPool, dc)
	g.curIndex[key] = i
	return i
}

func (g *generator) internSym(ss symbolSet) int {
	k := strings.Join([]string{ss.decimal, ss.group, ss.minus, ss.percent, ss.plus, ss.nan, ss.infinity}, "\x1f")
	if i, ok := g.symIndex[k]; ok {
		return i
	}
	i := len(g.symList)
	g.symList = append(g.symList, ss)
	g.symIndex[k] = i
	return i
}

func (g *generator) internPat(p string) int {
	if i, ok := g.patIndex[p]; ok {
		return i
	}
	i := len(g.patList)
	g.patList = append(g.patList, p)
	g.patIndex[p] = i
	return i
}

func (g *generator) emit() {
	b := &g.buf
	p := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }

	p("// Code generated by internal/gen; DO NOT EDIT.\n\n")
	p("package number\n\n")

	// symbolSet type + table.
	p("// symbolSet holds a deduplicated set of locale number symbols.\n")
	p("type symbolSet struct {\n")
	p("\tdecimal, group, minus, percent, plus, nan, infinity string\n")
	p("}\n\n")
	p("var symbolSets = [...]symbolSet{\n")
	for _, ss := range g.symList {
		p("\t{%s, %s, %s, %s, %s, %s, %s},\n",
			q(ss.decimal), q(ss.group), q(ss.minus), q(ss.percent), q(ss.plus), q(ss.nan), q(ss.infinity))
	}
	p("}\n\n")

	// pattern table.
	p("// patternTable holds deduplicated CLDR number patterns.\n")
	p("var patternTable = [...]string{\n")
	for _, pat := range g.patList {
		p("\t%s,\n", q(pat))
	}
	p("}\n\n")

	// numbering systems digit map.
	p("// numberingSystems maps a numbering system id to its 10 digit glyphs.\n")
	p("var numberingSystems = map[string]string{\n")
	nsKeys := sortedKeys(g.numberingSystems)
	for _, k := range nsKeys {
		p("\t%s: %s,\n", q(k), q(g.numberingSystems[k]))
	}
	p("}\n\n")

	// currency digits.
	p("// currencyDigits maps an ISO 4217 code to its CLDR default fraction digits.\n")
	p("// Codes absent here use defaultCurrencyDigits (2).\n")
	p("var currencyDigits = map[string]int8{\n")
	cdKeys := sortedIntKeys(g.currencyDigits)
	for _, k := range cdKeys {
		p("\t%s: %d,\n", q(k), g.currencyDigits[k])
	}
	p("}\n\n")

	// parent locales.
	p("// parentLocales is the CLDR parentLocale override map for fallback.\n")
	p("var parentLocales = map[string]string{\n")
	plKeys := sortedKeys(g.parentLocales)
	for _, k := range plKeys {
		p("\t%s: %s,\n", q(k), q(g.parentLocales[k]))
	}
	p("}\n\n")

	// localeEntry type + table.
	p("// localeEntry is the compact per-locale record. Pattern/symbol fields are\n")
	p("// indices into patternTable / symbolSets.\n")
	p("type localeEntry struct {\n")
	p("\tsymSet                                     int32\n")
	p("\tdecimalPat, percentPat, currencyPat, accountingPat int32\n")
	p("\tminGrouping                                int8\n")
	p("\tdigitSys                                   string\n")
	p("\tspacingBefore, spacingAfter                string\n")
	p("\tunitPatterns                               map[string]string\n")
	p("}\n\n")

	p("var localeTable = map[string]localeEntry{\n")
	locKeys := sortedKeys2(g.localeEntries)
	for _, k := range locKeys {
		e := g.localeEntries[k]
		p("\t%s: {%d, %d, %d, %d, %d, %d, %s, %s, %s, %s},\n",
			q(k), e.symSet, e.decimalPat, e.percentPat, e.currencyPat, e.accounting,
			e.minGrouping, q(e.digitSys), q(e.spacingBefore), q(e.spacingAfter),
			mapLit(e.unitPatterns))
	}
	p("}\n\n")

	// currency display.
	p("// displayCurrency holds locale-specific currency display data.\n")
	p("type displayCurrency struct {\n")
	p("\tsymbol, narrow string\n")
	p("\tnames          map[string]string\n")
	p("}\n\n")

	// Global deduplicated pool of displayCurrency values.
	p("// currencyPool is the deduplicated pool of currency display records;\n")
	p("// currencyDisplayTable entries are indices into it.\n")
	p("var currencyPool = [...]displayCurrency{\n")
	for _, dc := range g.curPool {
		p("\t{%s, %s, %s},\n", q(dc.symbol), q(dc.narrow), mapLit(dc.names))
	}
	p("}\n\n")

	p("// currencyDisplayTable maps locale -> ISO code -> index into currencyPool.\n")
	p("var currencyDisplayTable = map[string]map[string]int32{\n")
	cdlKeys := sortedKeys3(g.currencyDisplay)
	for _, loc := range cdlKeys {
		codes := g.currencyDisplay[loc]
		ckeys := make([]string, 0, len(codes))
		for c := range codes {
			ckeys = append(ckeys, c)
		}
		sort.Strings(ckeys)
		p("\t%s: {", q(loc))
		for i, c := range ckeys {
			if i > 0 {
				p(", ")
			}
			p("%s: %d", q(c), codes[c])
		}
		p("},\n")
	}
	p("}\n")
}

// ---- helpers ----

func mustJSON(path string, v any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("gen: read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		log.Fatalf("gen: parse %s: %v", path, err)
	}
}

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

func mapLit(m map[string]string) string {
	if len(m) == 0 {
		return "nil"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("map[string]string{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(q(k))
		b.WriteString(": ")
		b.WriteString(q(m[k]))
	}
	b.WriteString("}")
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys2(m map[string]localeEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys3(m map[string]map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
