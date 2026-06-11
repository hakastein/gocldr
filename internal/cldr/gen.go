package cldr

// Helpers shared by the generator programs (number, datetime and the locales
// umbrella). Gen-only, like the rest of the package.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

// MustDataDir returns $CLDR_DATA (the pinned CLDR JSON tree, set by the gen
// image), exiting fatally when unset.
func MustDataDir() string {
	base := os.Getenv("CLDR_DATA")
	if base == "" {
		log.Fatal("gen: CLDR_DATA is unset; run via `make gen`, never on the host")
	}
	return base
}

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
// (some per-locale files are legitimately absent); any other read error or
// malformed JSON is fatal.
func LoadJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		log.Fatalf("gen: read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("gen: parse %s: %v", path, err)
	}
	return true
}

// LoadNumberingSystems reads cldr-core's numberingSystems.json and returns
// the numeric systems as name -> ten digit glyphs. The runtime digit
// substitution relies on every set having exactly ten glyphs, so anything else
// is fatal.
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
		if ns.Type != "numeric" || ns.Digits == "" {
			continue
		}
		if n := utf8.RuneCountInString(ns.Digits); n != 10 {
			log.Fatalf("gen: numbering system %s has %d digit glyphs, want 10", name, n)
		}
		out[name] = ns.Digits
	}
	return out
}

// ResolveNumberDefaults resolves a locale's numbers.json defaults: the default
// numbering system (latn when unstated, with the ICU override applied — ICU
// resolves a handful of region-neutral locales via likely-subtags maximization,
// so Intl disagrees with CLDR's defaultNumberingSystem there) and that system's
// symbols block, falling back to latn's (nil when neither exists).
func ResolveNumberDefaults(loc string, numbers map[string]json.RawMessage) (string, map[string]string) {
	var ns string
	_ = json.Unmarshal(numbers["defaultNumberingSystem"], &ns)
	if ns == "" {
		ns = "latn"
	}
	if ov, ok := ICUNumberingOverride[loc]; ok {
		ns = ov
	}
	for _, key := range []string{"symbols-numberSystem-" + ns, "symbols-numberSystem-latn"} {
		raw, ok := numbers[key]
		if !ok {
			continue
		}
		var sym map[string]string
		if err := json.Unmarshal(raw, &sym); err == nil && sym != nil {
			return ns, sym
		}
	}
	return ns, nil
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

// EmitLocalePackages writes one self-registering package per locale tag at
// localesDir/<tag>/data_gen.go plus the localesDir/all aggregator that
// blank-imports them all, then prunes stale per-locale directories.
// domain is the formatter package name ("datetime", "number"); source is the
// CLDR source line for the generated headers; tags must be sorted; writeBody
// emits the composite literal after "&data.LocaleData" for one tag.
func EmitLocalePackages(localesDir, domain, source string, tags []string, writeBody func(buf *bytes.Buffer, tag string)) {
	importBase := "github.com/hakastein/gocldr/" + domain

	for _, tag := range tags {
		var lb bytes.Buffer
		lb.WriteString("// Code generated by internal/gen; DO NOT EDIT.\n")
		lb.WriteString("// Source: " + source + "\n\n")
		fmt.Fprintf(&lb, "// Package locale registers the %s locale data for %q.\n", domain, tag)
		fmt.Fprintf(&lb, "// Blank-import it to make that locale available to gocldr/%s.\n", domain)
		lb.WriteString("package locale\n\n")
		fmt.Fprintf(&lb, "import %q\n\n", importBase+"/internal/data")
		lb.WriteString("func init() {\n")
		fmt.Fprintf(&lb, "\tdata.Register(%q, &data.LocaleData", tag)
		writeBody(&lb, tag)
		lb.WriteString(")\n}\n")

		dir := filepath.Join(localesDir, tag)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
		if err := WriteFormatted(filepath.Join(dir, "data_gen.go"), lb.Bytes()); err != nil {
			log.Fatal(err)
		}
	}

	var ab bytes.Buffer
	ab.WriteString("// Code generated by internal/gen; DO NOT EDIT.\n")
	ab.WriteString("// Source: " + source + "\n\n")
	fmt.Fprintf(&ab, "// Package all blank-imports every %s per-locale data package, so a\n", domain)
	ab.WriteString("// program that imports it registers the data for every supported locale.\n")
	ab.WriteString("package all\n\n")
	ab.WriteString("import (\n")
	for _, tag := range tags {
		fmt.Fprintf(&ab, "\t_ %q\n", importBase+"/locales/"+tag)
	}
	ab.WriteString(")\n")
	allDir := filepath.Join(localesDir, "all")
	if err := os.MkdirAll(allDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := WriteFormatted(filepath.Join(allDir, "all_gen.go"), ab.Bytes()); err != nil {
		log.Fatal(err)
	}

	PruneLocaleDirs(localesDir, tags)
}

// PruneLocaleDirs deletes localesDir subdirectories (other than "all") whose
// name is not in keep, so a CLDR bump that drops a locale does not leave a
// stale generated package behind.
func PruneLocaleDirs(localesDir string, keep []string) {
	keepSet := make(map[string]bool, len(keep))
	for _, tag := range keep {
		keepSet[tag] = true
	}
	ents, err := os.ReadDir(localesDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range ents {
		if !e.IsDir() || e.Name() == "all" || keepSet[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(localesDir, e.Name())); err != nil {
			log.Fatal(err)
		}
	}
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
