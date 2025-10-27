package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

func setupTestRepo() {
	repo = repository.NewMemoryRepository()
}

// Тест на POST-запрос — проверяем, что сервер создаёт короткий URL
func TestHandlePost(t *testing.T) {
	setupTestRepo()
	// создаём поддельный HTTP-запрос
	body := "https://practicum.yandex.ru/"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	// создаём "виртуальный" ответ
	rec := httptest.NewRecorder()

	// вызываем обработчик
	handlePost(rec, req)

	// получаем результат
	res := rec.Result()
	defer res.Body.Close()

	// читаем тело
	respBody, _ := io.ReadAll(res.Body)

	// проверки
	if res.StatusCode != http.StatusCreated {
		t.Errorf("ожидался статус %d, получили %d", http.StatusCreated, res.StatusCode)
	}
	if !strings.HasPrefix(string(respBody), baseURL+"/") {
		t.Errorf("ожидался короткий URL с префиксом %s/, получили %s", baseURL, string(respBody))
	}
}

// Тест на GET-запрос — проверяем, что сервер делает редирект
func TestHandleGet(t *testing.T) {
	setupTestRepo()
	// добавляем в хранилище тестовые данные
	testID := "test123"
	testURL := "https://example.com"

	// сохраняем тестовые данные в репозиторий
	if err := repo.Save(testID, testURL); err != nil {
		t.Fatalf("ошибка при добавлении тестовых данных: %v", err)
	}

	// создаём поддельный HTTP-запрос
	req := httptest.NewRequest(http.MethodGet, "/"+testID, nil)
	rec := httptest.NewRecorder()

	// вызываем обработчик
	handleGet(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	// проверки
	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("ожидался статус %d, получили %d", http.StatusTemporaryRedirect, res.StatusCode)
	}

	location := res.Header.Get("Location")
	if location != testURL {
		t.Errorf("ожидался Location %s, получили %s", testURL, location)
	}
}
