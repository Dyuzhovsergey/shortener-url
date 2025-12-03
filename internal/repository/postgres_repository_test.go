package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresRepository_Save_OK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("cannot create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresRepository(db)

	shortID := "abc123"
	original := "https://example.com"

	// ожидаем, что при Save будет вызван INSERT ... ON CONFLICT
	mock.ExpectExec(`INSERT INTO short_urls`).
		WithArgs(shortID, original).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Save(context.Background(), shortID, original)
	if err != nil {
		t.Fatalf("unexpected error from Save: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
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
