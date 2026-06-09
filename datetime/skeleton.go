package datetime

import (
	"sort"
	"strings"
)

// buildSkeleton converts component options into a CLDR skeleton string in the
// canonical field order used by availableFormats keys: era, year, quarter,
// month, week, day, weekday, dayPeriod, hour, minute, second, fractionalSecond,
// timeZone. This mirrors how Intl maps options to a skeleton before best-fit.
func (c *formatCtx) buildSkeleton() string {
	o := c.opts
	var b strings.Builder

	if o.Era != "" {
		b.WriteString(eraSkel(o.Era))
	}
	switch o.Year {
	case "2-digit":
		b.WriteString("yy")
	case "numeric":
		b.WriteString("y")
	}
	switch o.Month {
	case "2-digit":
		b.WriteString("MM")
	case "numeric":
		b.WriteString("M")
	case "long":
		b.WriteString("MMMM")
	case "short":
		b.WriteString("MMM")
	case "narrow":
		b.WriteString("MMMMM")
	}
	switch o.Day {
	case "2-digit":
		b.WriteString("dd")
	case "numeric":
		b.WriteString("d")
	}
	switch o.Weekday {
	case "long":
		b.WriteString("EEEE")
	case "short":
		b.WriteString("EEE")
	case "narrow":
		b.WriteString("EEEEE")
	}
	// dayPeriod (flexible "B" field). It is only meaningful with a 12-hour
	// clock: Intl honors it when the resolved hour cycle is 12-hour, and drops
	// it for 24-hour locales (which keep their plain hour pattern). With no hour
	// requested the period is always rendered, so we emit B in that case too and
	// let the matcher / a synthesized bare-B pattern handle it.
	if o.DayPeriod != "" {
		if o.Hour == "" || c.hourLetter() == "h" {
			b.WriteString(dayPeriodSkel(o.DayPeriod))
		}
	}
	// hour: pick h vs H based on hour cycle.
	if o.Hour != "" {
		hl := c.hourLetter()
		if o.Hour == "2-digit" {
			b.WriteString(strings.Repeat(hl, 2))
		} else {
			b.WriteString(hl)
		}
	}
	switch o.Minute {
	case "2-digit":
		b.WriteString("mm")
	case "numeric":
		b.WriteString("m")
	}
	switch o.Second {
	case "2-digit":
		b.WriteString("ss")
	case "numeric":
		b.WriteString("s")
	}
	if o.FractionalSecondDigits != nil {
		b.WriteString(strings.Repeat("S", *o.FractionalSecondDigits))
	}
	switch o.TimeZoneName {
	case "long":
		b.WriteString("zzzz")
	case "short":
		b.WriteString("z")
	case "shortOffset":
		b.WriteString("O")
	case "longOffset":
		b.WriteString("OOOO")
	case "shortGeneric":
		b.WriteString("v")
	case "longGeneric":
		b.WriteString("vvvv")
	}
	return b.String()
}

// dayPeriodSkel maps the dayPeriod option width to a B-run length: short ->
// "B" (abbreviated), long -> "BBBB" (wide), narrow -> "BBBBB".
func dayPeriodSkel(v string) string {
	switch v {
	case "long":
		return "BBBB"
	case "narrow":
		return "BBBBB"
	default: // "short"
		return "B"
	}
}

func eraSkel(v string) string {
	switch v {
	case "long":
		return "GGGG"
	case "narrow":
		return "GGGGG"
	default:
		return "G"
	}
}

// hourLetter returns "h" or "H" depending on hour12 / locale default.
func (c *formatCtx) hourLetter() string {
	if c.opts.Hour12 != nil {
		if *c.opts.Hour12 {
			return "h"
		}
		return "H"
	}
	if c.localeUses12() {
		return "h"
	}
	return "H"
}

