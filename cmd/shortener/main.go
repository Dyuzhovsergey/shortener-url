package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

func main() {
	// парсим флаги (-a и -b)
	config.ParseFlags()

	// создаём репозиторий (хранилище)
	repo := repository.NewMemoryRepository()

	// создаём сервис и внедряем репозиторий
	shorter := service.NewShorterService(repo)

	// создаём HTTP-сервер и внедряем сервис
	server := handler.NewHTTPServer(config.Param.BaseURL, shorter)

	fmt.Printf("Server run on: http://%s\n", config.FlagRunAddr)
	if err := http.ListenAndServe(config.FlagRunAddr, server.Router()); err != nil {
		log.Fatal(err)
	}
}
