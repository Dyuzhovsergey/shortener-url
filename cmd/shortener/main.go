package main

import (
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/logger"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
)

func main() {
	// парсим флаги
	cfg := config.Load()

	// инициализируем zap logger
	zapLogger := logger.Init()
	defer zapLogger.Sync()

	var repo repository.Repository

	if cfg.FileStoragePath != "" {
		fileRepo, err := repository.NewFileRepository(cfg.FileStoragePath)
		if err != nil {
			zapLogger.Fatal("cannot init file repository: %v", zap.Error(err))
		}
		repo = fileRepo
		zapLogger.Info("using file repository", zap.String("path", cfg.FileStoragePath))
	} else {
		repo = repository.NewMemoryRepository()
		zapLogger.Info("using in-memory repository")

	}

	var (
		db  *sql.DB
		err error
	)

	if cfg.DatabaseDSN != "" {
		db, err = sql.Open("pgx", cfg.DatabaseDSN)
		if err != nil {
			zapLogger.Fatal("failed to open DB", zap.Error(err))
		}
	}

	// создаём сервис и внедряем репозиторий
	shorter := service.NewShorterService(repo, cfg)

	// создаём HTTP-сервер и внедряем сервис
	server := handler.NewHTTPServer(cfg.BaseURL, shorter, zapLogger, db)

	fmt.Printf("Server run on: http://%s\n", cfg.RunAddr)
	if err := http.ListenAndServe(cfg.RunAddr, server.Router()); err != nil {
		zapLogger.Fatal("server stopped", zap.Error(err))
	}
}
