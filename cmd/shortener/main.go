package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/logger"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

func main() {
	// парсим флаги (-a и -b)
	cfg := config.Load()

	// инициализируем zap logger
	zapLogger := logger.Init()
	defer zapLogger.Sync()

	// создаём репозиторий (хранилище)
	repo := repository.NewMemoryRepository()

	// создаём сервис и внедряем репозиторий
	shorter := service.NewShorterService(repo, cfg)

	// создаём HTTP-сервер и внедряем сервис
	server := handler.NewHTTPServer(cfg.BaseURL, shorter, zapLogger)

	fmt.Printf("Server run on: http://%s\n", cfg.RunAddr)
	if err := http.ListenAndServe(cfg.RunAddr, server.Router()); err != nil {
		log.Fatal(err)
	}
}
