// Package handler for working HTTP server
package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/model"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

// HTTPServer — слой HTTP, знает про сервис, но не про репозиторий.
type HTTPServer struct {
	baseURL string
	shorter *service.ShorterService
	logger  *zap.Logger
}

// NewHTTPServer — конструктор с внедрением зависимости (DI)
func NewHTTPServer(baseURL string, shorter *service.ShorterService, logger *zap.Logger) *HTTPServer {
	return &HTTPServer{
		baseURL: baseURL,
		shorter: shorter,
		logger:  logger,
	}
}

// // структуры для JSON API
// type shortenRequest struct {
// 	URL string `json:"url"`
// }

// type shortenResponse struct {
// 	Result string `json:"result"`
// }

// Router — возвращает готовый http.Handler (ServeMux)

func (srv *HTTPServer) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.ZapLogger(srv.logger))

	r.Post("/", srv.handlePost)
	r.Get("/{id}", srv.handleGet)
	r.Post("/api/shorten", srv.handleAPIPost)
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

	shortURL, err := srv.shorter.CreateShortURL(string(body), srv.baseURL)
	if err != nil {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

// POST /api/shorten
func (srv *HTTPServer) handleAPIPost(w http.ResponseWriter, r *http.Request) {

	var req model.ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON for request POST /api/shorten", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "empty url field", http.StatusBadRequest)
		return
	}

	shortURL, err := srv.shorter.CreateShortURL(req.URL, srv.baseURL)
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

// GET /{id}
func (srv *HTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad GET request", http.StatusBadRequest)
		return
	}

	originalURL, ok := srv.shorter.GetOriginalURL(id)
	if !ok {
		http.Error(w, "URL not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
