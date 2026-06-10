package locale_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hakastein/gocldr/internal/locale"
)

func TestCanonical(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already canonical", "en-US", "en-US"},
		{"lowercase region", "en-us", "en-US"},
		{"uppercase language", "EN-US", "en-US"},
		{"underscores", "pt_br", "pt-BR"},
		{"script titlecase", "zh-hant", "zh-Hant"},
		{"script with region", "zh_HANT_hk", "zh-Hant-HK"},
		{"bare language", "FR", "fr"},
		{"long subtag lowercased", "en-US-POSIX", "en-US-posix"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, locale.Canonical(tc.in))
		})
	}
}

func TestResolve(t *testing.T) {
	table := map[string]bool{
		"en":      true,
		"en-001":  true,
		"pt":      true,
		"pt-PT":   true,
		"zh-Hant": true,
	}
	parents := map[string]string{
		"en-150": "en-001",
		"pt-AO":  "pt-PT",
	}
	tests := []struct {
		name    string
		tag     string
		parents map[string]string
		want    string
		wantOK  bool
	}{
		{"exact", "pt-PT", parents, "pt-PT", true},
		{"exact after canonicalisation", "PT_pt", parents, "pt-PT", true},
		{"parent override beats truncation", "en-150", parents, "en-001", true},
		{"parent chain to explicit entry", "pt-AO", parents, "pt-PT", true},
		{"truncation", "en-GB", parents, "en", true},
		{"truncation across two subtags", "zh-Hant-HK", parents, "zh-Hant", true},
		{"nil parents ignores overrides", "pt-AO", nil, "pt", true},
		{"miss", "zz", parents, "", false},
		{"empty tag", "", parents, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := locale.Resolve(tc.tag, tc.parents, func(tag string) bool { return table[tag] })
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestResolveCyclicParents pins the guard against a cycling parentLocale
// chain: the walk must terminate and report a miss instead of looping.
func TestResolveCyclicParents(t *testing.T) {
	parents := map[string]string{"a": "b", "b": "a"}
	got, ok := locale.Resolve("a", parents, func(string) bool { return false })
	assert.False(t, ok)
	assert.Empty(t, got)
}
