# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-06-10

### Fixed

- `datetime`: best-fit skeleton matching now seeds its candidate pool with the
  standard date/time patterns, mirroring ICU's DateTimePatternGenerator. Time
  components combined with a time zone render the locale's full time-pattern
  literals (ja `15時04分05秒 協定世界時`, ko `오후 3시 4분 5초`, fa's
  parenthesised zone), and da component requests keep the `'den'` literal of
  its full date pattern. Zone-kind (specific/generic/offset) now influences
  candidate scoring, and minute/second widths follow the UTS #35
  numeric-to-numeric rule.
- `datetime`: a raw pattern's related-Gregorian-year field `r` renders as a
  year instead of being echoed as a literal.
- `number`: `UseGroupingMode` follows ECMA-402 exactly — the string forms
  `"true"` and `"false"` are legacy aliases of `"auto"`, so the string
  `"false"` no longer disables grouping (use the boolean `UseGrouping` for
  that). Covered by 1,400 new golden fixtures.
- `plural`: `OperandsFromString` rejects digit-less strings (`"."`, `"-"`,
  `"c5"`) instead of silently returning zero operands.

### Changed

- Locale-tag canonicalisation and the CLDR fallback walk are now a single
  shared implementation (`internal/locale`); the per-package terminal
  fallbacks (number → root, datetime → `en`, plural → other) are documented
  in ARCHITECTURE.md.
- The datetime golden-fixture suite asserts every row exactly against Node's
  `Intl.DateTimeFormat` with an explicit, test-enforced skip list (fa/th
  non-Gregorian calendars), replacing the bucketed match-rate thresholds.
- Generated datetime locale packages no longer carry three unread zone keys,
  shrinking every per-locale table slightly.
- The generators share their common plumbing via `internal/cldr`, run in
  normal build scope (covered by `go vet`/`staticcheck`), and the fixture
  scripts follow one self-contained output convention.

## [0.1.2] - 2026-06-09

### Added

- README: badges, install, a runnable quickstart (verified to compile and
  print the shown output), a "Registering locale data" guide for the opt-in
  import model, and a fact-checked "Why gocldr, not golang.org/x/text?"
  comparison.
- CONTRIBUTING: an explicit "Submitting changes" flow (branch, keep checks
  green, commit-message convention, CHANGELOG, push, PR).

### Fixed

- Corrected documentation that referenced a non-existent `cldr/` directory or
  leftover Fluent/FTL concepts in `NOTICE`, `ARCHITECTURE.md`, the issue and
  pull-request templates, and the generation toolchain.
- `datetime.Options.TimeZoneName` now documents all six accepted values
  (previously listed only `long`/`short`).

## [0.1.1] - 2026-06-09

### Changed

- Documentation now describes gocldr as a standalone CLDR/`Intl.*` formatter;
  removed the remaining `fluent.js`/`gofluent` references from source comments,
  the generation toolchain, and templates.

## [0.1.0] - 2026-06-09

### Added

- Initial standalone module: the `datetime`, `number`, and `plural` CLDR
  formatters and the pinned, hermetic CLDR generation toolchain.
- Modular locale data: each locale's `datetime` and `number` data lives in its
  own per-locale package and is opt-in via a blank import of
  `gocldr/locales/<tag>` (both domains for one locale), the per-domain
  `gocldr/{datetime,number}/locales/<tag>`, or `gocldr/locales/all` for the full
  set.
- Cross-domain locale umbrellas (`gocldr/locales/<tag>`, `gocldr/locales/all`).

### Changed

- Lowered the Go floor to 1.23 (was 1.26), now that the locale data is
  modularized and no single generated table is large enough to require the newer
  toolchain.

[Unreleased]: https://github.com/hakastein/gocldr/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/hakastein/gocldr/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/hakastein/gocldr/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/hakastein/gocldr/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/hakastein/gocldr/releases/tag/v0.1.0
