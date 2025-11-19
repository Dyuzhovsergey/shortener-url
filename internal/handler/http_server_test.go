package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/model"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

// makeTestConfig — возвращает тестовую конфигурацию.
func makeTestConfig() *config.ShortenerConfig {
	return &config.ShortenerConfig{
		CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID: 8,
		BaseURL:  "http://localhost:8080",
		RunAddr:  ":8080",
	}
}

// Создаёт "тестовый сервер" с in-memory репозиторием
func setupTestServer() *HTTPServer {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()

	return NewHTTPServer(cfg.BaseURL, svc, logger)
}

// Тест на POST / — создание короткого URL
func TestHandlePost(t *testing.T) {
	srv := setupTestServer()

	reqBody := "https://practicum.yandex.ru/"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "text/plain")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	bodyStr := string(body)

	if res.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, res.StatusCode)
	}
	if res.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("expected header Content-Type 'text/plain', got %s", res.Header.Get("Content-Type"))
	}
	if !strings.HasPrefix(bodyStr, "http://localhost:8080/") {
		t.Errorf("expected short URL prefix http://localhost:8080/, got %s", bodyStr)
	}

	// Проверяем, что URL действительно сохранился
	id := strings.TrimPrefix(bodyStr, "http://localhost:8080/")
	_, ok := srv.shorter.GetOriginalURL(id)
	if !ok {
		t.Errorf("short URL with ID %s was not saved", id)
	}
}

// --- Тест на GET /{id} ---
// Проверяет редирект на оригинальный URL
func TestHandleGet(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()

	srv := NewHTTPServer(cfg.BaseURL, svc, logger)

	shortID := "test123"
	original := "https://example.com"
	repo.Save(shortID, original)

	req := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, res.StatusCode)
	}
	location := res.Header.Get("Location")
	if location != original {
		t.Errorf("expected Location %s, got %s", original, location)
	}
}

func TestHandleAPIPost_OK(t *testing.T) {
	srv := setupTestServer()

	body := `{"url":"https://practicum.yandex.ru/"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.StatusCode)
	}

	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("cannot read body: %v", err)
	}

	var resp model.ShortenResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		t.Fatalf("cannot unmarshal response: %v", err)
	}

	if !strings.HasPrefix(resp.Result, "http://localhost:8080/") {
		t.Errorf("expected result prefix http://localhost:8080/, got %s", resp.Result)
	}
}
