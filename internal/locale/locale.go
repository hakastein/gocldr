// Package locale provides the BCP-47 / CLDR locale-tag canonicalisation and
// the CLDR fallback walk shared by the formatter packages (and their
// generators). Keeping a single implementation guarantees that plural
// selection and number/datetime formatting resolve the same tag identically.
package locale

import "strings"

// Canonical normalises a BCP-47 / CLDR tag for table lookup: underscores are
// treated as subtag separators, the language subtag is lower-cased, two-letter
// region subtags are upper-cased and four-letter script subtags are
// title-cased the way CLDR keys them (e.g. "zh_hant" -> "zh-Hant").
func Canonical(tag string) string {
	tag = strings.ReplaceAll(tag, "_", "-")
	parts := strings.Split(tag, "-")
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

// Resolve walks the CLDR locale fallback chain starting from the canonicalised
// tag — exact match, explicit parentLocale override, then trailing-subtag
// truncation — calling found at each step until it reports a hit, and returns
// the matched tag. parents may be nil for a truncation-only walk (Intl
// resolves plural rules that way: ICU selects pt's rules for pt-AO, not
// pt-PT's, ignoring the parentLocale override). The seen guard terminates the
// walk if a parents chain cycles.
func Resolve(tag string, parents map[string]string, found func(string) bool) (string, bool) {
	cur := Canonical(tag)
	seen := map[string]bool{}
	for cur != "" && !seen[cur] {
		seen[cur] = true
		if found(cur) {
			return cur, true
		}
		if p, ok := parents[cur]; ok {
			cur = p
			continue
		}
		if i := strings.LastIndexByte(cur, '-'); i >= 0 {
			cur = cur[:i]
			continue
		}
		break
	}
	return "", false
}
