package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

const testUser = "test-user"

// makeTestConfig — конфигурация для тестов.
func makeTestConfig() *config.ShortenerConfig {
	return &config.ShortenerConfig{
		CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID: 8,
		BaseURL:  "http://localhost:8080",
		RunAddr:  ":8080",
	}
}

// makeCtxWithUserID прогоняет запрос через AuthMiddleware,
// чтобы получить контекст, в котором лежит user_id (как в реальном сервере).
func makeCtxWithUserID(t *testing.T) (context.Context, string) {
	t.Helper()

	var gotCtx context.Context

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr := httptest.NewRecorder()

	middleware.AuthMiddleware(next).ServeHTTP(rr, req)

	if gotCtx == nil {
		t.Fatalf("не удалось получить контекст из AuthMiddleware")
	}

	uid, ok := middleware.UserIDFromContext(gotCtx)
	if !ok || uid == "" {
		t.Fatalf("ожидали user_id в контексте")
	}

	return gotCtx, uid
}

// --- Тест на успешное создание короткой ссылки ---
func TestCreateShortURL_Valid(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	original := "https://practicum.yandex.ru/"
	baseURL := cfg.BaseURL

	// кладём userID в контекст, чтобы Save получил userID
	ctx, _ := makeCtxWithUserID(t)

	shortURL, err := svc.CreateShortURL(ctx, original, baseURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shortURL == "" {
		t.Fatal("short URL is empty")
	}

	if len(shortURL) <= len(baseURL) {
		t.Errorf("short URL too short: %s", shortURL)
	}

	// Проверяем, что ссылка сохранилась в репозитории
	id := shortURL[len(baseURL)+1:]
	got, ok, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected repo.Get error: %v", err)
	}
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
		"ftp://example.com", // неподдерживаемая схема
	}

	for _, input := range tests {
		_, err := svc.CreateShortURL(context.Background(), input, cfg.BaseURL)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", input)
		}
	}
}

// --- Тест на уникальность ID ---
func TestCreateShortURL_Uniqueness(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	baseURL := cfg.BaseURL

	generated := make(map[string]bool)
	const n = 1000

	for i := 0; i < n; i++ {
		original := fmt.Sprintf("https://example.com/%d", i)

		shortURL, err := svc.CreateShortURL(context.Background(), original, baseURL)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
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

	if err := repo.Save(context.Background(), shortID, original, testUser); err != nil {
		t.Fatalf("cannot save to repo: %v", err)
	}
	got, ok, err := svc.GetOriginalURL(context.Background(), shortID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected to find shortID %s", shortID)
	}
	if got != original {
		t.Errorf("expected %s, got %s", original, got)
	}

	// неизвестный ID
	_, ok, err = svc.GetOriginalURL(context.Background(), "unknownID")
	if err != nil {
		t.Fatalf("unexpected error for unknownID: %v", err)
	}
	if ok {
		t.Errorf("expected false for unknownID")
	}
}

func TestGetUserURLs_ReturnsUserLinks(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	ctx, _ := makeCtxWithUserID(t)

	original := "https://example.com/user/urls"
	_, err := svc.CreateShortURL(ctx, original, cfg.BaseURL)
	if err != nil {
		t.Fatalf("CreateShortURL error: %v", err)
	}

	list, err := svc.GetUserURLs(ctx)
	if err != nil {
		t.Fatalf("GetUserURLs error: %v", err)
	}

	// Проверяем через JSON, чтобы не зависеть от конкретных полей структуры.
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var out []map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(out) == 0 {
		t.Fatalf("ожидали непустой список ссылок пользователя")
	}

	found := false
	for _, m := range out {
		orig := m["original_url"]
		if orig == "" {
			orig = m["url"]
		}
		if orig == original {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("не нашли original_url=%q в списке: %s", original, string(b))
	}
}

func TestCreateShortURLBatch_ReturnsCorrelationIDs(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	ctx, _ := makeCtxWithUserID(t)

	var items []BatchItem
	err := json.Unmarshal([]byte(`[
		{"correlation_id":"c1","original_url":"https://example.com/1"},
		{"correlation_id":"c2","original_url":"https://example.com/2"}
	]`), &items)
	if err != nil {
		t.Fatalf("unmarshal batch items: %v", err)
	}

	resp, err := svc.CreateShortURLBatch(ctx, cfg.BaseURL, items)
	if err != nil {
		t.Fatalf("CreateShortURLBatch error: %v", err)
	}

	// Проверяем через JSON
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var out []map[string]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("ожидали 2 элемента, получили %d", len(out))
	}

	if out[0]["correlation_id"] != "c1" || out[1]["correlation_id"] != "c2" {
		t.Fatalf("correlation_id не совпали: %v", out)
	}

	getShort := func(m map[string]string) string {
		if v := m["short_url"]; v != "" {
			return v
		}
		if v := m["result"]; v != "" {
			return v
		}
		return ""
	}

	if s := getShort(out[0]); !strings.HasPrefix(s, cfg.BaseURL) {
		t.Fatalf("ожидали short_url с префиксом %q, получили %q", cfg.BaseURL, s)
	}
	if s := getShort(out[1]); !strings.HasPrefix(s, cfg.BaseURL) {
		t.Fatalf("ожидали short_url с префиксом %q, получили %q", cfg.BaseURL, s)
	}
}

func TestDeleteUserURLsAsync_LeadsToDeleted(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := NewShorterService(repo, cfg)

	ctx, _ := makeCtxWithUserID(t)

	original := "https://example.com/to/delete"
	shortURL, err := svc.CreateShortURL(ctx, original, cfg.BaseURL)
	if err != nil {
		t.Fatalf("CreateShortURL error: %v", err)
	}

	u, err := url.Parse(shortURL)
	if err != nil {
		t.Fatalf("parse short url error: %v", err)
	}
	id := strings.TrimPrefix(u.Path, "/")
	if id == "" {
		t.Fatalf("не удалось извлечь id из %q", shortURL)
	}

	if err := svc.DeleteUserURLsAsync(ctx, []string{id}); err != nil {
		t.Fatalf("DeleteUserURLsAsync error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, ok, err := svc.GetOriginalURL(ctx, id)
		if errors.Is(err, repository.ErrDeleted) || !ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("не дождались удаления id=%q", id)
}
