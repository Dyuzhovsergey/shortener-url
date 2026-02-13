package repository

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
)

const testUser = "test-user"

func TestMemoryRepository_SaveAndGet(t *testing.T) {
	repo := NewMemoryRepository()

	shortID := "abc123"
	original := "https://example.com"

	if err := repo.Save(context.Background(), shortID, original, testUser); err != nil {
		t.Fatalf("unexpected error on Save: %v", err)
	}

	got, ok, err := repo.Get(context.Background(), shortID)
	if err != nil {
		t.Fatalf("unexpected error on Get: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true for existing key, got false")
	}
	if got != original {
		t.Errorf("expected %q, got %q", original, got)
	}
}

func TestMemoryRepository_GetNotFound(t *testing.T) {
	repo := NewMemoryRepository()

	got, ok, err := repo.Get(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for unknown id, got true with value %q", got)
	}
}

func TestMemoryRepository_Save_ConflictByOriginalURL(t *testing.T) {
	repo := NewMemoryRepository()

	original := "https://example.com/same"
	if err := repo.Save(context.Background(), "id1", original, testUser); err != nil {
		t.Fatalf("unexpected error on first Save: %v", err)
	}

	err := repo.Save(context.Background(), "id2", original, testUser)
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}

	var dup *ErrOriginalAlreadyExists
	if !errors.As(err, &dup) {
		t.Fatalf("expected ErrOriginalAlreadyExists, got %T: %v", err, err)
	}
	if dup.ShortID != "id1" {
		t.Fatalf("expected ShortID=id1 in dup error, got %q", dup.ShortID)
	}
}

func TestMemoryRepository_OverwriteSameShortID(t *testing.T) {
	repo := NewMemoryRepository()

	shortID := "abc123"
	url1 := "https://example.com/1"
	url2 := "https://example.com/2"

	if err := repo.Save(context.Background(), shortID, url1, testUser); err != nil {
		t.Fatalf("unexpected error on first Save: %v", err)
	}
	if err := repo.Save(context.Background(), shortID, url2, testUser); err != nil {
		t.Fatalf("unexpected error on second Save: %v", err)
	}

	got, ok, err := repo.Get(context.Background(), shortID)
	if err != nil {
		t.Fatalf("unexpected error on Get: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true for existing key, got false")
	}
	if got != url2 {
		t.Errorf("expected %q after overwrite, got %q", url2, got)
	}
}

func TestMemoryRepository_DeleteUserURLs_OwnerOnly(t *testing.T) {
	repo := NewMemoryRepository()

	owner := "owner"
	other := "other"

	idOwner := "o1"
	idOther := "x1"

	if err := repo.Save(context.Background(), idOwner, "https://example.com/owner", owner); err != nil {
		t.Fatalf("save owner url: %v", err)
	}
	if err := repo.Save(context.Background(), idOther, "https://example.com/other", other); err != nil {
		t.Fatalf("save other url: %v", err)
	}

	//  Удаляем чужую ссылку
	if err := repo.DeleteUserURLs(context.Background(), owner, []string{idOther}); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}

	_, ok, err := repo.Get(context.Background(), idOther)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if !ok {
		t.Fatalf("expected other url still exists")
	}

	if err := repo.DeleteUserURLs(context.Background(), owner, []string{idOwner}); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}

	_, ok, err = repo.Get(context.Background(), idOwner)
	if !ok {
		t.Fatalf("expected ok=true for deleted record (exists but deleted), got ok=false")
	}
	if !errors.Is(err, ErrDeleted) {
		t.Fatalf("expected ErrDeleted, got %v", err)
	}
}

func TestMemoryRepository_GetUserURLs_ExcludesDeleted(t *testing.T) {
	repo := NewMemoryRepository()

	user := "u1"
	id1 := "a1"
	id2 := "a2"

	if err := repo.Save(context.Background(), id1, "https://example.com/1", user); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.Save(context.Background(), id2, "https://example.com/2", user); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := repo.DeleteUserURLs(context.Background(), user, []string{id1}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	urls, err := repo.GetUserURLs(context.Background(), user)
	if err != nil {
		t.Fatalf("GetUserURLs: %v", err)
	}

	if len(urls) != 1 {
		t.Fatalf("expected 1 url, got %d: %+v", len(urls), urls)
	}
	if urls[0].ShortID != id2 {
		t.Fatalf("expected remaining ShortID=%q, got %q", id2, urls[0].ShortID)
	}
	if urls[0].OriginalURL != "https://example.com/2" {
		t.Fatalf("expected remaining OriginalURL=%q, got %q", "https://example.com/2", urls[0].OriginalURL)
	}
}

func TestMemoryRepository_ConcurrentAccess(t *testing.T) {
	repo := NewMemoryRepository()

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := "id-" + strconv.Itoa(g*perGoroutine+i)
				_ = repo.Save(context.Background(), id, "https://example.com/"+id, testUser)
			}
		}(g)
	}
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := "id-" + strconv.Itoa(g*perGoroutine+i)
				_, _, _ = repo.Get(context.Background(), id)
			}
		}(g)
	}

	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			ids := make([]string, 0, perGoroutine/10)
			for i := 0; i < perGoroutine/10; i++ {
				id := "id-" + strconv.Itoa(g*perGoroutine+i)
				ids = append(ids, id)
			}
			_ = repo.DeleteUserURLs(context.Background(), testUser, ids)
		}(g)
	}

	wg.Wait()
}
