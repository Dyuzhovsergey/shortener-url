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

	if returnedShortID != shortID {
		return &ErrOriginalAlreadyExists{ShortID: returnedShortID}
	}

	return nil
}

// Get возвращает оригинальный URL по shortID.
func (r *PostgresRepository) Get(ctx context.Context, shortID string) (string, bool, error) {
	const q = `
		SELECT original_url, is_deleted
		FROM short_urls
		WHERE short_id = $1
		LIMIT 1;
	`
	var original string
	var deleted bool

	err := r.db.QueryRowContext(ctx, q, shortID).Scan(&original, &deleted)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if deleted {
		return "", true, ErrDeleted
	}
	return original, true, nil
}

// GetUserURLs возвращает shortID Users.
func (r *PostgresRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	if userID == "" {
		return nil, nil
	}

	const q = `
		SELECT short_id, original_url
		FROM short_urls
		WHERE user_id = $1 AND is_deleted = FALSE
		ORDER BY short_id;
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

// DeleteUserURLs помечает ссылки пользователя как удалённые в базе данных.
func (r *PostgresRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	if userID == "" || len(shortIDs) == 0 {
		return nil
	}

	const q = `
		UPDATE short_urls
		SET is_deleted = TRUE
		WHERE user_id = $1
		  AND short_id = ANY($2)
		  AND is_deleted = FALSE;
	`
	_, err := r.db.ExecContext(ctx, q, userID, shortIDs)
	return err
}

// Stats возвращает агрегированную статистику сервиса.
func (r *PostgresRepository) Stats(ctx context.Context) (Stats, error) {
	const q = `
		SELECT COUNT(*), COUNT(DISTINCT NULLIF(user_id, ''))
		FROM short_urls;
	`

	var stats Stats
	if err := r.db.QueryRowContext(ctx, q).Scan(&stats.URLs, &stats.Users); err != nil {
		return Stats{}, err
	}

	return stats, nil
}
