// Package number provides CLDR-driven number, percent and currency formatting
// for Go, generated directly from the Unicode CLDR data (cldr-numbers-full and
// cldr-core). It has ZERO external dependencies and is designed to match the
// behaviour of JavaScript's Intl.NumberFormat (ECMA-402) as closely as possible.
//
// The locale tables in tables_gen.go are produced by the generator in
// internal/gen. To regenerate them, run:
//
//	go generate ./number/...
package number

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hakastein/gocldr/internal/decimal"
	"github.com/hakastein/gocldr/number/internal/data"
)

//go:generate go run ./internal/gen -out tables_gen.go
//go:generate node internal/gen/fixtures.js

// Options mirrors the commonly used subset of Intl.NumberFormatOptions (ECMA-402).
// Pointer fields distinguish "unset" from a zero value, mirroring how Intl
// merges option bags.
type Options struct {
	// Style is "decimal" (default), "percent" or "currency".
	Style string
	// Currency is the ISO 4217 code (e.g. "USD"); required when Style is
	// "currency".
	Currency string
	// CurrencyDisplay is "symbol" (default), "narrowSymbol", "code" or "name".
	CurrencyDisplay string

	// UseGrouping controls digit grouping. The three-valued semantics of
	// Intl ("auto", "always", "min2", true, false) are exposed via
	// UseGroupingMode; UseGrouping is the simple boolean form. When both are
	// unset grouping defaults to locale "auto" behaviour (group, honouring
	// minimumGroupingDigits).
	UseGrouping *bool
	// UseGroupingMode optionally selects Intl's string grouping semantics:
	// "always", "auto", "min2" or "false". It takes precedence over
	// UseGrouping when non-empty.
	UseGroupingMode string

	MinimumIntegerDigits     *int
	MinimumFractionDigits    *int
	MaximumFractionDigits    *int
	MinimumSignificantDigits *int
	MaximumSignificantDigits *int
}

// Format renders value for the given BCP-47 locale and options, matching
// Intl.NumberFormat as closely as possible.
func Format(locale string, value float64, opts Options) string {
	ld := resolveLocale(locale)

	style := opts.Style
	if style == "" {
		style = "decimal"
	}

	// Resolve currency metadata up front (needed for default fraction digits).
	var cur currencyInfo
	if style == "currency" {
		cur = resolveCurrency(ld, opts.Currency)
	}

	// Handle non-finite values the way Intl does.
	if math.IsNaN(value) {
		return ld.Sym.NaN
	}

	// Pick the base pattern.
	display := opts.CurrencyDisplay
	if display == "" {
		display = "symbol"
	}
	var pattern string
	switch style {
	case "percent":
		pattern = ld.Percent
	case "currency":
		if display == "name" {
			// currencyDisplay:"name" uses the decimal pattern plus a unit
			// pattern wrapper.
			pattern = ld.Decimal
		} else {
			pattern = ld.Currency
		}
	default:
		pattern = ld.Decimal
	}

	// Resolve digit-count options into concrete values.
	rs := resolveRounding(style, &opts, cur)

	// Percent multiplies by 100.
	scaled := value
	if style == "percent" {
		scaled = value * 100
	}

	if math.IsInf(scaled, 0) {
		body := ld.Sym.Infinity
		neg := math.Signbit(scaled)
		return wrapPattern(ld, pattern, body, neg, style, display, cur, "")
	}

	// Format the magnitude into integer/fraction digit strings.
	intPart, fracPart := formatMagnitude(math.Abs(scaled), rs)

	// Preserve the sign bit even when the magnitude rounds to zero: Intl renders
	// negative zero (and negatives that round to integer zero) as "-0"/"-0%".
	negative := math.Signbit(scaled)
	// Apply grouping to the integer part.
	grouped := applyGrouping(ld, pattern, intPart, &opts, style)

	body := grouped
	if fracPart != "" {
		body += "." + fracPart
	}

	// For currencyDisplay:"name" the plural category must reflect the digits
	// actually shown (Intl derives plural operands from the formatted number).
	plCat := ""
	if style == "currency" && display == "name" {
		plCat = pluralCategoryForDigits(locale, intPart, fracPart)
	}

	return wrapPattern(ld, pattern, body, negative, style, display, cur, plCat)
}

// roundSpec carries the resolved digit constraints for one Format call.
type roundSpec struct {
	minInt int
	minFr  int
	maxFr  int
	minSig int
	maxSig int
	useSig bool
}

