package handler

import (
	"context"
	"encoding/json"
	"errors"
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

type fakeDB struct {
	err error
}

func (f *fakeDB) PingContext(ctx context.Context) error {
	return f.err
}

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

	db := &fakeDB{err: nil}

	return NewHTTPServer(cfg.BaseURL, svc, logger, db)
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
	_, ok := srv.shorter.GetOriginalURL(context.Background(), id)
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
	var testUser string = "test-user"

	db := &fakeDB{err: nil}

	srv := NewHTTPServer(cfg.BaseURL, svc, logger, db)

	shortID := "test123"
	original := "https://example.com"
	repo.Save(context.Background(), shortID, original, testUser)

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

// --- Тест OK на GET /ping ---
func TestHandlePing_OK(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()
	db := &fakeDB{err: nil}

	srv := NewHTTPServer(cfg.BaseURL, svc, logger, db)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHandlePing_DBError(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()
	db := &fakeDB{err: errors.New("db down")}

	srv := NewHTTPServer(cfg.BaseURL, svc, logger, db)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestHandleAPIPostBatch_OK(t *testing.T) {
	srv := setupTestServer()

	body := `[
		{"correlation_id": "1", "original_url": "https://practicum.yandex.ru/"},
		{"correlation_id": "2", "original_url": "https://example.com"}
	]`

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	// 1) Проверяем статус
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.StatusCode)
	}

	// 2) Проверяем Content-Type
	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	// 3) Читаем и разбираем JSON-ответ
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("cannot read body: %v", err)
	}

	var resp []model.BatchShortenResponseItem
	if err := json.Unmarshal(respBody, &resp); err != nil {
		t.Fatalf("cannot unmarshal response: %v", err)
	}

	// 4) Проверяем количество элементов
	if len(resp) != 2 {
		t.Fatalf("expected 2 items in response, got %d", len(resp))
	}

	// 5) Проверяем correlation_id и short_url
	if resp[0].CorrelationID != "1" {
		t.Errorf("expected correlation_id '1', got %s", resp[0].CorrelationID)
	}
	if !strings.HasPrefix(resp[0].ShortURL, "http://localhost:8080/") {
		t.Errorf("expected short_url to start with http://localhost:8080/, got %s", resp[0].ShortURL)
	}

	if resp[1].CorrelationID != "2" {
		t.Errorf("expected correlation_id '2', got %s", resp[1].CorrelationID)
	}
	if !strings.HasPrefix(resp[1].ShortURL, "http://localhost:8080/") {
		t.Errorf("expected short_url to start with http://localhost:8080/, got %s", resp[1].ShortURL)
	}

	// 6) Поверим, что первая ссылка реально сохранилась в сервисе
	firstShort := resp[0].ShortURL
	const base = "http://localhost:8080"
	if !strings.HasPrefix(firstShort, base+"/") {
		t.Fatalf("unexpected short url format: %s", firstShort)
	}
	id := strings.TrimPrefix(firstShort, base+"/")

	original, ok := srv.shorter.GetOriginalURL(context.Background(), id)
	if !ok {
		t.Fatalf("short url with id %s was not saved in repository", id)
	}
	if original != "https://practicum.yandex.ru/" {
		t.Errorf("expected original url %q, got %q", "https://practicum.yandex.ru/", original)
	}
}
