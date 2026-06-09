//go:generate go run ./internal/gen

// Package gocldr is the module root. It carries no API of its own; the usable
// packages are gocldr/datetime, gocldr/number, and gocldr/plural, with the
// per-locale CLDR data opt-in via gocldr/locales/<tag> (both domains) or
// gocldr/locales/all. The go:generate directive above regenerates the
// cross-domain locale umbrellas under locales/.
package gocldr