// localeUses12 inspects the locale's short time pattern to decide whether the
// locale defaults to a 12-hour clock.
func (c *formatCtx) localeUses12() bool {
	p := c.ld.TimeFormats["short"]
	for _, ch := range p {
		switch ch {
		case 'h', 'K':
			return true
		case 'H', 'k':
			return false
		}
	}
	return false
}

// applyHourCycle rewrites the hour field of a pattern to honor opts.Hour12 (or
// the locale default), keeping the rest intact. It also drops or keeps the
// day-period field accordingly.
func (c *formatCtx) applyHourCycle(pattern string) string {
	if pattern == "" {
		return pattern
	}
	want12 := c.localeUses12()
	if c.opts.Hour12 != nil {
		want12 = *c.opts.Hour12
	}

	// Determine current cycle of the pattern.
	cur12 := false
	hasHour := false
	nonCanonical := false // pattern uses K (h11) or k (h24)
	for _, ch := range stripQuotes(pattern) {
		switch ch {
		case 'h':
			cur12 = true
			hasHour = true
		case 'K':
			cur12 = true
			hasHour = true
			nonCanonical = true
		case 'H':
			hasHour = true
		case 'k':
			hasHour = true
			nonCanonical = true
		}
	}
	// We always normalize to the canonical h12 ('h') / h23 ('H') cycle that
	// Intl uses for hour12 true/false, so a pattern using K/k must be rewritten
	// even when its 12/24-ness already matches the request.
	if !hasHour || (cur12 == want12 && !nonCanonical) {
		return pattern
	}

	// Rewrite hour letters and adjust the day-period token.
	var b strings.Builder
	runes := []rune(pattern)
	n := len(runes)
	inQuote := false
	for i := 0; i < n; {
		ch := runes[i]
		if ch == '\'' {
			inQuote = !inQuote
			b.WriteRune(ch)
			i++
			continue
		}
		if inQuote {
			b.WriteRune(ch)
			i++
			continue
		}
		if ch == 'h' || ch == 'H' || ch == 'K' || ch == 'k' {
			j := i
			for j < n && runes[j] == ch {
				j++
			}
			cnt := j - i
			var nl rune
			if want12 {
				nl = 'h'
			} else {
				nl = 'H'
			}
			// Pad to two digits when forcing the locale's non-preferred clock
			// (padNumericHour); ICU keeps the hour padded in that case even though
			// the source pattern used a single (unpadded) hour letter.
			if c.padNumericHour && cnt < 2 {
				cnt = 2
			}
			for k := 0; k < cnt; k++ {
				b.WriteRune(nl)
			}
			i = j
			continue
		}
		b.WriteRune(ch)
		i++
	}
	out := b.String()
	if want12 {
		out = c.ensureDayPeriod(out)
	} else {
		out = removeDayPeriod(out)
	}
	return out
}

// ensureDayPeriod adds an "a" field next to the hour if the 12-hour pattern is
// missing one (e.g. converting H -> h). The period's position and separator are
// taken from the locale's native 12-hour available format (hms/hm/h), so
// languages that place the period BEFORE the hour (e.g. zh "ah:mm:ss",
// ja "aK:mm:ss") are rendered correctly instead of textually splicing " a"
// after the hour.
func (c *formatCtx) ensureDayPeriod(pattern string) string {
	if strings.ContainsAny(stripQuotes(pattern), "aAbB") {
		return pattern
	}
	runes := []rune(pattern)
	// Locate the (single, converted) hour run.
	start, end := -1, -1
	inQuote := false
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\'' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && runes[i] == 'h' {
			if start < 0 {
				start = i
			}
			end = i
		}
	}
	if start < 0 {
		return pattern
	}
	prefix, sep := c.periodAffix()
	if prefix {
		return string(runes[:start]) + "a" + sep + string(runes[start:])
	}
	return string(runes[:end+1]) + sep + "a" + string(runes[end+1:])
}

