package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// ShortenerConfig описывает параметры запуска сервиса.
//
// Значения заполняются из:
// 1. значений по умолчанию,
// 2. файла конфигурации,
// 3. флагов командной строки,
// 4. переменных окружения.
//
// Приоритет ENV > flags > JSON > Default.
type ShortenerConfig struct {
	CharSet  string
	LengthID int
	BaseURL  string
	RunAddr  string

	FileStoragePath string
	DatabaseDSN     string

	AuditFile     string
	AuditURL      string
	TrustedSubnet string

	EnableHTTPS bool
}

// fileConfig описывает формат JSON-конфига.
type fileConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	TrustedSubnet   string `json:"trusted_subnet"`
	EnableHTTPS     *bool  `json:"enable_https"`
}

// Load читает конфигурацию из флагов, JSON-файла и переменных окружения.
func Load() (*ShortenerConfig, error) {
	const (
		defaultRunAddr       = "localhost:8080"
		defaultBaseURL       = "http://localhost:8080"
		charSet              = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		lengthID             = 8
		defaultFileStorePath = "shortener-storage.json"
	)

	// 1. Сначала определяем путь к JSON-конфигу.
	configPath := findConfigPath()

	// 2. Берём значения по умолчанию как базу.
	runAddr := defaultRunAddr
	baseURL := defaultBaseURL
	fileStoragePath := defaultFileStorePath
	trustedSubnet := ""
	databaseDSN := ""
	enableHTTPS := false

	// 3. Если файл задан — подмешиваем его значения поверх defaults.
	if configPath != "" {
		fc, err := loadFileConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("cannot load config file %q: %w", configPath, err)
		}

		if strings.TrimSpace(fc.ServerAddress) != "" {
			runAddr = fc.ServerAddress
		}
		if strings.TrimSpace(fc.BaseURL) != "" {
			baseURL = fc.BaseURL
		}
		if strings.TrimSpace(fc.FileStoragePath) != "" {
			fileStoragePath = fc.FileStoragePath
		}
		if strings.TrimSpace(fc.DatabaseDSN) != "" {
			databaseDSN = fc.DatabaseDSN
		}
		trustedSubnet = strings.TrimSpace(fc.TrustedSubnet)

		if fc.EnableHTTPS != nil {
			enableHTTPS = *fc.EnableHTTPS
		}
	}

	// 4. Теперь объявляем флаги уже с учётом значений из файла.
	flagRunAddr := flag.String("a", runAddr, "server address, e.g. ':8080'")
	flagBaseURL := flag.String("b", baseURL, "base URL for short links")
	flagFilePath := flag.String("f", fileStoragePath, "file path for URL storage")
	flagDBDSN := flag.String("d", databaseDSN, "PostgreSQL DSN")
	flagTrustedSubnet := flag.String("t", trustedSubnet, "trusted subnet in CIDR notation")
	flagAuditFile := flag.String("audit-file", "", "path to audit log file")
	flagAuditURL := flag.String("audit-url", "", "remote audit URL")
	flagHTTPS := flag.Bool("s", enableHTTPS, "enable HTTPS")

	// Поддержка -c и -config.
	flag.String("c", configPath, "path to JSON config file")
	flag.String("config", configPath, "path to JSON config file")

	flag.Parse()

	// 5. Поверх всего накладываем ENV — у них самый высокий приоритет.
	*flagRunAddr = stringFromEnv("SERVER_ADDRESS", *flagRunAddr)
	*flagBaseURL = stringFromEnv("BASE_URL", *flagBaseURL)
	*flagFilePath = stringFromEnv("FILE_STORAGE_PATH", *flagFilePath)
	*flagDBDSN = stringFromEnv("DATABASE_DSN", *flagDBDSN)
	*flagTrustedSubnet = stringFromEnvAllowEmpty("TRUSTED_SUBNET", *flagTrustedSubnet)
	*flagAuditFile = stringFromEnv("AUDIT_FILE", *flagAuditFile)
	*flagAuditURL = stringFromEnv("AUDIT_URL", *flagAuditURL)

	enableHTTPS = boolFromEnv("ENABLE_HTTPS", *flagHTTPS)

	baseURL = strings.TrimRight(*flagBaseURL, "/")
	if enableHTTPS {
		baseURL = ensureHTTPS(baseURL)
	}

	trustedSubnet = strings.TrimSpace(*flagTrustedSubnet)
	if trustedSubnet != "" {
		if _, _, err := net.ParseCIDR(trustedSubnet); err != nil {
			return nil, fmt.Errorf("invalid trusted_subnet %q: %w", trustedSubnet, err)
		}
	}

	cfg := &ShortenerConfig{
		CharSet:         charSet,
		LengthID:        lengthID,
		BaseURL:         baseURL,
		RunAddr:         strings.TrimSpace(*flagRunAddr),
		FileStoragePath: strings.TrimSpace(*flagFilePath),
		DatabaseDSN:     strings.TrimSpace(*flagDBDSN),
		AuditFile:       strings.TrimSpace(*flagAuditFile),
		AuditURL:        strings.TrimSpace(*flagAuditURL),
		EnableHTTPS:     enableHTTPS,
	}

	return cfg, nil
}

// findConfigPath ищет путь к JSON-конфигу.
// Приоритет: флаги -c/-config, затем переменная окружения CONFIG.
func findConfigPath() string {
	envPath := strings.TrimSpace(os.Getenv("CONFIG"))

	var configPath string
	args := os.Args[1:]

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "-c" || arg == "-config":
			if i+1 < len(args) {
				configPath = strings.TrimSpace(args[i+1])
			}
		case strings.HasPrefix(arg, "-c="):
			configPath = strings.TrimSpace(strings.TrimPrefix(arg, "-c="))
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimSpace(strings.TrimPrefix(arg, "-config="))
		}
	}

	if configPath != "" {
		return configPath
	}

	return envPath
}

// loadFileConfig читает JSON-конфиг из файла.
func loadFileConfig(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
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

// stringFromEnvAllowEmpty читает строковую переменную окружения.
// Если переменная задана, возвращает её значение даже если оно пустое.
func stringFromEnvAllowEmpty(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
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

// ensureHTTPS заменяет схему URL на https://
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
