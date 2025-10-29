package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

const baseURL = "http://localhost:8080"

func main() {
	// 1️⃣ создаём репозиторий (хранилище)
	repo := repository.NewMemoryRepository()

	// 2️⃣ создаём сервис и внедряем репозиторий
	shorter := service.NewShorterService(repo)

	// 3️⃣ создаём HTTP-сервер и внедряем сервис
	server := handler.NewHTTPServer(baseURL, shorter)

	fmt.Printf("Server run on: %s\n", baseURL)
	if err := http.ListenAndServe(":8080", server.Router()); err != nil {
		log.Fatal(err)
	}
}
