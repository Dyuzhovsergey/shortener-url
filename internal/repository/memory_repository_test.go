package repository

import (
	"strconv"
	"sync"
	"testing"
)

// TestMemoryRepository_SaveAndGet проверяет, что сохранённое значение можно получить.
func TestMemoryRepository_SaveAndGet(t *testing.T) {
	repo := NewMemoryRepository()

	shortID := "abc123"
	original := "https://example.com"

	if err := repo.Save(shortID, original); err != nil {
		t.Fatalf("unexpected error on Save: %v", err)
	}

	got, ok := repo.Get(shortID)
	if !ok {
		t.Fatalf("expected ok=true for existing key, got false")
	}
	if got != original {
		t.Errorf("expected %q, got %q", original, got)
	}
}

// TestMemoryRepository_GetNotFound проверяет поведение при отсутствии ключа.
func TestMemoryRepository_GetNotFound(t *testing.T) {
	repo := NewMemoryRepository()

	_, ok := repo.Get("unknown")
	if ok {
		t.Errorf("expected ok=false for unknown key, got true")
	}
}

// TestMemoryRepository_Overwrite проверяет, что повторный Save перезаписывает значение.
func TestMemoryRepository_Overwrite(t *testing.T) {
	repo := NewMemoryRepository()

	shortID := "abc123"
	url1 := "https://example.com/1"
	url2 := "https://example.com/2"

	if err := repo.Save(shortID, url1); err != nil {
		t.Fatalf("unexpected error on first Save: %v", err)
	}
	if err := repo.Save(shortID, url2); err != nil {
		t.Fatalf("unexpected error on second Save: %v", err)
	}

	got, ok := repo.Get(shortID)
	if !ok {
		t.Fatalf("expected ok=true for existing key, got false")
	}
	if got != url2 {
		t.Errorf("expected %q after overwrite, got %q", url2, got)
	}
}

// TestMemoryRepository_ConcurrentAccess проверяет, что нет паник при конкурентном доступе.
func TestMemoryRepository_ConcurrentAccess(t *testing.T) {
	repo := NewMemoryRepository()

	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// параллельные записывающие горутины
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := "id-" + strconv.Itoa(g*perGoroutine+i)
				_ = repo.Save(id, "https://example.com/"+id)
			}
		}(g)
	}

	// параллельные читающие горутины
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := "id-" + strconv.Itoa(g*perGoroutine+i)
				_, _ = repo.Get(id)
			}
		}(g)
	}

	wg.Wait()
	// если здесь нет паники и гонок при запуске с -race, всё ок
}
