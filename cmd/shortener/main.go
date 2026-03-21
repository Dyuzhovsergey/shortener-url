//go:generate go run ../reset
package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

	// парсим конфигурацию
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Println("config error:", cfgErr)
		return
	}

	// инициализируем zap logger
	zapLogger := logger.Init()
	defer zapLogger.Sync()

	// ---------------- Аудит ----------------
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
			zapLogger.Fatal("cannot init file repository", zap.Error(err))
		}
		repo = fileRepo
		zapLogger.Info("using file repository", zap.String("path", cfg.FileStoragePath))
	} else {
		repo = repository.NewMemoryRepository()
		zapLogger.Info("using in-memory repository")
	}

	// создаём сервис
	shorter := service.NewShorterService(repo, cfg)

	// создаём HTTP-сервер
	server := handler.NewHTTPServer(cfg.BaseURL, shorter, zapLogger, db, auditor)
	router := server.Router()

	// создаём http.Server (ВАЖНО для graceful shutdown)
	srv := &http.Server{
		Addr:    cfg.RunAddr,
		Handler: router,
	}

	// ---- запуск сервера в горутине ----
	go func() {
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

			zapLogger.Info("server started (HTTPS)", zap.String("addr", cfg.RunAddr))

			if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				zapLogger.Fatal("server failed", zap.Error(err))
			}
			return
		}

		zapLogger.Info("server started (HTTP)", zap.String("addr", cfg.RunAddr))

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zapLogger.Fatal("server failed", zap.Error(err))
		}
	}()

	// ---- обработка сигналов ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	sig := <-quit
	zapLogger.Info("shutdown signal received", zap.String("signal", sig.String()))

	// ---- graceful shutdown ----
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		zapLogger.Error("server shutdown failed", zap.Error(err))
	} else {
		zapLogger.Info("server stopped gracefully")
	}

	// ---- закрытие ресурсов ----
	if db != nil {
		if err := db.Close(); err != nil {
			zapLogger.Error("failed to close DB", zap.Error(err))
		}
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
