// Package config for config program and flags
package config

import (
	"flag"
	"os"
	"strings"
)

type ShortenerConfig struct {
	CharSet         string
	LengthID        int
	BaseURL         string
	RunAddr         string
	FileStoragePath string
	DatabaseDSN     string
}

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

	baseURL := strings.TrimRight(*flagBaseURL, "/")

	cfg := &ShortenerConfig{
		CharSet:         charSet,
		LengthID:        lengthID,
		BaseURL:         baseURL,
		RunAddr:         *flagRunAddr,
		FileStoragePath: *flagFilePath,
		DatabaseDSN:     *flagDBDSN,
	}
	return cfg
}
