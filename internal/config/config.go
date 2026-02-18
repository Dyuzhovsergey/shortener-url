// Package config содержит загрузку конфигурации сервиса сокращения URL из флагов
// командной строки и переменных окружения.
package config

import (
	"flag"
	"os"
	"strings"
)

// ShortenerConfig описывает параметры запуска сервиса.
//
// Значения заполняются из флагов командной строки и переменных окружения.
// Переменные окружения имеют приоритет над флагами.
type ShortenerConfig struct {
	CharSet  string
	LengthID int
	BaseURL  string
	RunAddr  string

	FileStoragePath string
	DatabaseDSN     string

	AuditFile string
	AuditURL  string
}

// Load читает конфигурацию из флагов и переменных окружения, применяя значения
// по умолчанию.
func Load() *ShortenerConfig {
	const (
		defaultRunAddr       = "localhost:8080"
		defaultBaseURL       = "http://localhost:8080"
		charSet              = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		lengthID             = 8
		defaultFileStorePath = "shortener-storage.json"
	)

	flagRunAddr := flag.String("a", defaultRunAddr, "server address, e.g. ':8080'")
	flagBaseURL := flag.String("b", defaultBaseURL, "base URL for short links")
	flagFilePath := flag.String("f", defaultFileStorePath, "file path for URL storage")
	flagDBDSN := flag.String("d", "", "PostgreSQL DSN")

	flagAuditFile := flag.String("audit-file", "", "path to audit log file")
	flagAuditURL := flag.String("audit-url", "", "remote audit URL")

	flag.Parse()

	if envRunAddr := os.Getenv("SERVER_ADDRESS"); envRunAddr != "" {
		*flagRunAddr = envRunAddr
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		*flagBaseURL = envBaseURL
	}

	if envFilePath := os.Getenv("FILE_STORAGE_PATH"); envFilePath != "" {
		*flagFilePath = envFilePath
	}

	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		*flagDBDSN = envDSN
	}

	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		*flagAuditFile = envAuditFile
	}

	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		*flagAuditURL = envAuditURL
	}

	baseURL := strings.TrimRight(*flagBaseURL, "/")

	cfg := &ShortenerConfig{
		CharSet:         charSet,
		LengthID:        lengthID,
		BaseURL:         baseURL,
		RunAddr:         *flagRunAddr,
		FileStoragePath: *flagFilePath,
		DatabaseDSN:     *flagDBDSN,
		AuditFile:       strings.TrimSpace(*flagAuditFile),
		AuditURL:        strings.TrimSpace(*flagAuditURL),
	}
	return cfg
}
