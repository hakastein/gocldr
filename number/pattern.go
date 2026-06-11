package number

import "strings"

// subpattern is one half of a CLDR number pattern (positive or negative): the
// literal prefix and suffix around the numeric body.
type subpattern struct {
	prefix string
	body   string
	suffix string
	set    bool
}

// splitSubpatterns splits a CLDR pattern on ';' into its positive and negative
// subpatterns and extracts the prefix/body/suffix of each.
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
// The numeric body is the maximal run containing only #, 0, comma and dot;
// the generator guarantees every pattern has one.
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
	return subpattern{
		prefix: string(runes[:start]),
		body:   string(runes[start : end+1]),
		suffix: string(runes[end+1:]),
	}
}

// patternGroupSizes returns the primary and secondary grouping sizes from the
// positive subpattern's numeric body. A return of (0, 0) means no grouping.
// For "#,##,##0.###" it returns (3, 2); for "#,##0.###" it returns (3, 3).
func patternGroupSizes(body string) (int, int) {
	if i := strings.IndexByte(body, '.'); i >= 0 {
		body = body[:i]
	}
	lastComma := strings.LastIndexByte(body, ',')
	if lastComma < 0 {
		return 0, 0
	}
	primary := len(body) - lastComma - 1
	rest := body[:lastComma]
	prevComma := strings.LastIndexByte(rest, ',')
	if prevComma < 0 {
		return primary, primary
	}
	secondary := len(rest) - prevComma - 1
	return primary, secondary
}
