package migrations

import "embed"

// FS contains Goose SQL migrations.
//
//go:embed *.sql
var FS embed.FS
