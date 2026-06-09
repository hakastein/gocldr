# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial standalone module: the `datetime`, `number`, and `plural` CLDR
  formatters extracted verbatim from gofluent, plus the pinned CLDR generation
  toolchain.
- Locale data is now modular: each locale's `datetime` and `number` data lives
  in its own per-locale package and is opt-in via a blank import of
  `gocldr/locales/<tag>` (both domains for one locale), the per-domain
  `gocldr/{datetime,number}/locales/<tag>`, or `gocldr/locales/all` for the full
  set.

### Changed

- Lowered the Go floor to 1.23 (was 1.26), now that the locale data is
  modularized and no single generated table is large enough to require the newer
  toolchain.
