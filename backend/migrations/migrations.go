// Package migrations owns the SQL migration assets embedded into the migration
// command. Keeping the files here makes the checked-in SQL the runtime source
// of truth.
package migrations

import "embed"

// Files contains every migration required by the application.
//
//go:embed *.sql
var Files embed.FS
