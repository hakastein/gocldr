package datetime

import (
	"strconv"
	"strings"
	"time"
)

// formatCtx carries the resolved locale data and options through formatting.
type formatCtx struct {
	ld     *localeData
	locale string
	digits []rune
	opts   Options
	// padNumericHour is set when a "numeric" hour should still be padded to two
	// digits, which ICU does when the requested clock (12/24) is not the
	// locale's preferred one.
	padNumericHour bool
	// zoneNoSecPad mirrors an ICU quirk: a "numeric" hour rendered by a 24-hour
	// pattern (HH) that also carries a zone field but NO seconds field is kept
	// padded to two digits (e.g. de "09:07 UTC"), whereas the same request
	// without the zone — or with seconds — is unpadded ("9:07", "9:07:02 UTC").
	zoneNoSecPad bool
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
}

// num formats an integer with the locale's numbering-system digits, zero-padded
// to at least minWidth.
func (c *formatCtx) num(v, minWidth int) string {
	s := strconv.Itoa(v)
	if len(s) < minWidth {
		s = strings.Repeat("0", minWidth-len(s)) + s
	}
	if len(c.digits) != 10 || (c.digits[0] == '0' && c.digits[9] == '9') {
		return s
	}
	var b strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(c.digits[ch-'0'])
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// interpret runs the CLDR pattern over t, emitting localized text.
func (c *formatCtx) interpret(pattern string, t time.Time) string {
	var b strings.Builder
	runes := []rune(pattern)
	n := len(runes)
	for i := 0; i < n; {
		ch := runes[i]
		if ch == '\'' {
			// Quoted literal. '' is a literal apostrophe.
			if i+1 < n && runes[i+1] == '\'' {
				b.WriteByte('\'')
				i += 2
				continue
			}
			i++
			for i < n && runes[i] != '\'' {
				b.WriteRune(runes[i])
				i++
			}
			i++ // skip closing quote
			continue
		}
		if isPatternLetter(ch) {
			j := i
			for j < n && runes[j] == ch {
				j++
			}
			count := j - i
			b.WriteString(c.field(ch, count, t))
			i = j
			continue
		}
		b.WriteRune(ch)
		i++
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
	case 'y', 'Y', 'u': // year (Y=week-year approximated as year)
		return c.year(count, t)
	case 'M', 'L': // month (format vs stand-alone)
		return c.month(ch, count, t)
	case 'd': // day of month
		return c.num(t.Day(), count)
	case 'D': // day of year
		return c.num(t.YearDay(), count)
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
	case 'w', 'W': // week of year / month (rarely used here)
		return c.num(isoWeek(t), count)
	}
	// Unknown letter: emit as-is.
	return strings.Repeat(string(ch), count)
}

func isoWeek(t time.Time) int {
	_, w := t.ISOWeek()
	return w
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
	if c.opts.DayPeriod != "" {
		switch c.opts.DayPeriod {
		case "narrow":
			width = "narrow"
		case "short":
			width = "abbreviated"
		case "long":
			width = "wide"
		}
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

// flexDayPeriod renders the flexible day-period field 'B'. It selects the
// locale's day-period key (morning1/afternoon1/evening1/night1, plus noon at
// the exact noon instant) whose rule range contains t's wall-clock time, then
// looks that key up in the format day-period names at the width implied by the
// field count (B/BB/BBB -> abbreviated, BBBB -> wide, BBBBB -> narrow). When no
// rule matches or the name is missing it falls back to the am/pm 'b' behaviour.
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
	width := "abbreviated"
	switch {
	case count >= 5:
		width = "narrow"
	case count == 4:
		width = "wide"
	}
	// DayPeriod option width overrides the field-count width.
	switch c.opts.DayPeriod {
	case "narrow":
		width = "narrow"
	case "short":
		width = "abbreviated"
	case "long":
		width = "wide"
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
	// count = number of fractional digits requested.
	ns := t.Nanosecond()
	// Build the fractional string scaled to 9 digits, then take first `count`.
	full := strconv.Itoa(ns)
	full = strings.Repeat("0", 9-len(full)) + full
	if count > 9 {
		full += strings.Repeat("0", count-9)
	}
	frac := full[:count]
	// localize digits
	var b strings.Builder
	for _, ch := range frac {
		if len(c.digits) == 10 && ch >= '0' && ch <= '9' {
			b.WriteRune(c.digits[ch-'0'])
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// zone renders a time zone field. For UTC it uses the CLDR zone names; for
// other zones it falls back to a GMT offset formatted with the CLDR gmtFormat.
func (c *formatCtx) zone(ch rune, count int, t time.Time) string {
	name, off := t.Zone()
	z := c.ld.Zones

	isUTC := off == 0 && (name == "UTC" || name == "" || strings.EqualFold(c.opts.TimeZone, "UTC") || strings.EqualFold(c.opts.TimeZone, "Etc/UTC"))

	switch ch {
	case 'z': // specific non-location name (standard/daylight)
		long := count >= 4
		if isUTC {
			if long {
				if v := z["utc.long"]; v != "" {
					return v
				}
			} else {
				if v := z["utc.short"]; v != "" {
					return v
				}
			}
		}
		if v := c.specificZoneName(t, long); v != "" {
			return v
		}
		return c.gmtOffset(off, z, count < 4)
	case 'v', 'V': // generic non-location name
		long := count >= 4
		if isUTC {
			if long {
				if v := z["utc.long"]; v != "" {
					return v
				}
			} else {
				if v := z["utc.short"]; v != "" {
					return v
				}
			}
		}
		if v := c.genericZoneName(t, long); v != "" {
			return v
		}
		return c.gmtOffset(off, z, count < 4)
	case 'O': // localized GMT
		return c.gmtOffset(off, z, count < 4)
	case 'Z': // ISO8601 basic / extended
		return isoOffset(off, count)
	case 'X', 'x':
		return isoOffset(off, count)
	}
	return ""
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
	body := c.localizeDigits(formatHourPattern(pat, h, m, short))
	gmt := z["gmt"]
	if gmt == "" {
		gmt = "GMT{0}"
	}
	return strings.Replace(gmt, "{0}", body, 1)
}

// localizeDigits maps ASCII digits in s to the locale's numbering-system
// glyphs (e.g. fa's "−4" -> "−۴"). Non-decimal numbering systems are left
// untouched.
func (c *formatCtx) localizeDigits(s string) string {
	if len(c.digits) != 10 || (c.digits[0] == '0' && c.digits[9] == '9') {
		return s
	}
	var b strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(c.digits[ch-'0'])
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
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
				s := strconv.Itoa(val)
				for len(s) < cnt {
					s = "0" + s
				}
				b.WriteString(s)
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
	runes := []rune(pattern)
	n := len(runes)
	// Find the zone run.
	zStart, zEnd := -1, -1
	inQuote := false
	for i := 0; i < n; i++ {
		ch := runes[i]
		if ch == '\'' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		if fieldClass(ch) == 'z' {
			if zStart < 0 {
				zStart = i
			}
			zEnd = i
		}
	}
	if zStart < 0 {
		return pattern
	}
	// Separator before the zone: literal run ending at zStart.
	if zStart > 0 {
		j := zStart - 1
		for j >= 0 && !isPatternLetter(runes[j]) && runes[j] != '\'' {
			j--
		}
		sep := string(runes[j+1 : zStart])
		if strings.Contains(sep, ",") {
			return string(runes[:j+1]) + " " + string(runes[zStart:])
		}
	}
	// Separator after the zone: literal run starting at zEnd+1.
	if zEnd+1 < n {
		j := zEnd + 1
		for j < n && !isPatternLetter(runes[j]) && runes[j] != '\'' {
			j++
		}
		sep := string(runes[zEnd+1 : j])
		if strings.Contains(sep, ",") {
			return string(runes[:zEnd+1]) + " " + string(runes[j:])
		}
	}
	return pattern
}

func isoOffset(off, count int) string {
	if off == 0 && count < 4 {
		return "Z"
	}
	sign := "+"
	a := off
	if off < 0 {
		sign = "-"
		a = -off
	}
	h := a / 3600
	m := (a % 3600) / 60
	hh := strconv.Itoa(h)
	if len(hh) < 2 {
		hh = "0" + hh
	}
	mm := strconv.Itoa(m)
	if len(mm) < 2 {
		mm = "0" + mm
	}
	if count >= 3 {
		return sign + hh + ":" + mm
	}
	return sign + hh + mm
}
