package plural_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/plural"
)

// sampleRow is one expanded CLDR sample: the value string (preserving its
// fraction-digit width) and the plural category CLDR declares for it.
type sampleRow struct {
	Type     string `json:"type"`
	Locale   string `json:"locale"`
	Category string `json:"category"`
	Value    string `json:"value"`
}

// TestCLDRSamples round-trips CLDR's own @integer/@decimal samples through
// the generated rules: every sample listed under a category must classify
// into that same category, for every locale in the data. The samples derive
// from the same CLDR release the tables are generated from, so this guards
// the generator's rule translation — it is NOT an independent oracle; the
// Intl/ICU parity check lives in intl_test.go.
//
// The samples are committed under testdata/cldr_samples.json, produced by
// internal/gen/samples.js (see that file to regenerate).
func TestCLDRSamples(t *testing.T) {
	rows := loadRows[sampleRow](t, "testdata/cldr_samples.json")

	for _, r := range rows {
		ops, err := plural.OperandsFromString(r.Value)
		require.NoErrorf(t, err, "%s %s %q: OperandsFromString", r.Type, r.Locale, r.Value)

		got := selectCategory(t, r.Type, r.Locale, ops)
		assert.Equalf(t, r.Category, string(got),
			"%s %s value=%q (ops=%+v)", r.Type, r.Locale, r.Value, ops)
	}
}