// periodAffix inspects the locale's native 12-hour available format to learn
// whether the day period precedes (prefix) or follows the hour, and which
// literal separates them. It falls back to a trailing " a" (the English
// convention) when no usable 12-hour format is available.
func (c *formatCtx) periodAffix() (prefix bool, sep string) {
	for _, skel := range []string{"hms", "hm", "h"} {
		pat := c.ld.Available[skel]
		if pat == "" {
			continue
		}
		if p, s, ok := scanPeriodAffix(pat); ok {
			return p, s
		}
	}
	return false, " "
}

// scanPeriodAffix parses a native 12-hour pattern and returns the day period's
// placement relative to the hour field plus the literal text between them.
func scanPeriodAffix(pat string) (prefix bool, sep string, ok bool) {
	runes := []rune(pat)
	n := len(runes)
	hStart, hEnd := -1, -1
	pStart, pEnd := -1, -1
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
		switch ch {
		case 'h', 'H', 'K', 'k':
			if hStart < 0 {
				hStart = i
			}
			hEnd = i
		case 'a', 'b', 'B':
			if pStart < 0 {
				pStart = i
			}
			pEnd = i
		}
	}
	if hStart < 0 || pStart < 0 {
		return false, "", false
	}
	if pEnd < hStart {
		// period before hour: separator is everything between them.
		return true, string(runes[pEnd+1 : hStart]), true
	}
	if pStart > hEnd {
		// period after hour.
		return false, string(runes[hEnd+1 : pStart]), true
	}
	return false, "", false
}

// removeDayPeriod strips the day-period field (a/b/B runs and adjacent spaces)
// from a pattern when converting to a 24-hour clock.
func removeDayPeriod(pattern string) string {
	var b strings.Builder
	runes := []rune(pattern)
	n := len(runes)
	inQuote := false
	for i := 0; i < n; {
		ch := runes[i]
		if ch == '\'' {
			inQuote = !inQuote
			b.WriteRune(ch)
			i++
			continue
		}
		if !inQuote && (ch == 'a' || ch == 'b' || ch == 'B') {
			j := i
			for j < n && runes[j] == ch {
				j++
			}
			// also consume one adjacent space (before or after)
			out := b.String()
			if len(out) > 0 && (out[len(out)-1] == ' ') {
				trimmed := []rune(out)
				b.Reset()
				b.WriteString(string(trimmed[:len(trimmed)-1]))
			} else if j < n && runes[j] == ' ' {
				j++
			}
			i = j
			continue
		}
		b.WriteRune(ch)
		i++
	}
	return strings.TrimSpace(b.String())
}

