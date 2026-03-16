//go:generate go run ../reset
package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/certutil"
	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/logger"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
	"github.com/Dyuzhovsergey/shortener-url/migrations"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo()

	// парсим флаги
	cfg := config.Load()

	// инициализируем zap logger
	zapLogger := logger.Init()
	defer zapLogger.Sync()

	// ---------------- Аудит (паттерн «Наблюдатель») ----------------
	auditor := audit.NewPublisher()

	var fileObserver *audit.FileObserver
	if strings.TrimSpace(cfg.AuditFile) != "" {
		obs, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			zapLogger.Fatal("cannot init audit file observer", zap.Error(err))
		}
		fileObserver = obs
		auditor.Add(obs)
		zapLogger.Info("audit to file enabled", zap.String("path", cfg.AuditFile))
	}

	if strings.TrimSpace(cfg.AuditURL) != "" {
		auditor.Add(audit.NewHTTPObserver(cfg.AuditURL))
		zapLogger.Info("audit to remote enabled", zap.String("url", cfg.AuditURL))
	}

	if fileObserver != nil {
		defer func() { _ = fileObserver.Close() }()
	}

	var (
		repo repository.Repository
		db   *sql.DB
		err  error
	)

	if cfg.DatabaseDSN != "" {
		db, err = sql.Open("pgx", cfg.DatabaseDSN)
		if err != nil {
			zapLogger.Fatal("failed to open DB", zap.Error(err))
		}
		if err := migrations.Run(context.Background(), db); err != nil {
			zapLogger.Fatal("failed to run migrations", zap.Error(err))
		}
		repo = repository.NewPostgresRepository(db)
		zapLogger.Info("using postgres repository", zap.String("dsn", cfg.DatabaseDSN))

	} else if cfg.FileStoragePath != "" {
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

	// создаём сервис и внедряем репозиторий
	shorter := service.NewShorterService(repo, cfg)

	// создаём HTTP-сервер и внедряем сервис
	server := handler.NewHTTPServer(cfg.BaseURL, shorter, zapLogger, db, auditor)
	router := server.Router()

	if cfg.EnableHTTPS {
		cert, err := certutil.GenerateSelfSignedCertificate(cfg.BaseURL, cfg.RunAddr)
		if err != nil {
			zapLogger.Fatal("failed to generate TLS certificate", zap.Error(err))
		}

		tlsConfig := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}

		listener, err := tls.Listen("tcp", cfg.RunAddr, tlsConfig)
		if err != nil {
			zapLogger.Fatal("failed to start HTTPS listener", zap.Error(err))
		}

		httpServer := &http.Server{
			Handler: router,
		}

		fmt.Printf("Server run on: https://%s\n", cfg.RunAddr)
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zapLogger.Fatal("server stopped", zap.Error(err))
		}
		return
	}

	fmt.Printf("Server run on: http://%s\n", cfg.RunAddr)
	if err := http.ListenAndServe(cfg.RunAddr, router); err != nil && !errors.Is(err, http.ErrServerClosed) {
		zapLogger.Fatal("server stopped", zap.Error(err))
	}
}

func printBuildInfo() {
	version := buildVersion
	if version == "" {
		version = "N/A"
	}

	date := buildDate
	if date == "" {
		date = "N/A"
	}

	commit := buildCommit
	if commit == "" {
		commit = "N/A"
	}

	fmt.Println("Build version:", version)
	fmt.Println("Build date:", date)
	fmt.Println("Build commit:", commit)
}