// resolveRounding turns the digit-count options into concrete constraints.
// style selects the fraction-digit defaults; cur supplies the per-currency
// default digits and is only consulted when style is "currency" (Format always
// resolves it for that style).
func resolveRounding(style string, o *Options, cur currencyInfo) roundSpec {
	rs := roundSpec{minInt: 1}
	if o.MinimumIntegerDigits != nil {
		rs.minInt = clampInt(*o.MinimumIntegerDigits, 1, 21)
	}

	if o.MinimumSignificantDigits != nil || o.MaximumSignificantDigits != nil {
		rs.useSig = true
		rs.minSig = 1
		rs.maxSig = 21
		if o.MinimumSignificantDigits != nil {
			rs.minSig = clampInt(*o.MinimumSignificantDigits, 1, 21)
		}
		if o.MaximumSignificantDigits != nil {
			rs.maxSig = clampInt(*o.MaximumSignificantDigits, 1, 21)
		}
		// Intl throws a RangeError for out-of-range or min>max digit counts; as a
		// best-effort, no-panic formatter we clamp into range and raise the
		// maximum up to the minimum instead (intentional divergence).
		if rs.maxSig < rs.minSig {
			rs.maxSig = rs.minSig
		}
		return rs
	}

	// Fraction-digit defaults.
	defMin, defMax := 0, 3
	switch style {
	case "percent":
		defMin, defMax = 0, 0
	case "currency":
		defMin, defMax = cur.digits, cur.digits
	}

	rs.minFr = defMin
	rs.maxFr = defMax
	if o.MinimumFractionDigits != nil {
		rs.minFr = *o.MinimumFractionDigits
		// When only the minimum is set, the maximum rises to at least it.
		if o.MaximumFractionDigits == nil && rs.maxFr < rs.minFr {
			rs.maxFr = rs.minFr
		}
	}
	if o.MaximumFractionDigits != nil {
		rs.maxFr = *o.MaximumFractionDigits
		if o.MinimumFractionDigits == nil && rs.minFr > rs.maxFr {
			rs.minFr = rs.maxFr
		}
	}
	rs.minFr = clampInt(rs.minFr, 0, 100)
	rs.maxFr = clampInt(rs.maxFr, 0, 100)
	if rs.maxFr < rs.minFr {
		rs.maxFr = rs.minFr
	}
	return rs
}

// clampInt constrains v to the inclusive range [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// formatMagnitude rounds and formats the non-negative magnitude into integer
// and fraction digit strings (ASCII), honouring the resolved digit constraints.
// Rounding is half-expand (round half away from zero), the ECMA-402 default.
func formatMagnitude(abs float64, rs roundSpec) (string, string) {
	if rs.useSig {
		return formatSignificant(abs, rs.minSig, rs.maxSig, rs.minInt)
	}

	// Round to maxFr fraction digits, half away from zero.
	intPart, fracPart := decimal.RoundFixed(abs, rs.maxFr)

	// Trim trailing zeros down to minFr.
	fracPart = trimFracTo(fracPart, rs.minFr)

	// Pad integer to minimumIntegerDigits.
	intPart = padInt(intPart, rs.minInt)
	return intPart, fracPart
}

// formatSignificant formats abs with the given significant-digit constraints,
// matching Intl: round to maxSig significant digits (half-expand), then show
// between minSig and maxSig significant digits (trailing zeros beyond minSig are
// trimmed; those needed to reach minSig are kept and significant). It operates
// on the significant-digit string plus the exponent of its most significant
// digit, rounds at the maxSig boundary, then places the decimal point.
func formatSignificant(abs float64, minSig, maxSig, minInt int) (string, string) {
	if abs == 0 {
		intPart := "0"
		if minInt > 1 {
			intPart = strings.Repeat("0", minInt)
		}
		fracDigits := minSig - 1
		if fracDigits <= 0 {
			return intPart, ""
		}
		return intPart, strings.Repeat("0", fracDigits)
	}

	intPart, fracPart := decimal.Shortest(abs)

	// Build the significant-digit string and the exponent of its most
	// significant digit (power of ten of the first significant digit).
	var sig string
	var msdExp int // exponent of the most significant digit
	ni := decimal.NormInt(intPart)
	if ni != "0" {
		// value >= 1
		msdExp = len(ni) - 1
		sig = ni + fracPart
	} else {
		// value < 1: skip leading fraction zeros.
		lead := 0
		for lead < len(fracPart) && fracPart[lead] == '0' {
			lead++
		}
		msdExp = -(lead + 1)
		sig = fracPart[lead:]
	}
	// sig has no leading zeros now. Round to maxSig significant digits.
	if len(sig) > maxSig {
		roundUp := sig[maxSig] >= '5'
		sig = sig[:maxSig]
		if roundUp {
			sig = decimal.Increment(sig)
			if len(sig) > maxSig {
				// carry produced an extra leading digit (e.g. 999->1000);
				// magnitude grew by one.
				msdExp++
				sig = sig[:maxSig]
			}
		}
	}
	// Trim trailing zeros, but keep at least minSig significant digits.
	for len(sig) > minSig && sig[len(sig)-1] == '0' {
		sig = sig[:len(sig)-1]
	}
	// Pad to minSig significant digits.
	if len(sig) < minSig {
		sig += strings.Repeat("0", minSig-len(sig))
	}

	// Place the decimal point: the most significant digit has exponent msdExp.
	// Integer digit count = msdExp+1 (if >=1) else 0.
	intDigitCount := msdExp + 1
	var resInt, resFrac string
	if intDigitCount <= 0 {
		// value < 1: leading zeros then the significant digits.
		resInt = "0"
		resFrac = strings.Repeat("0", -intDigitCount) + sig
	} else if intDigitCount >= len(sig) {
		resInt = sig + strings.Repeat("0", intDigitCount-len(sig))
		resFrac = ""
	} else {
		resInt = sig[:intDigitCount]
		resFrac = sig[intDigitCount:]
	}

	resInt = padInt(resInt, minInt)
	return resInt, resFrac
}

