//go:generate go run ../reset
package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"

	pb "github.com/Dyuzhovsergey/shortener-url/api"
	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/certutil"
	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/handler"
	"github.com/Dyuzhovsergey/shortener-url/internal/logger"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
	"github.com/Dyuzhovsergey/shortener-url/migrations"
	"google.golang.org/grpc"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo()

	// ---- загрузка конфигурации ----
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Println("config error:", cfgErr)
		return
	}

	// ---- инициализация логгера ----
	zapLogger := logger.Init()
	defer zapLogger.Sync()

	// ---- инициализация аудита ----
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
		defer func() {
			_ = fileObserver.Close()
		}()
	}

	// ---- инициализация хранилища ----
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

	// ---- инициализация сервиса ----
	shorter := service.NewShorterService(repo, cfg)

	// ---- инициализация HTTP-сервера ----
	server := handler.NewHTTPServer(cfg.BaseURL, cfg.TrustedSubnet, shorter, zapLogger, db, auditor)
	router := server.Router()

	srv := &http.Server{
		Addr:    cfg.RunAddr,
		Handler: router,
	}

	// ---- инициализация gRPC-сервера ----
	grpcHandler := handler.NewGRPCServer(cfg.BaseURL, shorter, zapLogger, auditor)

	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(handler.GRPCAuthInterceptor(zapLogger)),
	)
	pb.RegisterShortenerServiceServer(grpcSrv, grpcHandler)

	// ---- создание общего listener ----
	var rootListener net.Listener

	if cfg.EnableHTTPS {
		cert, err := certutil.GenerateSelfSignedCertificate(cfg.BaseURL, cfg.RunAddr)
		if err != nil {
			zapLogger.Fatal("failed to generate TLS certificate", zap.Error(err))
		}

		tlsConfig := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}

		rootListener, err = tls.Listen("tcp", cfg.RunAddr, tlsConfig)
		if err != nil {
			zapLogger.Fatal("failed to start HTTPS listener", zap.Error(err))
		}

		zapLogger.Info("server started (HTTPS + gRPC)", zap.String("addr", cfg.RunAddr))
	} else {
		rootListener, err = net.Listen("tcp", cfg.RunAddr)
		if err != nil {
			zapLogger.Fatal("failed to start listener", zap.Error(err))
		}

		zapLogger.Info("server started (HTTP + gRPC)", zap.String("addr", cfg.RunAddr))
	}

	// ---- cmux делит один listener на gRPC и HTTP ----
	muxer := cmux.New(rootListener)

	grpcListener := muxer.MatchWithWriters(
		cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
	)
	httpListener := muxer.Match(cmux.Any())

	// ---- контекст завершения по сигналам ----
	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	// ---- запуск серверов через errgroup ----
	group, groupCtx := errgroup.WithContext(signalCtx)

	group.Go(func() error {
		err := grpcSrv.Serve(grpcListener)
		if err != nil &&
			!errors.Is(err, cmux.ErrListenerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("grpc server failed: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		err := srv.Serve(httpListener)
		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, cmux.ErrListenerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	})

	group.Go(func() error {
		err := muxer.Serve()
		if err != nil &&
			!errors.Is(err, cmux.ErrListenerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("cmux failed: %w", err)
		}
		return nil
	})

	// ---- ожидание сигнала завершения или ошибки одной из goroutine ----
	<-groupCtx.Done()

	// ---- причины shutdown ----
	if signalCtx.Err() != nil {
		zapLogger.Info("shutdown signal received")
	} else {
		zapLogger.Error("shutdown started because one of the servers returned an error")
	}

	// ---- graceful shutdown ----

	// 1. закрываем корневой listener, чтобы новые соединения больше не принимались
	if err := rootListener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		zapLogger.Error("failed to close root listener", zap.Error(err))
	}

	// 2. параллельная остановка gRPC и HTTP
	shutdownGroup, _ := errgroup.WithContext(context.Background())

	shutdownGroup.Go(func() error {
		done := make(chan struct{})

		go func() {
			grpcSrv.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			zapLogger.Info("grpc server stopped gracefully")
			return nil
		case <-time.After(5 * time.Second):
			zapLogger.Warn("grpc graceful stop timeout, forcing stop")
			grpcSrv.Stop()
			return nil
		}
	})

	shutdownGroup.Go(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server shutdown failed: %w", err)
		}

		zapLogger.Info("http server stopped gracefully")
		return nil
	})

	if err := shutdownGroup.Wait(); err != nil {
		zapLogger.Error("shutdown failed", zap.Error(err))
	}

	// 3. дожидаемся завершения серверных goroutine
	if err := group.Wait(); err != nil {
		zapLogger.Fatal("server failed", zap.Error(err))
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
