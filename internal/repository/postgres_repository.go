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
// Save сохраняет оригинальный URL по shortID.
func (r *PostgresRepository) Save(ctx context.Context, shortID, originalURL string) error {
	// 1. Проверяем, есть ли уже такой original_url
	const selectQuery = `
        SELECT short_id
        FROM short_urls
        WHERE original_url = $1
        LIMIT 1;
    `
	var existingShortID string
	err := r.db.QueryRowContext(ctx, selectQuery, originalURL).Scan(&existingShortID)
	if err == nil {
		// Запись с таким original_url уже есть
		return &ErrOriginalAlreadyExists{ShortID: existingShortID}
	}
	if err != nil && err != sql.ErrNoRows {
		// Какая-то ошибка БД
		return err
	}

	// 2. Такого URL ещё нет — вставляем новую запись
	const insertQuery = `
        INSERT INTO short_urls (short_id, original_url)
        VALUES ($1, $2);
    `
	_, err = r.db.ExecContext(ctx, insertQuery, shortID, originalURL)
	return err
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