// trimFracTo trims trailing zeros from frac but never below minFr digits.
func trimFracTo(frac string, minFr int) string {
	end := len(frac)
	for end > minFr && frac[end-1] == '0' {
		end--
	}
	frac = frac[:end]
	if len(frac) < minFr {
		frac += strings.Repeat("0", minFr-len(frac))
	}
	return frac
}

// padInt left-pads an integer digit string with zeros to at least minInt.
func padInt(intPart string, minInt int) string {
	if len(intPart) < minInt {
		return strings.Repeat("0", minInt-len(intPart)) + intPart
	}
	return intPart
}

// applyGrouping inserts the locale group separator into the integer digit
// string per the pattern's grouping sizes and the grouping options.
func applyGrouping(ld *data.LocaleData, pattern, intPart string, o *Options, style string) string {
	mode := groupingMode(o)
	if mode == groupOff {
		return intPart
	}

	prim, sec := patternGroupSizes(pattern)
	if prim == 0 {
		return intPart
	}

	n := len(intPart)
	// minimumGroupingDigits / min2: suppress grouping when the integer has too
	// few digits.
	minGroupDigits := ld.MinGrouping
	if mode == groupMin2 && minGroupDigits < 2 {
		minGroupDigits = 2
	}
	if mode == groupAlways {
		minGroupDigits = 1
	}
	if n < prim+minGroupDigits {
		return intPart
	}

	// Build groups from the right. The first (rightmost) group uses prim;
	// subsequent groups use sec.
	var chunks []string
	i := n
	// primary group
	chunks = append(chunks, intPart[i-prim:i])
	i -= prim
	for i > 0 {
		size := sec
		if size <= 0 {
			size = prim
		}
		start := i - size
		if start < 0 {
			start = 0
		}
		chunks = append(chunks, intPart[start:i])
		i = start
	}
	// chunks are right-to-left; reverse and join.
	for l, r := 0, len(chunks)-1; l < r; l, r = l+1, r-1 {
		chunks[l], chunks[r] = chunks[r], chunks[l]
	}
	return strings.Join(chunks, "\x00") // placeholder; replaced by symbol later
}

type groupMode int

const (
	groupAuto groupMode = iota
	groupAlways
	groupMin2
	groupOff
)

func groupingMode(o *Options) groupMode {
	if o.UseGroupingMode != "" {
		switch o.UseGroupingMode {
		case "always", "true":
			return groupAlways
		case "min2":
			return groupMin2
		case "false":
			return groupOff
		case "auto":
			return groupAuto
		}
	}
	if o.UseGrouping != nil {
		if *o.UseGrouping {
			return groupAlways
		}
		return groupOff
	}
	return groupAuto
}

// wrapPattern applies prefix/suffix from the (positive or negative) subpattern,
// substitutes locale symbols and currency markers, applies digit substitution
// for non-latn numbering systems, and returns the final string.
func wrapPattern(ld *data.LocaleData, pattern, body string, negative bool, style, display string, cur currencyInfo, plCat string) string {
	pos, neg := splitSubpatterns(pattern)
	var sub subpattern
	var minus string
	usedNegSub := false
	if negative {
		if neg.set {
			sub = neg
			usedNegSub = true
		} else {
			sub = pos
			minus = ld.Sym.Minus
		}
	} else {
		sub = pos
	}

	// Replace the decimal point FIRST (the body's only literal '.'), then
	// expand grouping placeholders. Doing it in this order avoids confusing a
	// just-inserted group separator that may itself be '.' (e.g. de/es).
	body = strings.Replace(body, ".", ld.Sym.Decimal, 1)
	body = strings.ReplaceAll(body, "\x00", ld.Sym.Group)

	prefix := sub.prefix
	suffix := sub.suffix
	// In an explicit negative subpattern, the literal '-' is the minus-sign
	// placeholder and is rendered using the locale's minusSign symbol (which in
	// some locales, e.g. Arabic, carries bidi marks like LRM).
	if usedNegSub {
		prefix = strings.ReplaceAll(prefix, "-", ld.Sym.Minus)
		suffix = strings.ReplaceAll(suffix, "-", ld.Sym.Minus)
	}

	out := minus + prefix + body + suffix

	// The '%' literal in a pattern maps to the locale percentSign.
	out = strings.ReplaceAll(out, "%", ld.Sym.Percent)

	// Currency handling.
	if style == "currency" {
		if display == "name" {
			out = applyCurrencyName(ld, out, cur, plCat)
		} else {
			out = insertCurrencySymbol(ld, out, cur, display)
		}
	}

	// Digit substitution for non-latn numbering systems.
	if ld.Digits != "" {
		out = substituteDigits(out, ld.Digits)
	}
	return out
}

