// Package cldr holds CLDR-derived tables shared by the number and datetime code
// generators (both //go:build ignore programs run via `go run`). It is gen-only
// and not imported by any runtime package.
package cldr

// ICUNumberingOverride pins the default numbering system for locales where ICU
// (and thus JavaScript's Intl) disagrees with CLDR's defaultNumberingSystem.
// ICU resolves these via likely-subtags maximization of the bare/region tag; the
// result is identical for Intl.NumberFormat and Intl.DateTimeFormat, so both
// generators consume this one table. Verified against Node full-ICU
// resolvedOptions().numberingSystem.
var ICUNumberingOverride = map[string]string{
	"ar":         "latn",
	"az-Arab":    "latn",
	"az-Arab-IQ": "latn",
	"az-Arab-TR": "latn",
	"bgn":        "latn",
	"bgn-AE":     "latn",
	"bgn-AF":     "latn",
	"bgn-IR":     "latn",
	"bgn-OM":     "latn",
	"hnj":        "latn",
	"hnj-Hmnp":   "latn",
	"mni-Mtei":   "beng",
	"sat-Deva":   "olck",
	"sdh":        "latn",
	"sdh-IQ":     "latn",
}
