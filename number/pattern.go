package number

import "strings"

// subpattern is one half of a CLDR number pattern (positive or negative): the
// literal prefix and suffix around the numeric body.
type subpattern struct {
	prefix string
	suffix string
	set    bool
}

// splitSubpatterns splits a CLDR pattern on ';' into its positive and negative
// subpatterns and extracts the prefix/suffix of each.
func splitSubpatterns(pattern string) (subpattern, subpattern) {
	var pos, neg subpattern
	parts := strings.SplitN(pattern, ";", 2)
	pos = extractAffixes(parts[0])
	pos.set = true
	if len(parts) == 2 {
		neg = extractAffixes(parts[1])
		neg.set = true
	}
	return pos, neg
}

// extractAffixes splits one subpattern into prefix, numeric body and suffix.
// The numeric body is the maximal run containing only #, 0, comma and dot.
func extractAffixes(p string) subpattern {
	runes := []rune(p)
	start := -1
	end := -1
	for i, r := range runes {
		if r == '#' || r == '0' || r == ',' || r == '.' {
			if start < 0 {
				start = i
			}
			end = i
		}
	}
	if start < 0 {
		// No numeric body (shouldn't happen for valid patterns).
		return subpattern{prefix: p}
	}
	return subpattern{
		prefix: string(runes[:start]),
		suffix: string(runes[end+1:]),
	}
}

// patternGroupSizes returns the primary and secondary grouping sizes from a
// pattern (the integer part of the positive subpattern). A return of (0, 0)
// means no grouping. For "#,##,##0.###" it returns (3, 2); for "#,##0.###" it
// returns (3, 3).
func patternGroupSizes(pattern string) (int, int) {
	// Use the positive subpattern's integer section.
	pos := strings.SplitN(pattern, ";", 2)[0]
	// Strip everything from the decimal point onward.
	if i := strings.IndexByte(pos, '.'); i >= 0 {
		pos = pos[:i]
	}
	// Keep only the digit/comma run.
	var b strings.Builder
	for _, r := range pos {
		if r == '#' || r == '0' || r == ',' {
			b.WriteRune(r)
		}
	}
	intPart := b.String()
	lastComma := strings.LastIndexByte(intPart, ',')
	if lastComma < 0 {
		return 0, 0
	}
	primary := len(intPart) - lastComma - 1
	rest := intPart[:lastComma]
	prevComma := strings.LastIndexByte(rest, ',')
	if prevComma < 0 {
		return primary, primary
	}
	secondary := len(rest) - prevComma - 1
	return primary, secondary
}
