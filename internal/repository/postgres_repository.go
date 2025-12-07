package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
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
	const query = `
		INSERT INTO short_urls (short_id, original_url)
		VALUES ($1, $2)
		ON CONFLICT (short_id) DO UPDATE
		SET original_url = EXCLUDED.original_url;
	`

	_, err := r.db.ExecContext(ctx, query, shortID, originalURL)
	if err == nil {
		return nil
	}
	// Проверяем, не unique violation ли это
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		const selectQuery = `
			SELECT short_id 
			FROM short_urls 
			WHERE original_url = $1
			LIMIT 1;
		`
		var existingShortID string
		row := r.db.QueryRowContext(ctx, selectQuery, originalURL)
		if scanErr := row.Scan(&existingShortID); scanErr == nil {
			return &ErrOriginalAlreadyExists{ShortID: existingShortID}
		}
		return err
	}

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
