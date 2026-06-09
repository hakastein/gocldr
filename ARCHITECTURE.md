# Architecture

`gocldr` is three independent, dependency-free formatter packages plus a pinned
generation toolchain.

## Packages

- **`datetime/`** — `Intl.DateTimeFormat`-style formatting. `datetime.go` resolves
  a locale and interprets CLDR patterns; `pattern.go`/`skeleton.go` handle pattern
  and skeleton selection; `zone.go`/`zonealias.go` handle time-zone naming. Locale
  data lives in the generated `tables_gen.go`.
- **`number/`** — `Intl.NumberFormat`-style decimal/percent/currency formatting.
  `number.go` is the entry point; `locale.go` resolves locale data; `pattern.go`
  and `round.go` handle pattern application and rounding; `plural_bridge.go` exposes
  the plural-category lookups used for currency display names (it imports `plural`).
- **`plural/`** — CLDR cardinal/ordinal plural categories, compiled from the CLDR
  plural rules into Go predicates.

## Fault-tolerance / fallback

Each formatter resolves a requested BCP-47 locale through the CLDR fallback chain
(exact match → explicit `parentLocale` → trailing-subtag truncation → root/en).
Unknown locales degrade rather than error (`datetime.Format` falls back to RFC3339).

## Generated data

The CLDR tables (`tables_gen.go`) and the Node `Intl.*` golden fixtures
(`testdata/intl_*.json`) must describe the same CLDR release, so both are produced
by the pinned Docker toolchain in `gen/` (Node→ICU→CLDR + Go). Regenerate with
`make gen`; never run the generators on the host and never hand-edit generated
files. See CONTRIBUTING.md.
