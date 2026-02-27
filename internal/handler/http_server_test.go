package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/model"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

type fakeDB struct{ err error }

func (f *fakeDB) PingContext(ctx context.Context) error { return f.err }

// makeTestConfig — возвращает тестовую конфигурацию.
func makeTestConfig() *config.ShortenerConfig {
	return &config.ShortenerConfig{
		CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID: 8,
		BaseURL:  "http://localhost:8080",
		RunAddr:  ":8080",
	}
}

func setupTestServer() *HTTPServer {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()
	db := &fakeDB{err: nil}

	auditor := audit.NewPublisher()
	return NewHTTPServer(cfg.BaseURL, svc, logger, db, auditor)
}

func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// helper: достаём auth cookie, которую выставил сервер (middleware).
func getAuthCookieFromResponse(t *testing.T, res *http.Response) *http.Cookie {
	t.Helper()

	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected at least 1 Set-Cookie, got none")
	}

	// если имя у тебя user_id — можно отфильтровать, но безопаснее искать.
	for _, c := range cookies {
		if c.Name == "user_id" {
			return c
		}
	}
	// fallback: первая
	return cookies[0]
}

// --- POST / — создание короткого URL ---
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
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected Content-Type text/plain, got %s", ct)
	}
	if !strings.HasPrefix(bodyStr, "http://localhost:8080/") {
		t.Errorf("expected short URL prefix http://localhost:8080/, got %s", bodyStr)
	}
}

// --- GET /{id} — редирект ---
func TestHandleGet_Redirect(t *testing.T) {
	srv := setupTestServer()

	// 1) создаём ссылку
	orig := "https://example.com"
	createReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(orig))
	createReq.Header.Set("Content-Type", "text/plain")
	createRec := httptest.NewRecorder()

	srv.Router().ServeHTTP(createRec, createReq)
	createRes := createRec.Result()
	defer createRes.Body.Close()

	if createRes.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(createRes.Body)
		t.Fatalf("expected 201, got %d, body=%q", createRes.StatusCode, string(b))
	}

	shortBytes, _ := io.ReadAll(createRes.Body)
	shortURL := strings.TrimSpace(string(shortBytes))
	shortID := strings.TrimPrefix(shortURL, "http://localhost:8080/")

	// 2) GET
	getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	getRec := httptest.NewRecorder()

	srv.Router().ServeHTTP(getRec, getReq)
	getRes := getRec.Result()
	defer getRes.Body.Close()

	if getRes.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected %d, got %d", http.StatusTemporaryRedirect, getRes.StatusCode)
	}
	if loc := getRes.Header.Get("Location"); loc != orig {
		t.Fatalf("expected Location %q, got %q", orig, loc)
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
	requireNoErr(t, err)

	var resp model.ShortenResponse
	requireNoErr(t, json.Unmarshal(respBody, &resp))

	if !strings.HasPrefix(resp.Result, "http://localhost:8080/") {
		t.Errorf("expected result prefix http://localhost:8080/, got %s", resp.Result)
	}
}

// --- GET /ping OK ---
func TestHandlePing_OK(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()
	db := &fakeDB{err: nil}
	auditor := audit.NewPublisher()

	srv := NewHTTPServer(cfg.BaseURL, svc, logger, db, auditor)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHandlePing_DBError(t *testing.T) {
	repo := repository.NewMemoryRepository()
	cfg := makeTestConfig()
	svc := service.NewShorterService(repo, cfg)

	logger := zap.NewNop()
	db := &fakeDB{err: errors.New("db down")}
	auditor := audit.NewPublisher()

	srv := NewHTTPServer(cfg.BaseURL, svc, logger, db, auditor)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleAPIPostBatch_OK(t *testing.T) {
	srv := setupTestServer()

	body := `[
		{"correlation_id":"1","original_url":"https://practicum.yandex.ru/"},
		{"correlation_id":"2","original_url":"https://example.com"}
	]`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}

	respBody, err := io.ReadAll(res.Body)
	requireNoErr(t, err)

	var resp []model.BatchShortenResponseItem
	requireNoErr(t, json.Unmarshal(respBody, &resp))

	if len(resp) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp))
	}
	if resp[0].CorrelationID != "1" {
		t.Errorf("expected correlation_id=1, got %s", resp[0].CorrelationID)
	}
	if !strings.HasPrefix(resp[0].ShortURL, "http://localhost:8080/") {
		t.Errorf("unexpected short_url %s", resp[0].ShortURL)
	}
}

