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

	// Ожидаем один запрос INSERT ... ON CONFLICT ... RETURNING short_id
	// и возвращаем тот же shortID, который мы вставляем.
	rows := sqlmock.NewRows([]string{"short_id"}).
		AddRow(shortID)

	mock.ExpectQuery(`INSERT INTO short_urls`).
		WithArgs(shortID, original).
		WillReturnRows(rows)

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
	newShortID := "new456"

	// INSERT ... ON CONFLICT ... RETURNING short_id
	// возвращает существующий short_id (existingShortID),
	// хотя мы пытаемся вставить newShortID.
	rows := sqlmock.NewRows([]string{"short_id"}).
		AddRow(existingShortID)

	mock.ExpectQuery(`INSERT INTO short_urls`).
		WithArgs(newShortID, original).
		WillReturnRows(rows)

	err = repo.Save(context.Background(), newShortID, original)
	require.Error(t, err)

	var dupErr *ErrOriginalAlreadyExists
	require.ErrorAs(t, err, &dupErr)
	require.Equal(t, existingShortID, dupErr.ShortID)

	require.NoError(t, mock.ExpectationsWereMet())
}
