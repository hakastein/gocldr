# Architecture

`gocldr` is three independent, dependency-free formatter packages plus a pinned
generation toolchain.

## Packages

- **`datetime/`** — `Intl.DateTimeFormat`-style formatting. `datetime.go` resolves
  a locale and interprets CLDR patterns; `pattern.go`/`skeleton.go` handle pattern
  and skeleton selection; `zone.go`/`zonealias.go` handle time-zone naming. Shared
  tables live in the generated `tables_gen.go`; per-locale data lives in the opt-in
  `datetime/locales/<tag>` packages (see below).
- **`number/`** — `Intl.NumberFormat`-style decimal/percent/currency formatting.
  `number.go` is the entry point; `locale.go` resolves locale data (per-locale
  tables live in the opt-in `number/locales/<tag>` packages); `pattern.go`
  and `round.go` handle pattern application and rounding; `plural_bridge.go` exposes
  the plural-category lookups used for currency display names (it imports `plural`).
- **`plural/`** — CLDR cardinal/ordinal plural categories, compiled from the CLDR
  plural rules into Go predicates in `tables_gen.go` (no per-locale packages).

## Locale data (opt-in)

Per-locale CLDR tables are generated into small packages under
`datetime/locales/<tag>` and `number/locales/<tag>`, each of which registers
itself with its formatter's data registry in `init`. Programs blank-import only
the locales they use, so a binary never carries data it does not need. Three
convenience umbrellas exist: `locales/<tag>` (both domains, one locale),
`locales/all` (both domains, every locale), and the per-domain `*/locales/all`.
The `plural` package compiles its rules directly into `tables_gen.go` and needs
no registration.

## Fault-tolerance / fallback

Each formatter resolves a requested BCP-47 locale through the CLDR fallback chain
(exact match → explicit `parentLocale` → trailing-subtag truncation → root/en).
Unknown locales degrade rather than error (`datetime.Format` falls back to RFC3339).

## Generated data

The CLDR tables (the shared `tables_gen.go` plus the per-locale `locales/<tag>`
packages) and the Node `Intl.*` golden fixtures (`testdata/intl_*.json`) must
describe the same CLDR release, so both are produced
by the pinned Docker toolchain in `gen/` (Node→ICU→CLDR + Go). Regenerate with
`make gen`; never run the generators on the host and never hand-edit generated
files. See CONTRIBUTING.md.
