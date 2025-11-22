package service

import (
	"testing"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

// makeTestConfig — создаёт конфигурацию для тестов.
func makeTestConfig() *config.ShortenerConfig {
	return &config.ShortenerConfig{
		CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID: 8,
		BaseURL:  "http://localhost:8080",
		RunAddr:  ":8080",
	}
}

// --- Тест на успешное создание короткой ссылки ---
func TestCreateShortURL_Valid(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	original := "https://practicum.yandex.ru/"
	baseURL := cfg.BaseURL

	shortURL, err := svc.CreateShortURL(original, baseURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shortURL == "" {
		t.Fatal("short URL is empty")
	}

	if len(shortURL) <= len(baseURL) {
		t.Errorf("short URL too short: %s", shortURL)
	}

	// Проверим, что ссылка сохранилась в репозитории
	id := shortURL[len(baseURL)+1:] // +1 за '/'
	got, ok := svc.GetOriginalURL(id)
	if !ok {
		t.Fatalf("shortID %s not found in repository", id)
	}
	if got != original {
		t.Errorf("expected %s, got %s", original, got)
	}
}

// --- Тест на валидацию невалидных URL ---
func TestCreateShortURL_Invalid(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	tests := []string{
		"",                  // пустая строка
		"htt://example.com", // опечатка в схеме
		"http://",           // нет хоста
		"example.com",       // без схемы
		"ftp://example.com", // неподдерживаемая схема, если в сервисе запрещены схемы кроме http/https
	}

	for _, input := range tests {
		_, err := svc.CreateShortURL(input, cfg.BaseURL)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", input)
		}
	}
}

// --- Тест на уникальность ID ---
// При большом количестве генераций не должно быть дублей
func TestCreateShortURL_Uniqueness(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	original := "https://example.com"
	baseURL := cfg.BaseURL

	generated := make(map[string]bool)
	const n = 1000

	for i := 0; i < n; i++ {
		shortURL, err := svc.CreateShortURL(original, baseURL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		id := shortURL[len(baseURL)+1:]
		if generated[id] {
			t.Fatalf("duplicate shortID generated: %s", id)
		}
		generated[id] = true
	}
}

// --- Тест на GetOriginalURL ---
func TestGetOriginalURL(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	shortID := "abc123"
	original := "https://go.dev"

	// сохраняем напрямую в репозиторий
	if err := repo.Save(shortID, original); err != nil {
		t.Fatalf("cannot save to repo: %v", err)
	}

	got, ok := svc.GetOriginalURL(shortID)
	if !ok {
		t.Fatalf("expected to find shortID %s", shortID)
	}
	if got != original {
		t.Errorf("expected %s, got %s", original, got)
	}

	// неизвестный ID
	if _, ok := svc.GetOriginalURL("unknownID"); ok {
		t.Errorf("expected false for unknownID")
	}
}
