package datetime

import (
	"strconv"
	"strings"
	"time"

	"github.com/hakastein/gocldr/datetime/internal/data"
	"github.com/hakastein/gocldr/internal/digits"
)

// formatCtx carries the resolved locale data and options through formatting.
type formatCtx struct {
	ld     *data.LocaleData
	locale string
	digits []rune
	opts   Options
	// zoneID is the CLDR (legacy) zone key for the requested IANA time zone,
	// used to look up metazone / zone-name / territory data. Empty when no zone
	// was given.
	zoneID string
	// canonicalZone is the canonical IANA id as requested (before the
	// legacy-alias translation), used to derive an exemplar city when CLDR has
	// no explicit one (e.g. "America/New_York" -> "New York").
	canonicalZone string
	// zoneObservesDST records whether the requested zone observes daylight
	// saving anywhere in the surrounding year. ICU resolves a LONG generic name
	// to the zone's standard name when the zone never observes DST.
	zoneObservesDST bool
	// isUTC records whether t is rendered in UTC, which uses the dedicated CLDR
	// UTC zone names instead of metazone names or GMT offsets.
	isUTC bool
	// pool caches the best-fit candidate pool built by skeletonPool.
	pool map[string]string
}

// forcePad24 reports the ICU rule that a "numeric" hour is still padded to two
// digits when the caller forces the locale's non-preferred clock — i.e. requests
// a 24-hour clock (Hour12 == false) in a locale that prefers 12-hour. The read
// sites are reached only when an hour field is actually being rendered.
func (c *formatCtx) forcePad24() bool {
	return c.opts.Hour12 != nil && !*c.opts.Hour12 && c.localeUses12()
}

// zoneKeepsHourWidth reports an ICU quirk: a "numeric" hour rendered by a
// 24-hour pattern that also carries a zone field but NO seconds keeps the
// matched pattern's two-digit width (e.g. de "09:07 UTC"), whereas the same
// request without the zone — or with seconds — is unpadded ("9:07", "9:07:02").
func (c *formatCtx) zoneKeepsHourWidth() bool {
	return c.opts.Hour == "numeric" && c.opts.TimeZoneName != "" && c.opts.Second == ""
}

