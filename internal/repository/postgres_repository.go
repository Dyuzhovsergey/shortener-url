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
func (r *PostgresRepository) Save(ctx context.Context, shortID, originalURL, userID string) error {
	const q = `
		INSERT INTO short_urls (short_id, original_url, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (original_url) DO UPDATE
		SET short_id = short_urls.short_id
		RETURNING short_id;
	`

	var returnedShortID string
	err := r.db.QueryRowContext(ctx, q, shortID, originalURL, userID).Scan(&returnedShortID)
	if err != nil {
		return err
	}

	// если вернулся НЕ наш shortID — значит original_url уже был в базе
	if returnedShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: returnedShortID}
	}

	return nil
}

// Get возвращает оригинальный URL по shortID.
func (r *PostgresRepository) Get(ctx context.Context, shortID string) (string, bool) {
	const q = `
		SELECT original_url
		FROM short_urls
		WHERE short_id = $1
		LIMIT 1;
	`
	var original string
	err := r.db.QueryRowContext(ctx, q, shortID).Scan(&original)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return original, true
}

// GetUserURLs возвращает shortID Users.
func (r *PostgresRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	if userID == "" {
		return nil, nil
	}

	const q = `
		SELECT short_id, original_url
		FROM short_urls
		WHERE user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []UserURL
	for rows.Next() {
		var u UserURL
		if err := rows.Scan(&u.ShortID, &u.OriginalURL); err != nil {
			return nil, err
		}
		res = append(res, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
