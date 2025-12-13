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
	// 1) Пытаемся вставить новую запись. Если original_url уже есть — ничего не вставится.
	const insert = `
		INSERT INTO short_urls (short_id, original_url, user_id)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (original_url) DO NOTHING
		RETURNING short_id;
	`

	var returnedShortID string
	err := r.db.QueryRowContext(ctx, insert, shortID, originalURL, userID).Scan(&returnedShortID)
	if err == nil {
		// вставили новую запись
		return nil
	}

	if err != sql.ErrNoRows {
		// ошибка БД
		return err
	}

	// 2) Если ничего не вставили — значит original_url уже был, достаём его short_id
	const selectExisting = `
		SELECT short_id
		FROM short_urls
		WHERE original_url = $1
		LIMIT 1;
	`

	var existingShortID string
	if err := r.db.QueryRowContext(ctx, selectExisting, originalURL).Scan(&existingShortID); err != nil {
		return err
	}

	return &ErrOriginalAlreadyExists{ShortID: existingShortID}
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
