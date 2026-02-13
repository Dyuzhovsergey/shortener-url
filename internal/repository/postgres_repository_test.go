package repository

import (
	"context"
	"database/sql"
	"errors"
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

	// ExpectQuery, потому что Save использует QueryRowContext + RETURNING.
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

	// возвращаем existingShortID, отличный от newShortID -> repo.Save 
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

	// Get читает original_url + is_deleted, поэтому 2 колонки.
	rows := sqlmock.NewRows([]string{"original_url", "is_deleted"}).
		AddRow(original, false)

	mock.ExpectQuery(`(?s)SELECT original_url,\s*is_deleted`).
		WithArgs(shortID).
		WillReturnRows(rows)

	got, ok, err := repo.Get(context.Background(), shortID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, original, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Get_Deleted(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "del123"

	// deleted=true => ok=true, err=ErrDeleted, original можно вернуть пустым.
	rows := sqlmock.NewRows([]string{"original_url", "is_deleted"}).
		AddRow("https://example.com/deleted", true)

	mock.ExpectQuery(`(?s)SELECT original_url,\s*is_deleted`).
		WithArgs(shortID).
		WillReturnRows(rows)

	_, ok, err := repo.Get(context.Background(), shortID)
	require.True(t, ok)
	require.True(t, errors.Is(err, ErrDeleted), "expected ErrDeleted, got: %v", err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Get_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "unknown"

	// Get делает SELECT original_url, is_deleted
	mock.ExpectQuery(`(?s)SELECT original_url,\s*is_deleted`).
		WithArgs(shortID).
		WillReturnError(sql.ErrNoRows)

	got, ok, err := repo.Get(context.Background(), shortID)
	require.NoError(t, err) // ErrNoRows -> err=nil
	require.False(t, ok)
	require.Empty(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}
