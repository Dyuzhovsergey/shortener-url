package main

import (
	"fmt"
	"log"
	"net/http"

	"go.uber.org/zap"

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

	var repo repository.Repository
	if cfg.FileStoragePath != "" {
		fileRepo, err := repository.NewFileRepository(cfg.FileStoragePath)
		if err != nil {
			log.Fatalf("cannot init file repository: %v", err)
		}
		repo = fileRepo
		zapLogger.Info("using file repository", zap.String("path", cfg.FileStoragePath))
	} else {
		repo = repository.NewMemoryRepository()
		zapLogger.Info("using in-memory repository")
	}

	// создаём сервис и внедряем репозиторий
	shorter := service.NewShorterService(repo, cfg)

	// создаём HTTP-сервер и внедряем сервис
	server := handler.NewHTTPServer(cfg.BaseURL, shorter, zapLogger)

	fmt.Printf("Server run on: http://%s\n", cfg.RunAddr)
	if err := http.ListenAndServe(cfg.RunAddr, server.Router()); err != nil {
		log.Fatal(err)
	}
}
