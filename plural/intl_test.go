package plural_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hakastein/gocldr/plural"
)

// intlRow is one Intl.PluralRules result captured from Node/V8 (full-ICU).
type intlRow struct {
	Locale   string `json:"locale"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	MinFrac  int    `json:"minFrac"`
	MaxFrac  int    `json:"maxFrac"`
	Category string `json:"category"`
}

// loadRows decodes a JSON fixture into a non-empty slice of rows.
func loadRows[T any](t *testing.T, path string) []T {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read fixture")
	var rows []T
	require.NoError(t, json.Unmarshal(data, &rows), "unmarshal fixture")
	require.NotEmpty(t, rows, "no rows loaded")
	return rows
}

// selectCategory dispatches on a fixture row's rule type.
func selectCategory(t *testing.T, typ, locale string, ops plural.Operands) plural.Category {
	t.Helper()
	switch typ {
	case "cardinal":
		return plural.Cardinal(locale, ops)
	case "ordinal":
		return plural.Ordinal(locale, ops)
	}
	require.Failf(t, "unknown type", "type %q", typ)
	return ""
}

// TestIntlParity asserts the generated tables agree with JavaScript's
// Intl.PluralRules over a wide locale x type x value matrix. Both derive from
// the same CLDR data, so the match rate must be 100%; any mismatch signals an
// operand or parsing bug.
//
// The matrix is committed under testdata/intl_plurals.json, produced by
// internal/gen/intl.js (see that file to regenerate).
func TestIntlParity(t *testing.T) {
	rows := loadRows[intlRow](t, "testdata/intl_plurals.json")

	for _, r := range rows {
		// Use the string form (authoritative for v/w/f/t), matching the
		// fraction-digit shape Intl was given.
		ops, err := plural.OperandsFromString(r.Value)
		require.NoErrorf(t, err, "OperandsFromString(%q)", r.Value)

		got := selectCategory(t, r.Type, r.Locale, ops)
		assert.Equalf(t, r.Category, string(got),
			"%s %s value=%q (ops=%+v)", r.Type, r.Locale, r.Value, ops)
	}
}

// TestNewOperandsParity checks the float-based NewOperands path against Intl as
// well, ensuring the fraction-digit formatting logic produces the same
// operands as the string path for the same min/max fraction digits.
func TestNewOperandsParity(t *testing.T) {
	rows := loadRows[intlRow](t, "testdata/intl_plurals.json")

	for _, r := range rows {
		ops := plural.NewOperands(parseF(t, r.Value), r.MinFrac, r.MaxFrac)
		got := selectCategory(t, r.Type, r.Locale, ops)
		assert.Equalf(t, r.Category, string(got),
			"%s %s value=%q minF=%d maxF=%d", r.Type, r.Locale, r.Value, r.MinFrac, r.MaxFrac)
	}
}

func parseF(t *testing.T, s string) float64 {
	t.Helper()
	ops, err := plural.OperandsFromString(s)
	require.NoError(t, err)
	return ops.N
}
