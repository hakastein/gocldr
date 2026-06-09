# Contributing

## Requirements

- Go **1.26** or newer (drops to 1.23 after locale-data modularization).
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

## Regenerating CLDR data

The tables and the `Intl.*` golden fixtures must describe the same CLDR release,
so generation is pinned to a hermetic Docker image (`gen/Dockerfile`):

```sh
make gen
```

Never run the generators or their Node fixture scripts on the host, and never
hand-edit `tables_gen.go` or `testdata/`. When formatting behavior changes,
regenerate fixtures with `make gen` rather than editing expected values by hand.
