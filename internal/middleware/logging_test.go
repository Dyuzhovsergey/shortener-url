package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestZapLogger_Passthrough_WriteHeaderAndBody(t *testing.T) {
	logger := zap.NewNop()

	// Хэндлер специально вызывает и WriteHeader, и Write,
	// чтобы прокрыть обёртку ResponseWriter в middleware.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	mw := ZapLogger(logger)
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("ожидали статус %d, получили %d", http.StatusCreated, rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("ожидали тело %q, получили %q", "ok", rr.Body.String())
	}
}

func TestZapLogger_DefaultStatus200_WhenNoWriteHeader(t *testing.T) {
	logger := zap.NewNop()

	// Хэндлер НЕ вызывает WriteHeader → должен остаться 200.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	mw := ZapLogger(logger)
	handler := mw(next)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ожидали статус %d, получили %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "hello" {
		t.Fatalf("ожидали тело %q, получили %q", "hello", rr.Body.String())
	}
}
