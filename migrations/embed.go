package migrations

import "embed"

// SQLite contains the embedded SQLite migration files.
//
//go:embed sqlite/*.sql
var SQLite embed.FS
