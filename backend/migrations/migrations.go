package migrations

import "embed"

// FS содержит встроенные SQL-миграции базы данных.
//
//go:embed *.sql
var FS embed.FS
