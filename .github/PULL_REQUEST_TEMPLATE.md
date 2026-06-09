<!--
Thanks for contributing to gocldr. Please keep the PR focused on a single
concern, and see CONTRIBUTING.md for the conventions this project follows.
-->

## Summary

<!-- What does this change do, and why? -->

## Checklist

- [ ] `go test ./...` passes (and `go test -race ./...` for concurrency-sensitive changes)
- [ ] `gofmt -l .`, `go vet ./...`, and `staticcheck ./...` are clean
- [ ] Behavior still matches fluent.js / ECMA-402 `Intl.*` where relevant
- [ ] Generated files (`cldr/*/tables_gen.go`, `testdata/`) were regenerated with `make gen`, not hand-edited
- [ ] Docs and `CHANGELOG.md` updated for any user-facing change

## Related issues

<!-- e.g. Fixes #123 -->
