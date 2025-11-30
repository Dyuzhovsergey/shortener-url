// Package handler for working HTTP server
package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"

	"github.com/Dyuzhovsergey/shortener-url/internal/model"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

type DBPinger interface {
	PingContext(ctx context.Context) error
}

// HTTPServer — слой HTTP, знает про сервис, но не про репозиторий.
type HTTPServer struct {
	baseURL string
	shorter *service.ShorterService
	logger  *zap.Logger
	db      DBPinger
}

// NewHTTPServer — конструктор с внедрением зависимости (DI)
func NewHTTPServer(baseURL string, shorter *service.ShorterService, logger *zap.Logger, db DBPinger) *HTTPServer {
	return &HTTPServer{
		baseURL: baseURL,
		shorter: shorter,
		logger:  logger,
		db:      db,
	}
}

// Router — возвращает готовый http.Handler (ServeMux)
func (srv *HTTPServer) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.ZapLogger(srv.logger))
	r.Use(middleware.GzipMiddleware)

	r.Post("/", srv.handlePost)
	r.Get("/{id}", srv.handleGet)
	r.Post("/api/shorten", srv.handleAPIPost)
	r.Get("/ping", srv.handlePing)
	return r
}

// POST /
func (srv *HTTPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Bad POST request", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	shortURL, err := srv.shorter.CreateShortURL(r.Context(), string(body), srv.baseURL)
	if err != nil {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

// GET /{id}
func (srv *HTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad GET request", http.StatusBadRequest)
		return
	}

	originalURL, ok := srv.shorter.GetOriginalURL(r.Context(), id)
	if !ok {
		http.Error(w, "URL not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// POST /api/shorten
func (srv *HTTPServer) handleAPIPost(w http.ResponseWriter, r *http.Request) {

	var req model.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	shortURL, err := srv.shorter.CreateShortURL(r.Context(), req.URL, srv.baseURL)
	if err != nil {
		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}
	resp := model.ShortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.logger.Error("failed to write JSON response", zap.Error(err))
	}
}

func (srv *HTTPServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if srv.db == nil {
		// БД не настроена — считаем это 500
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	if err := srv.db.PingContext(r.Context()); err != nil {
		srv.logger.Error("database ping failed", zap.Error(err))
		http.Error(w, "database is not available", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
