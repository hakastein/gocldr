# gocldr

CLDR-driven date/time, number, percent, and currency formatting for Go, plus
CLDR plural rules — a standalone port of the relevant `Intl.*` behavior,
generated directly from the Unicode CLDR data. Zero runtime dependencies
(testify is test-only).

It is useful to anyone who wants `Intl.NumberFormat` / `Intl.DateTimeFormat` /
`Intl.PluralRules`-style formatting in Go.

## Packages

- `github.com/hakastein/gocldr/datetime` — `Intl.DateTimeFormat`-style date/time formatting.
- `github.com/hakastein/gocldr/number` — `Intl.NumberFormat`-style decimal/percent/currency formatting.
- `github.com/hakastein/gocldr/plural` — CLDR cardinal/ordinal plural categories.

## Correctness

Behavior matches ECMA-402 `Intl.*`. The number
and datetime formatters are checked against Node `Intl.*` golden fixtures, and
the plural rules against the CLDR sample data, all via `go test ./...`. The CLDR
tables are generated; never hand-edit `tables_gen.go` or `testdata/`.

## Requirements

Go 1.23 or newer.

## Status

Pre-1.0. The API may change, but changes are minimized and recorded in
CHANGELOG.md.
