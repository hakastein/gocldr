# CLAUDE.md — gocldr

Guidance for Claude Code and other AI assistants working in this repository. Read
the standard docs first:

- **[ARCHITECTURE.md](ARCHITECTURE.md)** — the three formatter packages, the
  fallback model, and why the CLDR tables are generated.
- **[CONTRIBUTING.md](CONTRIBUTING.md)** — build/test, linting, testing
  discipline, and how to regenerate CLDR data with `make gen`.
- **[README.md](README.md)** — what the library is and how to use it.

## Working agreements

- **Provenance is stated, not hidden.** This codebase is LLM-generated. The guard
  is verification: the formatters are checked against Node `Intl.*` golden fixtures
  and CLDR sample data via `go test ./...`. Trust the tests over the prose.
- **Match ECMA-402 `Intl.*`.** Behavior matches the ECMA-402 `Intl.*` spec.
  When changing formatting, regenerate fixtures with `make gen` rather than
  hand-editing expected values.
- **Never run the generators on the host.** Use `make gen` (pinned Docker
  toolchain). Do not edit `tables_gen.go` or `*/testdata/` by hand.
- **Tests are black-box.** External `_test` packages, exported API only, testify,
  table-driven. No test-only seams in production code.
- **Keep checks green.** `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, and
  `go test -race ./...` must all pass before a change is done.
