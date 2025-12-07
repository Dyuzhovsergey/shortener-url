package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestPostgresRepository_Save_OK проверяет, что новый URL сохраняется без ошибок.
func TestPostgresRepository_Save_OK(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "abc123"
	original := "https://example.com"

	// 1. Сначала ожидаем SELECT по original_url — вернётся sql.ErrNoRows.
	mock.ExpectQuery(`SELECT short_id FROM short_urls WHERE original_url = \$1`).
		WithArgs(original).
		WillReturnError(sql.ErrNoRows)

	// 2. Затем ожидаем INSERT.
	mock.ExpectExec(`INSERT INTO short_urls`).
		WithArgs(shortID, original).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Save(context.Background(), shortID, original)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresRepository_Get_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("cannot create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "abc123"
	original := "https://example.com"

	rows := sqlmock.NewRows([]string{"original_url"}).
		AddRow(original)

	mock.ExpectQuery(`SELECT original_url FROM short_urls WHERE short_id = \$1;`).
		WithArgs(shortID).
		WillReturnRows(rows)

	got, ok := repo.Get(context.Background(), shortID)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	if got != original {
		t.Errorf("expected %q, got %q", original, got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPostgresRepository_Get_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("cannot create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "unknown"

	mock.ExpectQuery(`SELECT original_url FROM short_urls WHERE short_id = \$1;`).
		WithArgs(shortID).
		WillReturnError(sql.ErrNoRows)

	got, ok := repo.Get(context.Background(), shortID)
	if ok {
		t.Fatalf("expected ok=false for unknown id, got true with value %q", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// TestPostgresRepository_Save_Duplicate проверяет,
// что при повторной попытке сократить тот же URL возвращается ErrOriginalAlreadyExists.
func TestPostgresRepository_Save_Duplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewPostgresRepository(db)

	existingShortID := "old123"
	original := "https://example.com"

	// 1. SELECT по original_url — находим уже существующую запись.
	rows := sqlmock.NewRows([]string{"short_id"}).
		AddRow(existingShortID)

	mock.ExpectQuery(`SELECT short_id FROM short_urls WHERE original_url = \$1`).
		WithArgs(original).
		WillReturnRows(rows)

	err = repo.Save(context.Background(), "newID", original)
	require.Error(t, err)

	var dupErr *ErrOriginalAlreadyExists
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, existingShortID, dupErr.ShortID)

	require.NoError(t, mock.ExpectationsWereMet())
}
