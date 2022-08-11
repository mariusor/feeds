//go:build !cgo
// +build !cgo

package feeds

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func openDb(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite", dbPath)
}