// --- DELETE /api/user/urls ---
// 1) создаём url -> получаем cookie владельца
// 2) DELETE с этой cookie -> 202
// 3) ждём чуть-чуть (т.к. async) и GET -> 410
func TestHandleDeleteUserURLs_AcceptsAndEventuallyGone(t *testing.T) {
	srv := setupTestServer()

	// 1) создаём ссылку
	orig := "https://example.com/to-delete"
	createReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(orig))
	createReq.Header.Set("Content-Type", "text/plain")
	createRec := httptest.NewRecorder()

	srv.Router().ServeHTTP(createRec, createReq)
	createRes := createRec.Result()
	defer createRes.Body.Close()

	if createRes.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(createRes.Body)
		t.Fatalf("expected 201, got %d, body=%q", createRes.StatusCode, string(b))
	}

	authCookie := getAuthCookieFromResponse(t, createRes)

	shortBytes, _ := io.ReadAll(createRes.Body)
	shortURL := strings.TrimSpace(string(shortBytes))
	shortID := strings.TrimPrefix(shortURL, "http://localhost:8080/")

	// 2) DELETE
	delBody, _ := json.Marshal([]string{shortID})
	delReq := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(string(delBody)))
	delReq.Header.Set("Content-Type", "application/json")
	delReq.AddCookie(authCookie)

	delRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(delRec, delReq)

	delRes := delRec.Result()
	defer delRes.Body.Close()

	if delRes.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(delRes.Body)
		t.Fatalf("expected 202, got %d, body=%q", delRes.StatusCode, string(b))
	}

	// 3) async: даём время воркеру (в тестах можно маленький polling)
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
		getRec := httptest.NewRecorder()
		srv.Router().ServeHTTP(getRec, getReq)

		getRes := getRec.Result()
		func() {
			defer getRes.Body.Close()

			if getRes.StatusCode == http.StatusGone {
				return
			}

			if time.Now().After(deadline) {
				b, _ := io.ReadAll(getRes.Body)
				t.Fatalf(
					"expected eventually 410 Gone, last=%d body=%q",
					getRes.StatusCode,
					string(b),
				)
			}
		}()

		if getRes.StatusCode == http.StatusGone {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}
}

type testAuditObserver struct {
	mu     sync.Mutex
	events []audit.Event
}

func (o *testAuditObserver) Observe(_ context.Context, event audit.Event) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
	return nil
}

func (o *testAuditObserver) Last() (audit.Event, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.events) == 0 {
		return audit.Event{}, false
	}
	return o.events[len(o.events)-1], true
}

func TestAudit_POSTRoot_Shorten(t *testing.T) {
	srv := setupTestServer()
	router := srv.Router()

	obs := &testAuditObserver{}
	auditor := audit.NewPublisher()
	auditor.Add(obs)
	// ВАЖНО: сервер в тесте должен быть создан с этим auditor.
	// Если в setupTestServer auditor создаётся внутри — поменяй setupTestServer так,
	// чтобы он принимал auditor параметром или чтобы srv.audit = auditor.
	srv.audit = auditor

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com/path"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, w.Code)
	}

	ev, ok := obs.Last()
	if !ok {
		t.Fatalf("expected audit event")
	}
	if ev.Action != "shorten" {
		t.Fatalf("expected action shorten, got %q", ev.Action)
	}
	if ev.URL != "https://example.com/path" {
		t.Fatalf("expected url %q, got %q", "https://example.com/path", ev.URL)
	}
	if ev.TS <= 0 {
		t.Fatalf("expected ts > 0, got %d", ev.TS)
	}
	if ev.UserID == "" {
		t.Fatalf("expected non-empty user_id")
	}
}

func TestAudit_POSTAPIShorten_Shorten(t *testing.T) {
	srv := setupTestServer()
	router := srv.Router()

	obs := &testAuditObserver{}
	auditor := audit.NewPublisher()
	auditor.Add(obs)
	srv.audit = auditor

	body := `{"url":"https://example.com/api"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, w.Code)
	}

	ev, ok := obs.Last()
	if !ok {
		t.Fatalf("expected audit event")
	}
	if ev.Action != "shorten" {
		t.Fatalf("expected action shorten, got %q", ev.Action)
	}
	if ev.URL != "https://example.com/api" {
		t.Fatalf("expected url %q, got %q", "https://example.com/api", ev.URL)
	}
	if ev.UserID == "" {
		t.Fatalf("expected non-empty user_id")
	}
}

func TestAudit_GETFollow_Follow(t *testing.T) {
	srv := setupTestServer()
	router := srv.Router()

	obs := &testAuditObserver{}
	auditor := audit.NewPublisher()
	auditor.Add(obs)
	srv.audit = auditor

	original := "https://example.com/follow"
	shortURL, err := srv.shorter.CreateShortURL(context.Background(), original, srv.baseURL)
	if err != nil {
		t.Fatalf("CreateShortURL: %v", err)
	}

	u, err := url.Parse(shortURL)
	if err != nil {
		t.Fatalf("parse short url: %v", err)
	}
	id := strings.TrimPrefix(u.Path, "/")

	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}

	ev, ok := obs.Last()
	if !ok {
		t.Fatalf("expected audit event")
	}
	if ev.Action != "follow" {
		t.Fatalf("expected action follow, got %q", ev.Action)
	}
	if ev.URL != original {
		t.Fatalf("expected url %q, got %q", original, ev.URL)
	}
	if ev.UserID == "" {
		t.Fatalf("expected non-empty user_id")
	}
}
