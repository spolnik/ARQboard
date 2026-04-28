package migrations

import "embed"

// FS contains SQL migrations used by both tests and the migrate command.
//
//go:embed postgres/*.sql sqlite/*.sql
var FS embed.FS
