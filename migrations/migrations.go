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
			original_url TEXT NOT NULL,
			user_id TEXT,
			is_deleted BOOLEAN NOT NULL DEFAULT FALSE
		);
	`
	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return err
	}

	const addUserID = `
		ALTER TABLE short_urls
		ADD COLUMN IF NOT EXISTS user_id TEXT;
	`
	if _, err := db.ExecContext(ctx, addUserID); err != nil {
		return err
	}

	const addIsDeleted = `
		ALTER TABLE short_urls
		ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT FALSE;
	`
	if _, err := db.ExecContext(ctx, addIsDeleted); err != nil {
		return err
	}

	// Уникальность original_url
	const createUniqueIndexOriginal = `
		CREATE UNIQUE INDEX IF NOT EXISTS short_urls_original_url_uindex
		ON short_urls (original_url);
	`
	if _, err := db.ExecContext(ctx, createUniqueIndexOriginal); err != nil {
		return err
	}

	const createUserIndex = `
		CREATE INDEX IF NOT EXISTS short_urls_user_id_idx
		ON short_urls (user_id);
	`
	if _, err := db.ExecContext(ctx, createUserIndex); err != nil {
		return err
	}

	// Индекс по user_id + is_deleted (ускоряет GET /api/user/urls и “живые” ссылки)
	const createIndexUserAlive = `
		CREATE INDEX IF NOT EXISTS short_urls_user_alive_idx
		ON short_urls (user_id, is_deleted);
	`
	if _, err := db.ExecContext(ctx, createIndexUserAlive); err != nil {
		return err
	}

	return nil
}
