package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var ulrStore = make(map[string]string)

const (
	baseURL  = "http://localhost:8080"
	charSet  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	idLength = 8
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRequest)

	fmt.Printf("Server run on: %s", baseURL)
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handlePost(w, r)
	case http.MethodGet:
		handleGet(w, r)
	default:
		http.Error(w, "Method not allowed server", http.StatusMethodNotAllowed)
	}
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Bad POST request ", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Error read body or len bbody = 0", http.StatusBadRequest)
		return
	}
	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" || !strings.HasPrefix(originalURL, "http") {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	//create short id
	shortID := generateID()
	ulrStore[shortID] = originalURL

	shortURL := fmt.Sprintf("%s/%s", baseURL, shortID)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" {
		http.Error(w, "Bad GET request", http.StatusBadRequest)
		return
	}
	originalURL, exists := ulrStore[id]
	if !exists {
		http.Error(w, "URL not found", http.StatusBadGateway)
	}
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func generateID() string {
	randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := make([]byte, idLength)

	for i := range id {
		index := randSrc.Intn(len(charSet))
		id[i] = charSet[index]
	}
	return string(id)
}
