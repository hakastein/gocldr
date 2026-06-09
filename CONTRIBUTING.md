# Contributing

## Requirements

- Go **1.23** or newer.
- Docker (only for regenerating CLDR tables/fixtures).

## Build & test

```sh
go build ./...
go test ./...
make lint        # go vet + staticcheck (pinned) + gofmt check
go test -race ./...
```

All of `gofmt -l .`, `go vet ./...`, `staticcheck ./...`, and `go test -race ./...`
must pass before a change is done.

## Testing discipline

Tests are black-box: external `_test` packages, exported API only, testify,
table-driven via `t.Run`. No test-only seams in production code.

## Submitting changes

1. Fork the repository and create a topic branch off `main`.
2. Make a focused change and keep the tree green — `gofmt -l .`, `go vet ./...`,
   `staticcheck ./...`, and `go test -race ./...` must all pass (`make lint`
   runs the first three).
3. Write a clear commit message: a concise, imperative subject scoped to the
   package (e.g. `number: fix grouping for es-419`), with a body explaining the
   *why* when it is not obvious.
4. Record any user-facing change under `[Unreleased]` in `CHANGELOG.md`.
5. Push your branch and open a pull request. Fill in the PR template and link any
   related issues.

By contributing you agree that your work is licensed under the project's
Apache-2.0 license.

## Regenerating CLDR data

The tables and the `Intl.*` golden fixtures must describe the same CLDR release,
so generation is pinned to a hermetic Docker image (`gen/Dockerfile`):

```sh
make gen
```

Never run the generators or their Node fixture scripts on the host, and never
hand-edit `tables_gen.go` or `testdata/`. When formatting behavior changes,
regenerate fixtures with `make gen` rather than editing expected values by hand.
