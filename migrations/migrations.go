// Package migrations for description migrat
package migrations

import (
	"context"
	"database/sql"
)

func Run(ctx context.Context, db *sql.DB) error {
	const createTable = `
		CREATE TABLE IF NOT EXISTS short_urls (
			short_id TEXT PRIMARY KEY,
			original_url TEXT NOT NULL
		);
	`
	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return err
	}

	const createUniqueIndex = `
		CREATE UNIQUE INDEX IF NOT EXISTS short_urls_original_url_uindex
		ON short_urls (original_url);
	`
	if _, err := db.ExecContext(ctx, createUniqueIndex); err != nil {
		return err
	}
	return nil
}
