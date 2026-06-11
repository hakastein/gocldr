package decimal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hakastein/gocldr/internal/decimal"
)

func TestRoundFixed(t *testing.T) {
	tests := []struct {
		name             string
		abs              float64
		keepFrac         int
		wantInt, wantFra string
	}{
		// Half-away-from-zero on the shortest decimal, not Go's half-to-even and
		// not the float-multiply artefact (8.575*100 == 857.4999...).
		{name: "8.575 to 2", abs: 8.575, keepFrac: 2, wantInt: "8", wantFra: "58"},
		{name: "1.005 to 2", abs: 1.005, keepFrac: 2, wantInt: "1", wantFra: "01"},
		{name: "2.5 to 0", abs: 2.5, keepFrac: 0, wantInt: "3", wantFra: ""},
		{name: "1.5 to 0", abs: 1.5, keepFrac: 0, wantInt: "2", wantFra: ""},
		{name: "carry grows magnitude", abs: 999.9, keepFrac: 0, wantInt: "1000", wantFra: ""},
		{name: "pads to keepFrac", abs: 1.5, keepFrac: 3, wantInt: "1", wantFra: "500"},
		{name: "zero", abs: 0, keepFrac: 2, wantInt: "0", wantFra: "00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotInt, gotFra := decimal.RoundFixed(tc.abs, tc.keepFrac)
			assert.Equal(t, tc.wantInt, gotInt, "int")
			assert.Equal(t, tc.wantFra, gotFra, "frac")
		})
	}
}

func TestIncrement(t *testing.T) {
	assert.Equal(t, "1", decimal.Increment(""))
	assert.Equal(t, "2", decimal.Increment("1"))
	assert.Equal(t, "10", decimal.Increment("9"))
	assert.Equal(t, "100", decimal.Increment("99"))
	assert.Equal(t, "130", decimal.Increment("129"))
}

func TestNormInt(t *testing.T) {
	assert.Equal(t, "0", decimal.NormInt(""))
	assert.Equal(t, "0", decimal.NormInt("000"))
	assert.Equal(t, "5", decimal.NormInt("005"))
	assert.Equal(t, "100", decimal.NormInt("100"))
}
