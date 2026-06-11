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
	if o.DayPeriod != "" && (o.Hour == "" || c.hourLetter() == "h") {
		b.WriteString(dayPeriodSkel(o.DayPeriod))
	}
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
	// No availableFormats entry carries the fractional-second S field, so it
	// must stay out of the matching skeleton (resolvePattern injects it after
	// matching); a fraction without seconds still needs an s field to anchor on.
	switch o.Second {
	case "2-digit":
		b.WriteString("ss")
	case "numeric":
		b.WriteString("s")
	default:
		if o.FractionalSecondDigits != nil && (o.Hour != "" || o.Minute != "") {
			b.WriteString("s")
		}
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

// dayPeriodSkel maps the dayPeriod option width to a B-run length, mirroring
// dayPeriodOptionWidth's widths.
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
	for _, ch := range c.ld.TimeFormats["short"] {
		if isHourLetter(ch) {
			return hourIs12(ch)
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

	toks := tokenizePattern(pattern)
	// Determine current cycle of the pattern.
	cur12 := false
	hasHour := false
	nonCanonical := false // pattern uses K (h11) or k (h24)
	for _, tok := range toks {
		if isHourLetter(tok.letter) {
			hasHour = true
			cur12 = cur12 || hourIs12(tok.letter)
			nonCanonical = nonCanonical || tok.letter == 'K' || tok.letter == 'k'
		}
	}
	// We always normalize to the canonical h12 ('h') / h23 ('H') cycle that
	// Intl uses for hour12 true/false, so a pattern using K/k must be rewritten
	// even when its 12/24-ness already matches the request. A pattern already
	// on the requested cycle may still need padding: ICU keeps a two-digit hour
	// when the caller forces the locale's non-preferred clock, even though the
	// source pattern used a single (unpadded) hour letter.
	rewrite := cur12 != want12 || nonCanonical
	forcePad := c.forcePad24()
	if !hasHour || (!rewrite && !forcePad) {
		return pattern
	}

	// Rewrite hour letters and adjust the day-period token.
	nl := "H"
	if want12 {
		nl = "h"
	}
	var b strings.Builder
	for _, tok := range toks {
		if !isHourLetter(tok.letter) {
			b.WriteString(tok.text)
			continue
		}
		cnt := tok.count
		if forcePad && cnt < 2 {
			cnt = 2
		}
		b.WriteString(strings.Repeat(nl, cnt))
	}
	out := b.String()
	if !rewrite {
		return out
	}
	if want12 {
		return c.ensureDayPeriod(out)
	}
	return removeDayPeriod(out)
}

// ensureDayPeriod adds an "a" field next to the hour if the 12-hour pattern is
// missing one (e.g. converting H -> h). The period's position and separator are
// taken from the locale's native 12-hour available format (hms/hm/h), so
// languages that place the period BEFORE the hour (e.g. zh "ah:mm:ss",
// ja "aK:mm:ss") are rendered correctly instead of textually splicing " a"
// after the hour.
func (c *formatCtx) ensureDayPeriod(pattern string) string {
	toks := tokenizePattern(pattern)
	// Locate the (single, converted) hour run.
	first, last := -1, -1
	for i, tok := range toks {
		switch tok.letter {
		case 'a', 'A', 'b', 'B':
			return pattern
		case 'h':
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return pattern
	}
	prefix, sep := c.periodAffix()
	var b strings.Builder
	for i, tok := range toks {
		if prefix && i == first {
			b.WriteString("a")
			b.WriteString(sep)
		}
		b.WriteString(tok.text)
		if !prefix && i == last {
			b.WriteString(sep)
			b.WriteString("a")
		}
	}
	return b.String()
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
	toks := tokenizePattern(pat)
	hFirst, hLast := -1, -1
	pFirst, pLast := -1, -1
	for i, tok := range toks {
		switch {
		case isHourLetter(tok.letter):
			if hFirst < 0 {
				hFirst = i
			}
			hLast = i
		case tok.letter == 'a' || tok.letter == 'b' || tok.letter == 'B':
			if pFirst < 0 {
				pFirst = i
			}
			pLast = i
		}
	}
	if hFirst < 0 || pFirst < 0 {
		return false, "", false
	}
	if pLast < hFirst {
		// period before hour: separator is everything between them.
		return true, joinTokens(toks[pLast+1 : hFirst]), true
	}
	if pFirst > hLast {
		// period after hour.
		return false, joinTokens(toks[hLast+1 : pFirst]), true
	}
	return false, "", false
}

// removeDayPeriod strips the day-period field (a/b/B runs and adjacent spaces)
// from a pattern when converting to a 24-hour clock.
func removeDayPeriod(pattern string) string {
	var b strings.Builder
	skipSpace := false
	for _, tok := range tokenizePattern(pattern) {
		if tok.letter == 'a' || tok.letter == 'b' || tok.letter == 'B' {
			// also consume one adjacent space (before or after)
			if out := b.String(); strings.HasSuffix(out, " ") {
				b.Reset()
				b.WriteString(strings.TrimSuffix(out, " "))
			} else {
				skipSpace = true
			}
			continue
		}
		text := tok.text
		if skipSpace {
			text = strings.TrimPrefix(text, " ")
			skipSpace = false
		}
		b.WriteString(text)
	}
	return strings.TrimSpace(b.String())
}

// ---- best-fit skeleton matching ----

// skelField captures one field in a skeleton: its canonical letter and length.
type skelField struct {
	letter rune
	count  int
}

// parseSkeleton breaks a skeleton into canonical fields keyed by field class,
// keeping the larger count if a class is duplicated.
func parseSkeleton(skel string) map[rune]skelField {
	out := map[rune]skelField{}
	for _, tok := range tokenizePattern(skel) {
		if cls := fieldClass(tok.letter); cls != 0 && tok.count > out[cls].count {
			out[cls] = skelField{letter: tok.letter, count: tok.count}
		}
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

// patternSkeleton derives the skeleton of a concrete pattern: unquoted field
// runs collapsed per field class (keeping the widest run), emitted in the
// canonical class order. This is how ICU's DateTimePatternGenerator computes
// the skeletons under which it files the standard date/time patterns.
func patternSkeleton(pattern string) string {
	fields := map[rune]skelField{}
	for _, tok := range tokenizePattern(pattern) {
		// AM/PM (a, and the b variant) is an implicit companion of the 12-hour
		// 'h' field: ICU drops it when computing pattern skeletons, so a
		// request for "hms" can match "a h시 m분 s초 zzzz" without the day
		// period counting as an extra field. The flexible day period B stays
		// significant.
		if tok.letter == 'a' || tok.letter == 'b' {
			continue
		}
		if cls := fieldClass(tok.letter); cls != 0 && tok.count > fields[cls].count {
			fields[cls] = skelField{letter: tok.letter, count: tok.count}
		}
	}
	var b strings.Builder
	for _, cls := range append(dateClasses, timeClasses...) {
		if f, ok := fields[cls]; ok {
			b.WriteString(strings.Repeat(string(f.letter), f.count))
		}
	}
	return b.String()
}

// skeletonPool returns the candidate pool for best-fit matching: the locale's
// availableFormats plus — mirroring ICU's DateTimePatternGenerator, which
// seeds its pool with the standard patterns — the four standard date and four
// standard time patterns keyed by their derived skeletons. That seeding is
// what lets e.g. a ja Hms+zone request find the full time pattern
// "H時mm分ss秒 zzzz" (availableFormats only carry a plain "H:mm:ss").
// An availableFormats entry wins over a standard pattern with the same
// skeleton. The pool is cached on the context.
func (c *formatCtx) skeletonPool() map[string]string {
	if c.pool != nil {
		return c.pool
	}
	pool := make(map[string]string, len(c.ld.Available)+8)
	for k, v := range c.ld.Available {
		pool[k] = v
	}
	// Fixed style order keeps the pool deterministic if two standard patterns
	// ever derive the same skeleton (first one wins).
	for _, styles := range []map[string]string{c.ld.DateFormats, c.ld.TimeFormats} {
		for _, style := range []string{"full", "long", "medium", "short"} {
			pat := styles[style]
			if pat == "" {
				continue
			}
			skel := patternSkeleton(pat)
			if skel == "" {
				continue
			}
			if _, ok := pool[skel]; !ok {
				pool[skel] = pat
			}
		}
	}
	c.pool = pool
	return pool
}

// bestMatch finds the closest candidate pattern for the requested skeleton and
// adjusts field widths to the request. This follows ICU's
// DateTimePatternGenerator best-fit approach approximately.
func (c *formatCtx) bestMatch(skel string) string {
	avail := c.skeletonPool()
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
	if p := c.matchPortion(skel, want); p != "" {
		return p
	}

	// 3. Fallback: synthesize from date + time available formats and combine.
	return c.synthesize(want)
}

// matchScore returns a distance between requested and candidate field sets,
// mirroring ICU's penalty model: missing or extra fields cost far more than
// width differences, so a structurally closer candidate always wins.
func matchScore(want, have map[rune]skelField) int {
	score := 0
	// Penalize missing requested fields heavily.
	for cls, wf := range want {
		hf, ok := have[cls]
		if !ok {
			score += 1000
			continue
		}
		score += widthDistance(cls, wf, hf)
		// Prefer the candidate whose hour cycle (12 vs 24) matches the request,
		// so e.g. skeleton "hmm" picks "h:mm a" over "HH:mm".
		if cls == 'h' && hourIs12(wf.letter) != hourIs12(hf.letter) {
			score += 5
		}
		// Zone letters encode the format kind (z specific name, v generic name,
		// O/X/Z offsets). Prefer the candidate whose kind matches the request:
		// the letter is rewritten to the requested one later anyway, but the
		// surrounding pattern should come from the closest kind (e.g. a zzzz
		// request must pick ko's full time pattern "a h시 m분 s초 zzzz" over the
		// generic-v "a h:mm:ss v" availableFormat).
		if cls == 'z' && zoneKind(wf.letter) != zoneKind(hf.letter) {
			score += 20
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
	return score
}

func hourIs12(letter rune) bool {
	return letter == 'h' || letter == 'K'
}

func isHourLetter(ch rune) bool {
	return ch == 'h' || ch == 'H' || ch == 'K' || ch == 'k'
}

// zoneKind groups the zone pattern letters by the format they produce:
// specific names (z), generic/location names (v, V) and numeric offsets
// (O, Z, X, x).
func zoneKind(letter rune) rune {
	switch letter {
	case 'z':
		return 'z'
	case 'v', 'V':
		return 'v'
	default:
		return 'O'
	}
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
// crossing the numeric<->name boundary (e.g. M vs MMM). The boundary only
// exists for fields that have both numeric and name forms — month/quarter
// (M/L/Q/q numeric at count<=2) and weekday (e/c numeric, E a name); hour,
// minute and second are always numeric, and zone/era widths merely select
// among name forms, so for them a width difference is just that.
func widthDistance(cls rune, want, have skelField) int {
	d := want.count - have.count
	if d < 0 {
		d = -d
	}
	switch cls {
	case 'M', 'Q', 'E':
		if fieldNumeric(want.letter, want.count) != fieldNumeric(have.letter, have.count) {
			d += 10
		}
	}
	return d
}

// adjustWidths rewrites the chosen pattern's field lengths to match the request
// where the field classes line up (e.g. widen M->MMMM if the request asked for
// a long month). Non-matching fields are left untouched.
func (c *formatCtx) adjustWidths(pattern string, want map[rune]skelField) string {
	var b strings.Builder
	for _, tok := range tokenizePattern(pattern) {
		cls := fieldClass(tok.letter)
		wf, ok := want[cls]
		if tok.letter == 0 || !ok {
			b.WriteString(tok.text)
			continue
		}
		ch, cnt := tok.letter, tok.count
		// Match the requested length, but keep the candidate's own letter
		// variant (e.g. keep 'L' vs 'M', 'h' vs 'H').
		outCh := ch
		newCnt := wf.count
		// Zone: the candidate may carry a different zone letter (e.g. the time
		// pattern's generic "v") than the one requested. The zone letter
		// encodes the format kind (z name, O localized GMT offset, ...), so
		// honor the REQUESTED letter and width rather than the candidate's;
		// otherwise shortOffset/longOffset both collapse to the candidate's
		// rendering.
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
		// hour. Padding for a forced non-preferred clock is applyHourCycle's
		// job; it always runs after this on the resolved pattern.
		if cls == 'h' {
			switch {
			case wf.count >= 2:
				newCnt = 2
			case c.zoneKeepsHourWidth() && (ch == 'H' || ch == 'k'):
				// With a zone field and no seconds, ICU keeps the matched 24-hour
				// pattern's own hour width rather than unpadding a numeric hour:
				// de's "HH:mm v" stays "09:07 UTC" while ja's "H:mm v" stays
				// "9:07 GMT...". Preserve the candidate's count.
				newCnt = cnt
			default:
				newCnt = 1
			}
		}
		// Minute / second: requested and pattern fields are both numeric,
		// and UTS #35 keeps the pattern's own length in the
		// numeric<->numeric case, so the candidate width always wins
		// (en "HH:mm:ss" stays padded for a numeric request, ko
		// "a h시 m분 s초" stays unpadded for a 2-digit one — both match
		// Intl).
		if cls == 'm' || cls == 's' {
			newCnt = cnt
		}
		b.WriteString(strings.Repeat(string(outCh), newCnt))
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
	}
	switch {
	case datePat != "" && timePat != "":
		return c.combineDateTime(connectorStyle(want), datePat, timePat)
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
	avail := c.skeletonPool()
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
		if sc := matchScore(wantP, have); sc < bestScore {
			bestScore = sc
			best = avail[k]
		}
	}
	if best != "" {
		return c.adjustWidths(best, want)
	}
	return ""
}