// pad formats an integer in ASCII digits, zero-padded to at least width.
func pad(v, width int) string {
	s := strconv.Itoa(v)
	if len(s) < width {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return s
}

// num formats an integer with the locale's numbering-system digits, zero-padded
// to at least minWidth.
func (c *formatCtx) num(v, minWidth int) string {
	return digits.Substitute(pad(v, minWidth), c.digits)
}

// patternToken is one token of a CLDR pattern: a field run (letter != 0,
// repeated count times) or a literal segment (letter == 0). text is the raw
// source slice, quote characters included, so concatenating the texts of all
// tokens reproduces the pattern.
type patternToken struct {
	letter rune
	count  int
	text   string
}

// literal resolves CLDR quoting in a literal token's text: surrounding quotes
// are dropped and the escaped ” becomes one apostrophe.
func (t patternToken) literal() string {
	s := t.text
	if !strings.HasPrefix(s, "'") {
		return s
	}
	if s == "''" {
		return "'"
	}
	s = strings.TrimSuffix(s[1:], "'")
	return strings.ReplaceAll(s, "''", "'")
}

// tokenizePattern splits a CLDR pattern into field runs (maximal runs of one
// unquoted ASCII letter) and literal segments. A quoted section — including
// any ” escapes inside it — forms a single literal token; unquoted non-letter
// characters form one literal token per run.
func tokenizePattern(pattern string) []patternToken {
	var toks []patternToken
	runes := []rune(pattern)
	n := len(runes)
	lit := -1
	flush := func(end int) {
		if lit >= 0 {
			toks = append(toks, patternToken{text: string(runes[lit:end])})
			lit = -1
		}
	}
	for i := 0; i < n; {
		ch := runes[i]
		switch {
		case ch == '\'':
			flush(i)
			j := i + 1
			for j < n {
				if runes[j] == '\'' {
					if j+1 < n && runes[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j < n {
				j++ // consume the closing quote
			}
			toks = append(toks, patternToken{text: string(runes[i:j])})
			i = j
		case isPatternLetter(ch):
			flush(i)
			j := i
			for j < n && runes[j] == ch {
				j++
			}
			toks = append(toks, patternToken{letter: ch, count: j - i, text: string(runes[i:j])})
			i = j
		default:
			if lit < 0 {
				lit = i
			}
			i++
		}
	}
	flush(n)
	return toks
}

func joinTokens(toks []patternToken) string {
	var b strings.Builder
	for _, t := range toks {
		b.WriteString(t.text)
	}
	return b.String()
}

// interpret runs the CLDR pattern over t, emitting localized text.
func (c *formatCtx) interpret(pattern string, t time.Time) string {
	var b strings.Builder
	for _, tok := range tokenizePattern(pattern) {
		if tok.letter != 0 {
			b.WriteString(c.field(tok.letter, tok.count, t))
		} else {
			b.WriteString(tok.literal())
		}
	}
	return b.String()
}

func isPatternLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// field renders a single CLDR field (letter repeated count times) for t.
func (c *formatCtx) field(ch rune, count int, t time.Time) string {
	switch ch {
	case 'G': // era
		return c.era(count, t)
	case 'y', 'Y', 'u', 'r': // year (Y=week-year, r=related Gregorian, both ≈ year here)
		return c.year(count, t)
	case 'M', 'L': // month (format vs stand-alone)
		return c.month(ch, count, t)
	case 'd': // day of month
		return c.num(t.Day(), count)
	case 'E', 'e', 'c': // weekday
		return c.weekday(ch, count, t)
	case 'a', 'b': // am/pm (b also handles noon/midnight)
		return c.dayPeriod(ch, count, t)
	case 'B': // flexible day period (morning1/afternoon1/evening1/night1/noon)
		return c.flexDayPeriod(count, t)
	case 'h': // hour 1-12
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		return c.num(h, count)
	case 'H': // hour 0-23
		return c.num(t.Hour(), count)
	case 'k': // hour 1-24
		h := t.Hour()
		if h == 0 {
			h = 24
		}
		return c.num(h, count)
	case 'K': // hour 0-11
		return c.num(t.Hour()%12, count)
	case 'm': // minute
		return c.num(t.Minute(), count)
	case 's': // second
		return c.num(t.Second(), count)
	case 'S': // fractional second
		return c.fraction(count, t)
	case 'Q', 'q': // quarter
		return c.quarter(ch, count, t)
	case 'z', 'Z', 'O', 'v', 'V', 'X', 'x': // zone
		return c.zone(ch, count, t)
	}
	// Unknown letter: emit as-is.
	return strings.Repeat(string(ch), count)
}

func (c *formatCtx) era(count int, t time.Time) string {
	idx := 1 // AD
	if t.Year() <= 0 {
		idx = 0 // BC
	}
	var width string
	switch {
	case count >= 5:
		width = "narrow"
	case count == 4:
		width = "names"
	default:
		width = "abbr"
	}
	if arr, ok := c.ld.Eras[width]; ok && idx < len(arr) && arr[idx] != "" {
		return arr[idx]
	}
	if arr, ok := c.ld.Eras["abbr"]; ok && idx < len(arr) {
		return arr[idx]
	}
	return ""
}

func (c *formatCtx) year(count int, t time.Time) string {
	y := t.Year()
	if y <= 0 {
		y = -y + 1 // proleptic: year 0 -> 1 BC, -1 -> 2 BC; CLDR uses absolute era year
	}
	if count == 2 {
		return c.num(y%100, 2)
	}
	return c.num(y, count)
}

func (c *formatCtx) month(ch rune, count int, t time.Time) string {
	m := int(t.Month()) - 1
	if count <= 2 {
		return c.num(m+1, count)
	}
	width := widthFor(count) // 3->abbreviated,4->wide,5->narrow
	table := c.ld.MonthsFormat
	if ch == 'L' {
		table = c.ld.MonthsStand
	}
	if arr, ok := table[width]; ok && m < len(arr) {
		return arr[m]
	}
	if arr, ok := c.ld.MonthsFormat[width]; ok && m < len(arr) {
		return arr[m]
	}
	return c.num(m+1, count)
}

func widthFor(count int) string {
	switch {
	case count >= 5:
		return "narrow"
	case count == 4:
		return "wide"
	default:
		return "abbreviated"
	}
}

// dayPeriodOptionWidth maps the DayPeriod option to a CLDR name width; ""
// when the option is unset.
func dayPeriodOptionWidth(opt string) string {
	switch opt {
	case "narrow":
		return "narrow"
	case "short":
		return "abbreviated"
	case "long":
		return "wide"
	}
	return ""
}

func (c *formatCtx) weekday(ch rune, count int, t time.Time) string {
	wd := int(t.Weekday()) // Sunday=0
	// e/c can be numeric for count<=2 (local day-of-week). E is always text.
	if (ch == 'e' || ch == 'c') && count <= 2 {
		// local day of week: 1..7 with Sunday=1 in CLDR root ordering.
		local := wd + 1
		return c.num(local, count)
	}
	// CLDR widths by count are identical for E (format) and e/c (the e/c
	// numeric forms are handled above at count<=2): 6+ short, 5 narrow, 4 wide,
	// else abbreviated.
	var width string
	switch {
	case count >= 6:
		width = "short"
	case count == 5:
		width = "narrow"
	case count == 4:
		width = "wide"
	default:
		width = "abbreviated"
	}
	table := c.ld.DaysFormat
	if ch == 'c' {
		table = c.ld.DaysStand
	}
	if arr, ok := table[width]; ok && wd < len(arr) {
		return arr[wd]
	}
	if arr, ok := c.ld.DaysFormat["abbreviated"]; ok && wd < len(arr) {
		return arr[wd]
	}
	return ""
}

func (c *formatCtx) dayPeriod(ch rune, count int, t time.Time) string {
	// Per LDML, a/aa/aaa are abbreviated and aaaa is wide, but ICU/Intl render
	// the AM/PM "a" field using the WIDE day-period names (e.g. Korean uses
	// "오후" for "a", not the abbreviated "PM"). We therefore default to wide for
	// count<=4 and use narrow only for the 5-letter form.
	width := "wide"
	if count >= 5 {
		width = "narrow"
	}
	// DayPeriod option ("long"/"short"/"narrow") refines the width when set.
	if w := dayPeriodOptionWidth(c.opts.DayPeriod); w != "" {
		width = w
	}
	table := c.ld.DayPeriodsFmt
	key := "am"
	h, m, s, ns := t.Hour(), t.Minute(), t.Second(), t.Nanosecond()
	if h >= 12 {
		key = "pm"
	}
	// 'b' uses midnight/noon, but only at the exact instant (matching Intl,
	// which requires zero minutes/seconds/nanoseconds).
	if ch == 'b' && m == 0 && s == 0 && ns == 0 {
		if h == 0 {
			if v := lookupPeriod(table, width, "midnight"); v != "" {
				return v
			}
		}
		if h == 12 {
			if v := lookupPeriod(table, width, "noon"); v != "" {
				return v
			}
		}
	}
	if v := lookupPeriod(table, width, key); v != "" {
		return v
	}
	return ""
}

// flexDayPeriod renders the flexible day-period field 'B'.
//
// Effective minute/second resolution mirrors Intl: the day period is evaluated
// at the precision the formatter displays. When the request shows an hour but
// no minute, the minute/second are treated as zero (so e.g. 12:30 with an
// hour-only request resolves to noon, exactly as Intl does).
func (c *formatCtx) flexDayPeriod(count int, t time.Time) string {
	rules := c.ld.DayPeriodRules
	if len(rules) == 0 {
		return c.dayPeriod('a', count, t)
	}
	width := widthFor(count)
	// DayPeriod option width overrides the field-count width.
	if w := dayPeriodOptionWidth(c.opts.DayPeriod); w != "" {
		width = w
	}

	h := t.Hour()
	m, s, ns := t.Minute(), t.Second(), t.Nanosecond()
	// Effective precision: if minute is not part of the request, Intl evaluates
	// the period as if minute/second were zero (truncate to the hour).
	if c.opts.Minute == "" && c.opts.Second == "" && c.opts.FractionalSecondDigits == nil {
		m, s, ns = 0, 0, 0
	}

	table := c.ld.DayPeriodsFmt
	// Exact noon takes priority. (Intl never emits "midnight" for the flexible
	// period in practice — a wrapping night/evening rule wins at 00:00 — so we
	// deliberately do NOT special-case midnight here.)
	if h == 12 && m == 0 && s == 0 && ns == 0 {
		if v := lookupPeriod(table, width, "noon"); v != "" {
			return v
		}
	}

	mins := h*60 + m
	if key := matchDayPeriodRule(rules, mins); key != "" {
		if v := lookupPeriod(table, width, key); v != "" {
			return v
		}
	}
	// Fallback: am/pm.
	return c.dayPeriod('a', count, t)
}

// matchDayPeriodRule returns the range day-period key (morning1/afternoon1/
// evening1/night1/...) whose half-open range [from,before) contains mins
// (minutes since 00:00). Ranges may wrap past midnight (from > before). The
// exact-point rules (noon/midnight, stored as from==before) are skipped here;
// noon is handled separately by the caller.
func matchDayPeriodRule(rules map[string][2]int, mins int) string {
	for key, r := range rules {
		from, before := r[0], r[1]
		if from == before {
			continue // exact-point rule (noon/midnight): handled elsewhere
		}
		if from < before {
			if mins >= from && mins < before {
				return key
			}
		} else {
			// wrapping range, e.g. night1 21:00 -> 06:00.
			if mins >= from || mins < before {
				return key
			}
		}
	}
	return ""
}

func lookupPeriod(table map[string]map[string]string, width, key string) string {
	if mm, ok := table[width]; ok {
		if v, ok := mm[key]; ok {
			return v
		}
	}
	// width fallback
	for _, w := range []string{"wide", "abbreviated", "narrow"} {
		if mm, ok := table[w]; ok {
			if v, ok := mm[key]; ok {
				return v
			}
		}
	}
	return ""
}

func (c *formatCtx) quarter(ch rune, count int, t time.Time) string {
	q := (int(t.Month()) - 1) / 3
	if count <= 2 {
		return c.num(q+1, count)
	}
	width := widthFor(count)
	table := c.ld.QuartersFmt
	if ch == 'q' {
		table = c.ld.QuartersStd
	}
	if arr, ok := table[width]; ok && q < len(arr) {
		return arr[q]
	}
	return c.num(q+1, count)
}

func (c *formatCtx) fraction(count int, t time.Time) string {
	full := pad(t.Nanosecond(), 9)
	if count > 9 {
		full += strings.Repeat("0", count-9)
	}
	return digits.Substitute(full[:count], c.digits)
}

// zone renders a time zone field: the CLDR UTC zone names for UTC, the
// metazone / per-zone names for other zones, with a GMT offset formatted via
// the CLDR gmtFormat as the fallback.
func (c *formatCtx) zone(ch rune, count int, t time.Time) string {
	_, off := t.Zone()
	z := c.ld.Zones

	switch ch {
	case 'z': // specific non-location name (standard/daylight)
		long := count >= 4
		if c.isUTC {
			if v := utcName(z, long); v != "" {
				return v
			}
		}
		if v := c.specificZoneName(t, long); v != "" {
			return v
		}
		return c.gmtOffset(off, z, count < 4)
	case 'v', 'V': // generic non-location name
		long := count >= 4
		if c.isUTC {
			if v := utcName(z, long); v != "" {
				return v
			}
		}
		if v := c.genericZoneName(t, long); v != "" {
			return v
		}
		return c.gmtOffset(off, z, count < 4)
	case 'O': // localized GMT
		return c.gmtOffset(off, z, count < 4)
	case 'Z': // ISO8601 basic; ZZZZ is localized GMT, ZZZZZ extended
		if count == 4 {
			return c.gmtOffset(off, z, false)
		}
		return isoOffset(off, count, ch)
	case 'X', 'x': // ISO8601, with and without "Z" for offset zero
		return isoOffset(off, count, ch)
	}
	return ""
}

func utcName(z map[string]string, long bool) string {
	if long {
		return z["utc.long"]
	}
	return z["utc.short"]
}

func (c *formatCtx) gmtOffset(off int, z map[string]string, short bool) string {
	if off == 0 {
		if v := z["gmtZero"]; v != "" {
			return v
		}
		return "GMT"
	}
	sign := "hourPos"
	a := off
	if off < 0 {
		sign = "hourNeg"
		a = -off
	}
	h := a / 3600
	m := (a % 3600) / 60
	pat := z[sign]
	body := digits.Substitute(formatHourPattern(pat, h, m, short), c.digits)
	gmt := z["gmt"]
	if gmt == "" {
		gmt = "GMT{0}"
	}
	return strings.Replace(gmt, "{0}", body, 1)
}

// formatHourPattern fills a CLDR hourFormat half like "+HH:mm". For the short
// (shortOffset / localized-GMT "O") form, ICU does not zero-pad the hour and
// drops the minutes (and their separator) when the offset has no minute part:
// "GMT-4", "GMT+5:30". The long form always keeps "+HH:mm".
func formatHourPattern(pat string, h, m int, short bool) string {
	runes := []rune(pat)
	// Locate the end of the hour run and the start of the minute field so the
	// short form can drop the minutes together with their separator literal
	// (e.g. the ":" in "+HH:mm") when m == 0.
	hEnd, mStart := -1, -1
	for i := 0; i < len(runes); i++ {
		if runes[i] == 'H' {
			hEnd = i + 1
		}
		if runes[i] == 'm' {
			mStart = i
			break
		}
	}
	emit := func(seg []rune) string {
		var b strings.Builder
		for i := 0; i < len(seg); {
			ch := seg[i]
			if ch == 'H' || ch == 'm' {
				j := i
				for j < len(seg) && seg[j] == ch {
					j++
				}
				cnt := j - i
				val := h
				if ch == 'm' {
					val = m
				}
				if ch == 'H' {
					if short {
						cnt = 1
					} else if cnt < 2 {
						// The long localized-GMT form (longOffset / OOOO) always uses a
						// two-digit hour, regardless of the locale hourFormat's own width
						// (e.g. cs "-H:mm" still renders "GMT-05:00").
						cnt = 2
					}
				}
				b.WriteString(pad(val, cnt))
				i = j
				continue
			}
			b.WriteRune(ch)
			i++
		}
		return b.String()
	}
	if mStart < 0 {
		return emit(runes)
	}
	if short && m == 0 && hEnd >= 0 {
		// Drop the separator + minutes; keep only sign + hour.
		return emit(runes[:hEnd])
	}
	return emit(runes)
}

// normalizeZoneSeparator collapses a comma-bearing separator literal that sits
// immediately adjacent to a zone field (z/Z/O/v/V/X/x run) into a single space.
// CLDR keeps only generic zone availableFormats, and a few locales join them
// with ", " (e.g. cs "H:mm, vvvv"); Intl uses a plain space when the zone is a
// specific or offset name. Only the literal touching the zone run is rewritten,
// so the rest of the pattern is untouched.
func normalizeZoneSeparator(pattern string) string {
	toks := tokenizePattern(pattern)
	zi := -1
	for i, tok := range toks {
		if tok.letter != 0 && fieldClass(tok.letter) == 'z' {
			zi = i
		}
	}
	if zi < 0 {
		return pattern
	}
	for _, i := range []int{zi - 1, zi + 1} {
		if i >= 0 && i < len(toks) && toks[i].letter == 0 &&
			!strings.Contains(toks[i].text, "'") && strings.Contains(toks[i].text, ",") {
			toks[i].text = " "
		}
	}
	return joinTokens(toks)
}

// isoOffset renders the ISO 8601 zone fields per UTS #35: "Z" stands in for
// offset zero at every 'X' width and at ZZZZZ, never for 'x' or Z..ZZZ; the
// extended (colon) format applies at X/x widths 3 and 5 and at ZZZZZ.
func isoOffset(off, count int, ch rune) string {
	if off == 0 && (ch == 'X' || (ch == 'Z' && count == 5)) {
		return "Z"
	}
	sign := "+"
	a := off
	if off < 0 {
		sign = "-"
		a = -off
	}
	hh := pad(a/3600, 2)
	mm := pad((a%3600)/60, 2)
	switch {
	case ch != 'Z' && count == 1 && a%3600 == 0:
		return sign + hh
	case ch == 'Z' && count == 5, ch != 'Z' && (count == 3 || count == 5):
		return sign + hh + ":" + mm
	}
	return sign + hh + mm
}
