// Package trainingrecord holds module-root assets shared by the backend
// binary. The embedded migrations live here because //go:embed cannot
// reference paths outside the embedding file's directory.
package trainingrecord

import "embed"

// MigrationsFS contains the ordered SQL migration files applied at startup.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
