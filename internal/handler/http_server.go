package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/model"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

// DBPinger описывает минимальный интерфейс для проверки доступности базы данных.
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// HTTPServer — слой HTTP, знает про сервис, но не про репозиторий.
type HTTPServer struct {
	baseURL       string
	trustedSubnet *net.IPNet
	shorter       *service.ShorterService
	logger        *zap.Logger
	db            DBPinger
	audit         *audit.Publisher
}

// NewHTTPServer — конструктор с внедрением зависимости (DI).
func NewHTTPServer(baseURL string, trustedSubnet string, shorter *service.ShorterService, logger *zap.Logger, db DBPinger, auditor *audit.Publisher) *HTTPServer {
	var subnet *net.IPNet
	if strings.TrimSpace(trustedSubnet) != "" {
		_, parsedSubnet, err := net.ParseCIDR(strings.TrimSpace(trustedSubnet))
		if err == nil {
			subnet = parsedSubnet
		}
	}

	return &HTTPServer{
		baseURL:       baseURL,
		trustedSubnet: subnet,
		shorter:       shorter,
		logger:        logger,
		db:            db,
		audit:         auditor,
	}
}

// Router собирает роутер и возвращает готовый http.Handler.
func (srv *HTTPServer) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.ZapLogger(srv.logger))
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.AuthMiddleware)

	// pprof только в отладочных запусках.  PPROF=1 ./shortener
	if os.Getenv("PPROF") == "1" {
		mountPprof(r)
	}

	r.Post("/", srv.handlePost)
	r.Post("/api/shorten", srv.handleAPIPost)
	r.Get("/ping", srv.handlePing)
	r.With(middleware.TrustedSubnetMiddleware(srv.trustedSubnet)).
		Get("/api/internal/stats", srv.handleStats)
	r.Post("/api/shorten/batch", srv.handleAPIPostBatch)
	r.Delete("/api/user/urls", srv.handleUserURLsDelete)
	r.Get("/api/user/urls", srv.handleUserURLs)
	r.Get("/{id}", srv.handleGet)
	return r
}

// GET /{id}
func (srv *HTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad GET request", http.StatusBadRequest)
		return
	}

	originalURL, ok, err := srv.shorter.GetOriginalURL(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrDeleted) {
			w.WriteHeader(http.StatusGone) // 410
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !ok {
		http.Error(w, "URL not found", http.StatusBadRequest)
		return
	}

	srv.publishAudit(r.Context(), "follow", originalURL)

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// GET /api/internal/stats
func (srv *HTTPServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := srv.shorter.GetStats(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := model.StatsResponse{URLs: stats.URLs, Users: stats.Users}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.logger.Error("failed to write JSON stats response", zap.Error(err))
		return
	}
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

	originalURL := strings.TrimSpace(string(body))
	shortURL, err := srv.shorter.CreateShortURL(r.Context(), originalURL, srv.baseURL)
	if err != nil {
		if shortURL != "" {
			srv.publishAudit(r.Context(), "shorten", originalURL)

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(shortURL))
			return
		}

		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	srv.publishAudit(r.Context(), "shorten", originalURL)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(shortURL))
}

// POST /api/shorten
func (srv *HTTPServer) handleAPIPost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req model.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(req.URL)
	if req.URL == "" {
		http.Error(w, "empty url field", http.StatusBadRequest)
		return
	}

	shortURL, err := srv.shorter.CreateShortURL(r.Context(), originalURL, srv.baseURL)
	if err != nil {
		if shortURL != "" {
			srv.publishAudit(r.Context(), "shorten", originalURL)
			resp := model.ShortenResponse{Result: shortURL}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict) // 409

			if err := json.NewEncoder(w).Encode(resp); err != nil {
				srv.logger.Error("failed to write JSON conflict response", zap.Error(err))
			}
			return
		}

		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}

	srv.publishAudit(r.Context(), "shorten", originalURL)
	resp := model.ShortenResponse{Result: shortURL}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.logger.Error("failed to write JSON response", zap.Error(err))
	}
}

func (srv *HTTPServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if srv.db == nil {
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

// POST /api/shorten/batch
func (srv *HTTPServer) handleAPIPostBatch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req []model.BatchShortenRequestItem

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req) == 0 {
		http.Error(w, "empty batch", http.StatusBadRequest)
		return
	}

	// готовим данные для сервиса
	items := make([]service.BatchItem, 0, len(req))
	for _, it := range req {
		items = append(items, service.BatchItem{
			CorrelationID: it.CorrelationID,
			OriginalURL:   it.OriginalURL,
		})
	}

	results, err := srv.shorter.CreateShortURLBatch(r.Context(), srv.baseURL, items)
	if err != nil {
		http.Error(w, "invalid URL in batch", http.StatusBadRequest)
		return
	}

	// ответ для клиента
	resp := make([]model.BatchShortenResponseItem, 0, len(results))
	for _, res := range results {
		resp = append(resp, model.BatchShortenResponseItem{
			CorrelationID: res.CorrelationID,
			ShortURL:      res.ShortURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.logger.Error("failed to write JSON batch response", zap.Error(err))
	}
}

// GET /api/user/urls
func (srv *HTTPServer) handleUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized) // 401
		return
	}

	userURLs, err := srv.shorter.GetUserURLs(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError) // 500
		return
	}

	if len(userURLs) == 0 {
		w.WriteHeader(http.StatusNoContent) // 204
		return
	}

	base := strings.TrimRight(srv.baseURL, "/")
	resp := make([]model.UserURLResponse, 0, len(userURLs))

	for _, u := range userURLs {
		resp = append(resp, model.UserURLResponse{
			ShortURL:    base + "/" + u.ShortID,
			OriginalURL: u.OriginalURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// DELETE /api/user/urls
func (srv *HTTPServer) handleUserURLsDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var shortIDs []string
	if err := json.NewDecoder(r.Body).Decode(&shortIDs); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if len(shortIDs) == 0 {
		http.Error(w, "empty list", http.StatusBadRequest) // 400
		return
	}

	if err := srv.shorter.DeleteUserURLsAsync(r.Context(), shortIDs); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError) // 500/503
		return
	}

	w.WriteHeader(http.StatusAccepted) // 202
}

// publishAudit рассылает событие аудита всем подключённым приёмникам.
func (srv *HTTPServer) publishAudit(ctx context.Context, action, originalURL string) {
	if srv.audit == nil {
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)

	ev := audit.Event{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    originalURL,
	}

	if err := srv.audit.Publish(ctx, ev); err != nil {
		if srv.logger != nil {
			srv.logger.Warn("audit publish failed", zap.Error(err))
		}
	}
}
