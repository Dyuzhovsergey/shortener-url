// Package handler for working HTTP server
package handler

import (
	"io"
	"net/http"
	"strings"

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
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleRequest)
	return mux
}

// handleRequest распределяет методы
func (srv *HTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		srv.handlePost(w, r)
	case http.MethodGet:
		srv.handleGet(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	id := strings.TrimPrefix(r.URL.Path, "/")
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
