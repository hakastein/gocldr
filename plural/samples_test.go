package plural_test

import (
	"encoding/json"
	"os"
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

// TestCLDRSamples is a self-consistency proof straight from CLDR: every
// @integer / @decimal sample listed under a category must be classified into
// that same category by the generated rules, for every locale in the data.
//
// The samples are committed under testdata/cldr_samples.json, produced by
// internal/gen/samples.js (see that file to regenerate).
func TestCLDRSamples(t *testing.T) {
	data, err := os.ReadFile("testdata/cldr_samples.json")
	require.NoError(t, err, "read samples")

	var rows []sampleRow
	require.NoError(t, json.Unmarshal(data, &rows), "unmarshal samples")
	require.NotEmpty(t, rows, "no samples loaded")

	for _, r := range rows {
		ops, err := plural.OperandsFromString(r.Value)
		require.NoErrorf(t, err, "%s %s %q: OperandsFromString", r.Type, r.Locale, r.Value)

		var got plural.Category
		switch r.Type {
		case "cardinal":
			got = plural.Cardinal(r.Locale, ops)
		case "ordinal":
			got = plural.Ordinal(r.Locale, ops)
		default:
			require.Failf(t, "unknown type", "type %q", r.Type)
		}
		assert.Equalf(t, r.Category, string(got),
			"%s %s value=%q (ops=%+v)", r.Type, r.Locale, r.Value, ops)
	}
}
