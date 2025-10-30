// Package handler for working HTTP server
package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

// HTTPServer — слой HTTP, знает про сервис, но не про репозиторий.
type HTTPServer struct {
	baseURL string
	shorter *service.ShorterService
}

// NewHTTPServer — конструктор с внедрением зависимости (DI)
func NewHTTPServer(baseURL string, shorter *service.ShorterService) *HTTPServer {
	return &HTTPServer{
		baseURL: baseURL,
		shorter: shorter,
	}
}

// Router — возвращает готовый http.Handler (ServeMux)
func (srv *HTTPServer) Router() http.Handler {
	r := chi.NewRouter()
	r.Post("/", srv.handlePost)
	r.Get("/{id}", srv.handleGet)
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
