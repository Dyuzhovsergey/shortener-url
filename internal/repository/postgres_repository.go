package repository

import (
	"context"
	"database/sql"
)

// PostgresRepository — реализация Repository в PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository — конструктор.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Save(ctx context.Context, shortID, originalURL string) error {
	var exists bool

	row := r.db.QueryRowContext(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM short_urls WHERE short_id = $1)",
		shortID,
	)

	err := row.Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		_, err := r.db.ExecContext(ctx,
			"UPDATE short_urls SET original_url = $1 WHERE short_id = $2",
			originalURL, shortID,
		)
		return err
	}

	_, err = r.db.ExecContext(ctx,
		"INSERT INTO short_urls (short_id, original_url) VALUES ($1, $2)",
		shortID, originalURL,
	)
	return err
}

// Get возвращает оригинальный URL по shortID.
func (r *PostgresRepository) Get(ctx context.Context, shortID string) (string, bool) {
	const query = `
		SELECT original_url
		FROM short_urls
		WHERE short_id = $1;
	`

	var original string
	err := r.db.QueryRowContext(ctx, query, shortID).Scan(&original)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {

		return "", false
	}
	return original, true
}
