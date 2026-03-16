package config

import (
	"flag"
	"os"
	"strconv"
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

	EnableHTTPS bool
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
	flagHTTPS := flag.Bool("s", false, "enable HTTPS")

	flag.Parse()

	*flagRunAddr = stringFromEnv("SERVER_ADDRESS", *flagRunAddr)
	*flagBaseURL = stringFromEnv("BASE_URL", *flagBaseURL)
	*flagFilePath = stringFromEnv("FILE_STORAGE_PATH", *flagFilePath)
	*flagDBDSN = stringFromEnv("DATABASE_DSN", *flagDBDSN)
	*flagAuditFile = stringFromEnv("AUDIT_FILE", *flagAuditFile)
	*flagAuditURL = stringFromEnv("AUDIT_URL", *flagAuditURL)

	enableHTTPS := boolFromEnv("ENABLE_HTTPS", *flagHTTPS)

	baseURL := strings.TrimRight(*flagBaseURL, "/")
	if enableHTTPS {
		baseURL = ensureHTTPS(baseURL)
	}

	cfg := &ShortenerConfig{
		CharSet:         charSet,
		LengthID:        lengthID,
		BaseURL:         baseURL,
		RunAddr:         *flagRunAddr,
		FileStoragePath: *flagFilePath,
		DatabaseDSN:     *flagDBDSN,
		AuditFile:       strings.TrimSpace(*flagAuditFile),
		AuditURL:        strings.TrimSpace(*flagAuditURL),
		EnableHTTPS:     enableHTTPS,
	}

	return cfg
}

// stringFromEnv читает строковую переменную окружения.
// Если переменная не задана или содержит только пробелы, возвращает fallback.
func stringFromEnv(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}

	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return value
}

// boolFromEnv читает bool-переменную окружения.
// Если переменная не задана или распарсить её не удалось, возвращает fallback.
func boolFromEnv(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

// ensureHTTPS заменяет схему URL на https://, если это нужно.
func ensureHTTPS(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")

	if strings.HasPrefix(baseURL, "https://") {
		return baseURL
	}

	if strings.HasPrefix(baseURL, "http://") {
		return "https://" + strings.TrimPrefix(baseURL, "http://")
	}

	return baseURL
}
