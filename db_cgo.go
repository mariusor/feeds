//go:build cgo
// +build cgo

package feeds

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func openDb(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite3", dbPath)
}
