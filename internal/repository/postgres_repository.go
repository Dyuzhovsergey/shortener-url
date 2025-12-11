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

// Save сохраняет оригинальный URL по shortID.
func (r *PostgresRepository) Save(ctx context.Context, shortID, originalURL string) error {
	// 1. Проверяем, есть ли уже такой original_url
	const query = `
	INSERT INTO short_urls (short_id, original_url)
	VALUES ($1, $2)
	ON CONFLICT (original_url) DO UPDATE
	SET short_id = short_urls.short_id
	RETURNING short_id;
`
	var returnedShortID string
	err := r.db.QueryRowContext(ctx, query, shortID, originalURL).Scan(&returnedShortID)
	if err != nil {
		return err
	}

	// Если вернулся не тот shortID, который мы предлагали,
	// значит URL уже был и мы получили старый shortID.
	if returnedShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: returnedShortID}
	}
	return nil
}

// Get возвращает оригинальный URL по shortID.
func (r *PostgresRepository) Get(ctx context.Context, shortID string) (string, bool) {
	const query = `
		SELECT original_url
		FROM short_urls
		WHERE short_id = $1;
	`
	var originalURL string
	rows := r.db.QueryRowContext(ctx, query, shortID)

	err := rows.Scan(&originalURL)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {

		return "", false
	}
	return originalURL, true
}