// substituteDigits maps ASCII digits 0-9 to the numbering system's digit runes.
func substituteDigits(s, digits string) string {
	dr := []rune(digits)
	if len(dr) != 10 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(dr[r-'0'])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// insertCurrencySymbol replaces the ¤ marker with the chosen currency display
// form, honouring currency spacing.
func insertCurrencySymbol(ld *data.LocaleData, s string, cur currencyInfo, display string) string {
	var symbol string
	switch display {
	case "narrowSymbol":
		symbol = cur.narrow
	case "code":
		symbol = cur.code
	default:
		symbol = cur.symbol
	}
	if symbol == "" {
		symbol = cur.code
	}

	idx := strings.IndexRune(s, '¤')
	if idx < 0 {
		return s
	}
	before := s[:idx]
	after := s[idx+len("¤"):]

	// Determine whether the symbol precedes or follows the numeric digits.
	// The symbol precedes the number when a digit appears after the ¤ marker
	// (e.g. "¤#,##0" or the negative "-¤#,##0"); it follows when the digits are
	// before it. Checking for a digit (rather than an empty prefix) handles the
	// minus sign / RTL marks that the negative subpattern places before ¤.
	symbolBefore := containsASCIIDigit(after)

	// Apply currency spacing: if a digit borders the symbol and the symbol's
	// bordering char is not a symbol/separator, insert the locale's
	// insertBetween (typically a NBSP / space).
	if symbolBefore {
		if symbol != "" && needCurrencySpacing(lastRune(symbol), firstRune(after)) {
			return before + symbol + ld.SpacingBefore + after
		}
		return before + symbol + after
	}
	if symbol != "" && needCurrencySpacing(firstRune(symbol), lastRune(before)) {
		return before + ld.SpacingAfter + symbol + after
	}
	return before + symbol + after
}

// containsASCIIDigit reports whether s contains an ASCII digit (the number body
// still uses ASCII digits at currency-insertion time; substitution is later).
func containsASCIIDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			return true
		}
	}
	return false
}

// needCurrencySpacing implements the CLDR currencySpacing rule used here:
// the locale separator is inserted when the symbol's rune facing the number
// is not itself a symbol or space and the number side borders it with a
// digit. Callers pass the two edge runes that end up adjacent.
func needCurrencySpacing(symbolEdge, numberEdge rune) bool {
	return !isSymbolOrSpace(symbolEdge) && numberEdge >= '0' && numberEdge <= '9'
}

// isSymbolOrSpace reports whether r is a symbol, any Unicode separator, or a
// bidi mark (LRM \u200e / RLM \u200f) — runes that already separate the
// currency symbol from the digits, so no extra spacing is needed.
func isSymbolOrSpace(r rune) bool {
	if r == '\u200e' || r == '\u200f' {
		return true
	}
	return unicode.IsSymbol(r) || unicode.In(r, unicode.Z)
}

func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

// applyCurrencyName implements currencyDisplay:"name": it replaces the ¤ marker
// (if present) and wraps the formatted number with the plural-selected currency
// display name via the locale unit pattern.
func applyCurrencyName(ld *data.LocaleData, s string, cur currencyInfo, cat string) string {
	// Remove any ¤ marker that came from the pattern (decimal pattern has none).
	s = strings.ReplaceAll(s, "¤", "")
	name := cur.names[cat]
	if name == "" {
		name = cur.names["other"]
	}
	if name == "" {
		name = cur.code
	}
	pat := ld.UnitPatterns[cat]
	if pat == "" {
		pat = ld.UnitPatterns["other"]
	}
	if pat == "" {
		pat = "{0} {1}"
	}
	pat = strings.Replace(pat, "{0}", s, 1)
	pat = strings.Replace(pat, "{1}", name, 1)
	return pat
}
