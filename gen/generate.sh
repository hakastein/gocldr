#!/bin/sh
# Runs inside the pinned gen image (see gen/Dockerfile). Regenerates every CLDR
# table and its golden Intl fixtures against the single pinned CLDR release, then
# verifies the module. Invoke via `make gen`, never on the host.
set -eu

# Writable caches for the host-uid container user.
export HOME=/tmp GOCACHE=/tmp/gocache GOPATH=/tmp/gopath GOFLAGS=-mod=mod

echo "==> Node $(node -v) | ICU $(node -e 'process.stdout.write(process.versions.icu)') | CLDR $(node -e 'process.stdout.write(process.versions.cldr)')"
echo "==> CLDR JSON: ${CLDR_DATA}"
echo "==> go $(go version)"

# Each formatter package's //go:generate directives run both its table
# generator (reading $CLDR_DATA) and its Node fixture dump (using this image's
# Intl.*). The module root deliberately carries no directive.
echo "==> go generate ./..."
go generate ./...

# The cross-domain locale umbrellas (locales/<tag>, locales/all) regenerate
# strictly AFTER the per-domain generators so they track the locale sets just
# produced; this is the only place the umbrella generator runs. Reads no CLDR
# data.
echo "==> go run ./internal/gen (cross-domain umbrellas)"
go run ./internal/gen

echo "==> go test ./..."
go test ./...

echo "==> done. CLDR release: $(node -e 'process.stdout.write(process.versions.cldr)')"
