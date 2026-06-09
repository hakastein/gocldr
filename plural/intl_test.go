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

func loadIntlRows(t *testing.T) []intlRow {
	t.Helper()
	data, err := os.ReadFile("testdata/intl_plurals.json")
	require.NoError(t, err, "read intl data")
	var rows []intlRow
	require.NoError(t, json.Unmarshal(data, &rows), "unmarshal intl data")
	require.NotEmpty(t, rows, "no intl rows loaded")
	return rows
}

// TestIntlParity asserts the generated tables agree with JavaScript's
// Intl.PluralRules over a wide locale x type x value matrix. Both derive from
// the same CLDR data, so the match rate must be 100%; any mismatch signals an
// operand or parsing bug.
//
// The matrix is committed under testdata/intl_plurals.json, produced by
// internal/gen/intl.js (see that file to regenerate).
func TestIntlParity(t *testing.T) {
	rows := loadIntlRows(t)

	for _, r := range rows {
		// Use the string form (authoritative for v/w/f/t), matching the
		// fraction-digit shape Intl was given.
		ops, err := plural.OperandsFromString(r.Value)
		require.NoErrorf(t, err, "OperandsFromString(%q)", r.Value)

		var got plural.Category
		switch r.Type {
		case "cardinal":
			got = plural.Cardinal(r.Locale, ops)
		case "ordinal":
			got = plural.Ordinal(r.Locale, ops)
		}
		assert.Equalf(t, r.Category, string(got),
			"%s %s value=%q (ops=%+v)", r.Type, r.Locale, r.Value, ops)
	}
}

// TestNewOperandsParity checks the float-based NewOperands path against Intl as
// well, ensuring the fraction-digit formatting logic produces the same
// operands as the string path for the same min/max fraction digits.
func TestNewOperandsParity(t *testing.T) {
	rows := loadIntlRows(t)

	for _, r := range rows {
		var got plural.Category
		switch r.Type {
		case "cardinal":
			got = plural.CardinalFor(r.Locale, parseF(r.Value), r.MinFrac, r.MaxFrac)
		case "ordinal":
			got = plural.OrdinalFor(r.Locale, parseF(r.Value), r.MinFrac, r.MaxFrac)
		}
		assert.Equalf(t, r.Category, string(got),
			"%s %s value=%q minF=%d maxF=%d", r.Type, r.Locale, r.Value, r.MinFrac, r.MaxFrac)
	}
}

func parseF(s string) float64 {
	ops, err := plural.OperandsFromString(s)
	if err != nil {
		return 0
	}
	return ops.N
}
