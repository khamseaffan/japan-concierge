// Package migrations exposes the goose-formatted SQL migrations as an
// embedded filesystem so they can be run programmatically by tests (and
// eventually by the binary itself for self-migration on startup).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
