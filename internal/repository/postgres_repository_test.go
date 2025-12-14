package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPostgresRepository_Save_OK(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "abc123"
	original := "https://example.com"

	rows := sqlmock.NewRows([]string{"short_id"}).AddRow(shortID)

	// Используем ExpectQuery, потому что внутри Save: QueryRowContext + RETURNING
	mock.ExpectQuery(`(?s)INSERT INTO short_urls`).
		WithArgs(shortID, original, testUser).
		WillReturnRows(rows)

	err = repo.Save(context.Background(), shortID, original, testUser)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Save_Duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	existingShortID := "old123"
	original := "https://example.com"
	newShortID := "new456"

	rows := sqlmock.NewRows([]string{"short_id"}).AddRow(existingShortID)

	mock.ExpectQuery(`(?s)INSERT INTO short_urls`).
		WithArgs(newShortID, original, testUser).
		WillReturnRows(rows)

	err = repo.Save(context.Background(), newShortID, original, testUser)
	require.Error(t, err)

	var dupErr *ErrOriginalAlreadyExists
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, existingShortID, dupErr.ShortID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Get_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "abc123"
	original := "https://example.com"

	rows := sqlmock.NewRows([]string{"original_url"}).AddRow(original)

	mock.ExpectQuery(`(?s)SELECT original_url`).
		WithArgs(shortID).
		WillReturnRows(rows)

	got, ok := repo.Get(context.Background(), shortID)
	require.True(t, ok)
	require.Equal(t, original, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Get_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "unknown"

	mock.ExpectQuery(`(?s)SELECT original_url`).
		WithArgs(shortID).
		WillReturnError(sql.ErrNoRows)

	got, ok := repo.Get(context.Background(), shortID)
	require.False(t, ok)
	require.Empty(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}
