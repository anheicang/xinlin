package safelog

import "github.com/zeromicro/go-zero/core/stores/sqlx"

// DisableSQLStatements stops go-zero from logging interpolated SQL, including
// slow/error statements. It doesn't disable business logs or database execution.
func DisableSQLStatements() { sqlx.DisableLog() }
