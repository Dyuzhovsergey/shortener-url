// Package migrations for description migrat
package migrations

import (
	"context"
	"database/sql"
)

const createShortURLsTable = `
CREATE TABLE IF NOT EXISTS short_urls (
    id SERIAL PRIMARY KEY,
    short_id VARCHAR(255) NOT NULL UNIQUE,
    original_url TEXT NOT NULL
);
`

// Run выполняет необходимые миграции (пока одна, но можно наращивать).
func Run(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, createShortURLsTable)
	return err
}
