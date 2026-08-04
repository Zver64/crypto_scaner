// Package migrations owns the SQL migration assets embedded into the migration
// command. Keeping the files here makes the checked-in SQL the runtime source
// of truth.
package migrations

import "embed"

const CurrentVersion int64 = 1

// Files contains every migration required by the MVP.
//
//go:embed *.sql
var Files embed.FS
