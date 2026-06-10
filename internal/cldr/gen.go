package cldr

// Helpers shared by the //go:build ignore generator programs (number,
// datetime and the locales umbrella). Gen-only, like the rest of the package.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
)

// WriteFormatted runs src through gofmt and writes it to path, dumping a
// .broken sibling on a gofmt failure for debugging.
func WriteFormatted(path string, src []byte) error {
	formatted, err := format.Source(src)
	if err != nil {
		_ = os.WriteFile(path+".broken", src, 0o644)
		return fmt.Errorf("gofmt failed for %s: %w (wrote %s.broken)", path, err, path)
	}
	return os.WriteFile(path, formatted, 0o644)
}

// MustJSON reads and unmarshals path into v, exiting fatally on any error.
// Generators use it for inputs that must exist in the pinned CLDR tree.
func MustJSON(path string, v any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("gen: read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		log.Fatalf("gen: parse %s: %v", path, err)
	}
}

// LoadJSON reads and unmarshals path into v. A missing file reports false
// (some per-locale files are legitimately absent); malformed JSON is fatal.
func LoadJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("gen: parse %s: %v", path, err)
	}
	return true
}

// LoadNumberingSystems reads cldr-core's numberingSystems.json and returns
// the numeric systems as name -> ten digit glyphs.
func LoadNumberingSystems(path string) map[string]string {
	var doc struct {
		Supplemental struct {
			NumberingSystems map[string]struct {
				Type   string `json:"_type"`
				Digits string `json:"_digits"`
			} `json:"numberingSystems"`
		} `json:"supplemental"`
	}
	MustJSON(path, &doc)
	out := map[string]string{}
	for name, ns := range doc.Supplemental.NumberingSystems {
		if ns.Type == "numeric" && ns.Digits != "" {
			out[name] = ns.Digits
		}
	}
	return out
}

// LoadParentLocales reads cldr-core's parentLocales.json and returns the
// explicit child -> parent overrides.
func LoadParentLocales(path string) map[string]string {
	var doc struct {
		Supplemental struct {
			ParentLocales struct {
				ParentLocale map[string]string `json:"parentLocale"`
			} `json:"parentLocales"`
		} `json:"supplemental"`
	}
	MustJSON(path, &doc)
	return doc.Supplemental.ParentLocales.ParentLocale
}

// SortedKeys returns the keys of m in ascending order. Generated output is
// emitted in this order so it stays stable across Go's randomized map
// iteration.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// WriteStrMap emits `field: map[string]string{...}, ` with sorted keys, or
// nothing when the map is empty.
func WriteStrMap(buf *bytes.Buffer, field string, m map[string]string) {
	if len(m) == 0 {
		return
	}
	buf.WriteString(field + ": map[string]string{")
	for _, k := range SortedKeys(m) {
		fmt.Fprintf(buf, "%q: %q, ", k, m[k])
	}
	buf.WriteString("}, ")
}