func stripQuotes(pattern string) string {
	var b strings.Builder
	inQuote := false
	for _, ch := range pattern {
		if ch == '\'' {
			inQuote = !inQuote
			continue
		}
		if !inQuote {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// ---- best-fit skeleton matching ----

// skelField captures one field in a skeleton: its canonical letter and length.
type skelField struct {
	letter rune
	count  int
}

// parseSkeleton breaks a skeleton into canonical fields keyed by field class.
func parseSkeleton(skel string) map[rune]skelField {
	out := map[rune]skelField{}
	runes := []rune(skel)
	for i := 0; i < len(runes); {
		ch := runes[i]
		j := i
		for j < len(runes) && runes[j] == ch {
			j++
		}
		cls := fieldClass(ch)
		if cls != 0 {
			// Keep the larger count if duplicated.
			f := out[cls]
			cnt := j - i
			if cnt > f.count {
				out[cls] = skelField{letter: ch, count: cnt}
			}
		}
		i = j
	}
	return out
}

// fieldClass maps a pattern letter to a canonical field class letter so that,
// e.g., L and M (month) or E/e/c (weekday) compare as the same field.
func fieldClass(ch rune) rune {
	switch ch {
	case 'G':
		return 'G'
	case 'y', 'Y', 'u', 'r':
		return 'y'
	case 'Q', 'q':
		return 'Q'
	case 'M', 'L':
		return 'M'
	case 'w', 'W':
		return 'w'
	case 'd', 'D':
		return 'd'
	case 'E', 'e', 'c':
		return 'E'
	case 'a', 'b', 'B':
		return 'a'
	case 'h', 'H', 'k', 'K':
		return 'h'
	case 'm':
		return 'm'
	case 's':
		return 's'
	case 'S':
		return 'S'
	case 'z', 'Z', 'O', 'v', 'V', 'X', 'x':
		return 'z'
	}
	return 0
}

// bestMatch finds the closest availableFormats pattern for the requested
// skeleton and adjusts field widths to the request. This follows ICU's
// DateTimePatternGenerator best-fit approach approximately.
func (c *formatCtx) bestMatch(skel string) string {
	avail := c.ld.Available
	// 1. exact hit
	if p, ok := avail[skel]; ok {
		return p
	}

	want := parseSkeleton(skel)

	// dayPeriod-only request (just the flexible "B" field, no hour): Intl renders
	// the bare day period. CLDR has no bare-B availableFormat, so synthesize one
	// at the requested width.
	if len(want) == 1 {
		if f, ok := want['a']; ok && f.letter == 'B' {
			return strings.Repeat("B", f.count)
		}
	}

	// If the request mixes date and time fields, ICU splits the skeleton into a
	// date sub-skeleton and a time sub-skeleton, best-matches each, then joins
	// them with the appropriate dateTimeFormats connector. A single
	// availableFormats entry rarely covers both, so do this first.
	if hasDateFields(want) && hasTimeFields(want) {
		return c.synthesize(want)
	}

	// 2. score every available skeleton; lower is better.
	type cand struct {
		key   string
		pat   string
		score int
	}
	var best *cand
	keys := make([]string, 0, len(avail))
	for k := range avail {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		have := parseSkeleton(k)
		score, ok := matchScore(want, have)
		if !ok {
			continue
		}
		if best == nil || score < best.score {
			cp := cand{key: k, pat: avail[k], score: score}
			best = &cp
		}
	}

	if best != nil {
		return c.adjustWidths(best.pat, want)
	}

	// 3. Fallback: synthesize from date + time available formats and combine.
	return c.synthesize(want)
}

// matchScore returns a distance between requested and candidate field sets.
// A candidate is only eligible if it does not contain fields absent from the
// request beyond an allowed tolerance, mirroring ICU's penalty model.
func matchScore(want, have map[rune]skelField) (int, bool) {
	score := 0
	// Penalize missing requested fields heavily.
	for cls, wf := range want {
		hf, ok := have[cls]
		if !ok {
			score += 1000
			continue
		}
		score += widthDistance(wf, hf)
		// Prefer the candidate whose hour cycle (12 vs 24) matches the request,
		// so e.g. skeleton "hmm" picks "h:mm a" over "HH:mm".
		if cls == 'h' && hourIs12(wf.letter) != hourIs12(hf.letter) {
			score += 5
		}
		// Component options always use the format context (letters E and M), so
		// prefer candidates that also use the format letter over the stand-alone
		// variant (c, L). ICU treats the format pattern as canonical, so this
		// outweighs a small width difference (e.g. format "E" beats stand-alone
		// "cccc" even though the latter's width matches the request exactly).
		if isStandalone(hf.letter) != isStandalone(wf.letter) {
			score += 20
		}
	}
	// Penalize extra fields in candidate not requested.
	for cls := range have {
		if _, ok := want[cls]; !ok {
			score += 1000
		}
	}
	return score, true
}

func hourIs12(letter rune) bool {
	return letter == 'h' || letter == 'K'
}

// isStandalone reports whether a letter is a CLDR stand-alone field variant
// (L for month, c for weekday, q for quarter) as opposed to the format variant.
func isStandalone(letter rune) bool {
	return letter == 'L' || letter == 'c' || letter == 'q'
}

// fieldNumeric reports whether a (letter,count) field renders as a number
// rather than a name. For month/quarter, count<=2 is numeric; for weekday only
// the e/c variants are numeric (and only at count<=2); E is always a name.
func fieldNumeric(letter rune, count int) bool {
	switch letter {
	case 'M', 'L', 'Q', 'q':
		return count <= 2
	case 'e', 'c':
		return count <= 2
	case 'E':
		return false
	default:
		return count <= 2
	}
}

// widthDistance scores the difference in field length, with extra penalty when
// crossing the numeric<->alpha boundary (e.g. M vs MMM).
func widthDistance(want, have skelField) int {
	d := want.count - have.count
	if d < 0 {
		d = -d
	}
	score := d
	wantNumeric := want.count <= 2
	haveNumeric := have.count <= 2
	if wantNumeric != haveNumeric {
		score += 10
	}
	return score
}

// adjustWidths rewrites the chosen pattern's field lengths to match the request
// where the field classes line up (e.g. widen M->MMMM if the request asked for
// a long month). Non-matching fields are left untouched.
func (c *formatCtx) adjustWidths(pattern string, want map[rune]skelField) string {
	var b strings.Builder
	runes := []rune(pattern)
	n := len(runes)
	inQuote := false
	for i := 0; i < n; {
		ch := runes[i]
		if ch == '\'' {
			inQuote = !inQuote
			b.WriteRune(ch)
			i++
			continue
		}
		if inQuote || !isPatternLetter(ch) {
			b.WriteRune(ch)
			i++
			continue
		}
		j := i
		for j < n && runes[j] == ch {
			j++
		}
		cnt := j - i
		cls := fieldClass(ch)
		if wf, ok := want[cls]; ok {
			// Match the requested length, but keep the candidate's own letter
			// variant (e.g. keep 'L' vs 'M', 'h' vs 'H').
			outCh := ch
			newCnt := wf.count
			// Zone: the candidate may carry a different zone letter (e.g. the time
			// pattern's generic "v") than the one requested. The zone letter
			// encodes the format kind (z name, O localized GMT offset, ...), so
			// honor the REQUESTED letter and width rather than the candidate's;
			// otherwise shortOffset/longOffset both collapse to the candidate's
			// rendering. Name-based widths for non-UTC zones remain data-blocked.
			if cls == 'z' {
				outCh = wf.letter
			}
			// Do not promote a numeric pattern field to an alpha (name) one or
			// vice versa: ICU's adjustFieldTypes never crosses the
			// numeric<->text boundary. E.g. ja's yMMMd pattern uses a numeric
			// "M" followed by a literal 月; a long-month request must not widen
			// it to MMMM. Whether a field is numeric depends on its letter, not
			// only the count (e.g. single "E" is the abbreviated weekday name,
			// while single "M" is a numeric month).
			if (cls == 'M' || cls == 'Q') && fieldNumeric(ch, cnt) != fieldNumeric(wf.letter, wf.count) {
				newCnt = cnt
			}
			// Numeric year: keep candidate count unless request is 2-digit.
			if cls == 'y' {
				if wf.count == 2 {
					newCnt = 2
				} else {
					newCnt = cnt
				}
			}
			// Hour: "2-digit" pads to HH, "numeric" uses a single (unpadded)
			// hour, EXCEPT when the request forces the locale's non-preferred
			// clock, in which case ICU keeps the padded width (e.g. en forced to
			// a 24-hour clock renders "09:07").
			if cls == 'h' {
				switch {
				case wf.count >= 2:
					newCnt = 2
				case c.padNumericHour:
					newCnt = 2
				case c.zoneNoSecPad && (ch == 'H' || ch == 'k'):
					// With a zone field and no seconds, ICU keeps the matched 24-hour
					// pattern's own hour width rather than unpadding a numeric hour:
					// de's "HH:mm v" stays "09:07 UTC" while ja's "H:mm v" stays
					// "9:07 GMT...". Preserve the candidate's count.
					newCnt = cnt
				default:
					newCnt = 1
				}
			}
			// Minute / second: keep the candidate width unless 2-digit is
			// requested. These fields are conventionally rendered padded inside a
			// time pattern, so we preserve the matched pattern's width for
			// "numeric" rather than forcing a single digit.
			if cls == 'm' || cls == 's' {
				if wf.count >= 2 {
					newCnt = 2
				} else {
					newCnt = cnt
				}
			}
			for k := 0; k < newCnt; k++ {
				b.WriteRune(outCh)
			}
		} else {
			for k := 0; k < cnt; k++ {
				b.WriteRune(ch)
			}
		}
		i = j
	}
	return b.String()
}

var dateClasses = []rune{'G', 'y', 'Q', 'M', 'w', 'd', 'E'}
var timeClasses = []rune{'a', 'h', 'm', 's', 'S', 'z'}

func hasDateFields(want map[rune]skelField) bool {
	for _, cls := range dateClasses {
		if _, ok := want[cls]; ok {
			return true
		}
	}
	return false
}

func hasTimeFields(want map[rune]skelField) bool {
	for _, cls := range timeClasses {
		if _, ok := want[cls]; ok {
			return true
		}
	}
	return false
}

// synthesize builds a pattern by separately best-matching the date portion and
// the time portion of the request and combining them with the dateTimeFormats
// connector whose length matches the date portion's style (ICU rule: a long
// month or weekday selects the "at"/long connector, a short month the medium
// connector, numeric-only the short connector).
func (c *formatCtx) synthesize(want map[rune]skelField) string {
	var dateSkel, timeSkel strings.Builder
	for _, cls := range dateClasses {
		if f, ok := want[cls]; ok {
			dateSkel.WriteString(strings.Repeat(string(f.letter), f.count))
		}
	}
	for _, cls := range timeClasses {
		if f, ok := want[cls]; ok {
			timeSkel.WriteString(strings.Repeat(string(f.letter), f.count))
		}
	}
	var datePat, timePat string
	if dateSkel.Len() > 0 {
		datePat = c.matchPortion(dateSkel.String(), want)
	}
	if timeSkel.Len() > 0 {
		timePat = c.matchPortion(timeSkel.String(), want)
		timePat = c.applyHourCycle(timePat)
	}
	switch {
	case datePat != "" && timePat != "":
		style := connectorStyle(want)
		conn := c.ld.AtTime[style]
		if conn == "" {
			conn = c.ld.DateTime[style]
		}
		return combine(conn, datePat, timePat)
	case datePat != "":
		return datePat
	case timePat != "":
		return timePat
	}
	return c.ld.DateFormats["medium"]
}

// connectorStyle picks full/long/medium/short for the dateTimeFormats connector
// based on the date portion of the request, mirroring ICU's behaviour.
func connectorStyle(want map[rune]skelField) string {
	if m, ok := want['M']; ok && m.count >= 4 {
		return "long" // long/full share the "at" connector in CLDR
	}
	if e, ok := want['E']; ok && e.count >= 4 {
		if _, hasM := want['M']; !hasM {
			return "long"
		}
	}
	if m, ok := want['M']; ok && m.count == 3 {
		return "medium"
	}
	return "short"
}

func (c *formatCtx) matchPortion(skel string, want map[rune]skelField) string {
	avail := c.ld.Available
	if p, ok := avail[skel]; ok {
		return c.adjustWidths(p, want)
	}
	wantP := parseSkeleton(skel)
	var best string
	bestScore := 1 << 30
	keys := make([]string, 0, len(avail))
	for k := range avail {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		have := parseSkeleton(k)
		sc, ok := matchScore(wantP, have)
		if !ok {
			continue
		}
		if sc < bestScore {
			bestScore = sc
			best = avail[k]
		}
	}
	if best != "" {
		return c.adjustWidths(best, want)
	}
	return ""
}
