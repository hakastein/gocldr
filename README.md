# gocldr

[![Go Reference](https://pkg.go.dev/badge/github.com/hakastein/gocldr.svg)](https://pkg.go.dev/github.com/hakastein/gocldr)
[![CI](https://github.com/hakastein/gocldr/actions/workflows/ci.yml/badge.svg)](https://github.com/hakastein/gocldr/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hakastein/gocldr)](https://goreportcard.com/report/github.com/hakastein/gocldr)
[![Go version](https://img.shields.io/github/go-mod/go-version/hakastein/gocldr)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

**Locale-aware date, time, number, percent, and currency formatting for Go — plus
CLDR plural rules.** A standalone, zero-dependency port of the `Intl.*` formatters,
generated directly from the Unicode CLDR and verified against ECMA-402.

If you have ever reached for `Intl.NumberFormat`, `Intl.DateTimeFormat`, or
`Intl.PluralRules` in JavaScript, this gives you the same output in Go.

```go
number.Format("de", 1234.5, number.Options{})                                  // 1.234,5
number.Format("en", 1234, number.Options{Style: "currency", Currency: "USD"})  // $1,234.00
plural.CardinalFor("ru", 2, 0, 0)                                              // few
```

## Install

```sh
go get github.com/hakastein/gocldr
```

Requires Go 1.23 or newer. No runtime dependencies.

## Quickstart

Locale data is **opt-in**: you register the locales you need with a blank import,
then call the formatters. You ship only the data you use, not all 700+ locales,
so your binary stays small.

```go
package main

import (
	"fmt"
	"time"

	"github.com/hakastein/gocldr/datetime"
	"github.com/hakastein/gocldr/number"
	"github.com/hakastein/gocldr/plural"

	// Register CLDR data for the locales this program formats (datetime + number).
	// Import only what you need — see "Registering locale data" below.
	// (plural needs no import; "ru" below works without one.)
	_ "github.com/hakastein/gocldr/locales/de"
	_ "github.com/hakastein/gocldr/locales/en"
	_ "github.com/hakastein/gocldr/locales/fr"
)

func main() {
	// Numbers, percent, currency — Intl.NumberFormat style.
	fmt.Println(number.Format("de", 1234.5, number.Options{}))
	// 1.234,5
	fmt.Println(number.Format("en", 1234, number.Options{Style: "currency", Currency: "USD"}))
	// $1,234.00

	// Dates and times — Intl.DateTimeFormat style.
	t := time.Date(2021, 1, 5, 15, 4, 5, 0, time.UTC)
	fmt.Println(datetime.Format("en", t, datetime.Options{DateStyle: "long", TimeStyle: "short", TimeZone: "UTC"}))
	// January 5, 2021 at 3:04 PM
	fmt.Println(datetime.Format("fr", t, datetime.Options{DateStyle: "full", TimeStyle: "medium", TimeZone: "UTC"}))
	// mardi 5 janvier 2021 à 15:04:05

	// Plural categories — Intl.PluralRules style (no locale import needed).
	fmt.Println(plural.CardinalFor("ru", 2, 0, 0))
	// few
}
```

## Registering locale data

Each formatter reads from a registry that locale packages populate in their
`init`. Blank-import the granularity you want:

| Import | Registers |
| --- | --- |
| `gocldr/locales/all` | every locale, datetime **and** number |
| `gocldr/locales/<tag>` | one locale, datetime **and** number |
| `gocldr/datetime/locales/all` | every locale, datetime only |
| `gocldr/datetime/locales/<tag>` | one locale, datetime only |
| `gocldr/number/locales/all` | every locale, number only |
| `gocldr/number/locales/<tag>` | one locale, number only |

```go
import (
	_ "github.com/hakastein/gocldr/locales/en" // en, both formatters
	_ "github.com/hakastein/gocldr/locales/de" // de, both formatters
)
```

`<tag>` is a CLDR locale identifier such as `en`, `en-GB`, `de`, `pt-BR`, or
`zh-Hans`. The `plural` package needs no import — its rules are compiled in.

> **Import the locales you need, not `…/locales/all`.** A single-locale binary is
> ~2 MB; `locales/all` links every locale and produces a ~70 MB binary. Reach for
> `all` only in tests or tooling, never as a convenience default in a service.

If a requested locale is not registered, the formatters fall back through the
CLDR parent chain rather than failing (`number` ends at root data, `datetime`
at `en`; see [ARCHITECTURE.md](ARCHITECTURE.md) for the exact chains).

## Packages

| Package | Mirrors | Formats |
| --- | --- | --- |
| [`datetime`](https://pkg.go.dev/github.com/hakastein/gocldr/datetime) | `Intl.DateTimeFormat` | dates, times, combined styles, skeletons, eras, time-zone names |
| [`number`](https://pkg.go.dev/github.com/hakastein/gocldr/number) | `Intl.NumberFormat` | decimals, percent, currency, grouping, significant/fraction digits |
| [`plural`](https://pkg.go.dev/github.com/hakastein/gocldr/plural) | `Intl.PluralRules` | cardinal and ordinal plural categories |

## Correctness

Behavior matches the ECMA-402 `Intl.*` specification. All three packages are
checked row-exact against golden fixtures captured from Node's `Intl.*` (an
independent ICU implementation): `Intl.NumberFormat`, `Intl.DateTimeFormat`
and `Intl.PluralRules` output respectively — all via `go test ./...`. The only
tolerated divergence is an explicit, test-enforced skip list (currently fa/th,
whose default calendars are non-Gregorian); `plural` additionally round-trips
CLDR's own rule samples. The bundled tables and the fixtures are generated
from the **same pinned CLDR release**, so they agree by construction. See
[ARCHITECTURE.md](ARCHITECTURE.md).

This codebase is LLM-generated; the guard is verification, not authorship. Trust
the tests over the prose.

## Why gocldr, not `golang.org/x/text`?

[`golang.org/x/text`](https://pkg.go.dev/golang.org/x/text) is the Go team's broad
text toolkit, and it is excellent. But the package people reach for by name —
[`golang.org/x/text/unicode/cldr`](https://pkg.go.dev/golang.org/x/text/unicode/cldr)
— is a **parser for CLDR's LDML XML**, a code-generation building block, not a
formatter you call at runtime. The formatting in that ecosystem lives in
`message` + `number` (numbers), `currency`, and `feature/plural` (plurals).
Measured against those, gocldr is narrower and sharper:

| Capability | gocldr | `golang.org/x/text` |
| --- | :---: | :---: |
| Numbers, percent, currency | ✅ | ✅ |
| **Locale-aware date / time** | ✅ | ❌ |
| Plural categories | ✅ | ⚠️ low-level |
| **ECMA-402 `Intl.*` parity (verified vs. Node)** | ✅ | ❌ |
| Import only the locales you use | ✅ | ❌ |
| Collation, encodings, normalization, bidi, language matching, translation | ❌ | ✅ |

- **Date and time is the big gap.** x/text ships no working locale-aware
  date/time formatter — its `date` package is an unimplemented stub. The standard
  library's `time.Format` only covers the mechanics: its month and weekday names
  are hardcoded English, and the ordering and separators are whatever layout you
  bake in by hand — it is not locale-aware. gocldr gives you full
  `Intl.DateTimeFormat`-style output: localized names, date/time styles,
  skeletons, eras, and time-zone names.
- **Output matches JavaScript.** gocldr is verified against Node's `Intl.*`
  golden fixtures, so a value formats identically on your Go backend and your web
  frontend (handy for SSR or shared snapshots). x/text faithfully follows CLDR
  but does not target `Intl.*` output and uses its own option model.
- **You link only the locales you import.** Locale data lives in per-locale
  packages, so a binary that needs one locale links just that locale (~2 MB
  stripped, on par with x/text) and each extra locale adds ~180 KB; x/text cannot
  subset — importing a formatter pulls its whole CLDR table set. The flip side is
  honest: x/text's tables are tightly packed, so its *all-locales* footprint
  (~3 MB) is far smaller than gocldr's `locales/all` (tens of MB). The win is
  paying only for the locales you actually use — not blanket-importing `all`.
- **Familiar API.** If you know `Intl.NumberFormat` / `Intl.DateTimeFormat`,
  gocldr's `Options` will feel like home.

**Prefer `golang.org/x/text` when** you need what gocldr deliberately leaves out
— language matching, collation, character-set encodings, Unicode normalization,
bidi, case mapping, or message/translation catalogs — or when you are already
invested in that ecosystem and need neither date/time formatting nor strict
`Intl.*` parity. gocldr does one thing: CLDR / `Intl.*` formatting.

## Supported locales

All 725 CLDR locales, including region and script variants (`en-GB`, `zh-Hans`,
`pt-BR`, …). Data is regenerated from the pinned CLDR release with `make gen`.

## Documentation

- [API reference (pkg.go.dev)](https://pkg.go.dev/github.com/hakastein/gocldr)
- [ARCHITECTURE.md](ARCHITECTURE.md) — package layout, fallback model, data generation
- [CONTRIBUTING.md](CONTRIBUTING.md) — build, test, commit, and regenerate data
- [CHANGELOG.md](CHANGELOG.md) — notable changes

## Status

Pre-1.0. The API may still change; changes are minimized and recorded in
[CHANGELOG.md](CHANGELOG.md).

## License

Apache License 2.0 — see [LICENSE](LICENSE).

The bundled locale tables are generated from the [Unicode CLDR](https://cldr.unicode.org/),
© Unicode, Inc., under the [Unicode License](https://www.unicode.org/license.txt);
see [NOTICE](NOTICE).
