package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

// Создаёт "тестовый сервер" с in-memory репозиторием
func setupTestServer() *HTTPServer {
	repo := repository.NewMemoryRepository()
	svc := service.NewShorterService(repo)
	return NewHTTPServer("http://localhost:8080", svc)
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

	if res.StatusCode != http.StatusCreated {
		t.Errorf("ожидался статус %d, получили %d", http.StatusCreated, res.StatusCode)
	}
	if res.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("ожидался заголовок 'text/plain', получили %s", res.Header.Get("Content-Type"))
	}
	if !strings.HasPrefix(string(body), "http://localhost:8080/") {
		t.Errorf("ожидался короткий URL с префиксом http://localhost:8080/, получили %s", string(body))
	}
}

// Тест на GET /{id} — редирект на оригинальный URL
func TestHandleGet(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := service.NewShorterService(repo)
	srv := NewHTTPServer("http://localhost:8080", svc)

	// добавляем тестовые данные в репозиторий
	shortID := "test123"
	original := "https://example.com"
	repo.Save(shortID, original)

	req := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("ожидался статус %d, получили %d", http.StatusTemporaryRedirect, res.StatusCode)
	}

	location := res.Header.Get("Location")
	if location != original {
		t.Errorf("ожидался Location %s, получили %s", original, location)
	}
}
