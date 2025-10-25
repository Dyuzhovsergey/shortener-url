package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
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
		handlePost(r, w)
	case http.MethodGet:
		handleGet(r, w)
	default:
		http.Error(w, "Method not allowed server", http.StatusMethodNotAllowed)
	}
}

func generateID() {
	randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
	id := make([]byte, idLength)

	for i := range id {
		index := randSrc.Intn(len(charSet))
		id[i] = charSet[index]
	}
}
