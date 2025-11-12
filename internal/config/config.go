// Package config for config program and flags
package config

import (
	"flag"
	"os"
	"strings"
)

type ShortenerConfig struct {
	CharSet  string
	LengthID int
	BaseURL  string
	RunAddr  string
}

func Load() *ShortenerConfig {

	const (
		defaultRunAddr = "localhost:8080"
		defaultBaseURL = "http://localhost:8080"
		charSet        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		lengthID       = 8
	)

	flagRunAddr := flag.String("a", defaultRunAddr, "server address, e.g. ':8080'")
	flagBaseURL := flag.String("b", defaultBaseURL, "base URL for short links")

	flag.Parse()

	if envRunAddr := os.Getenv("SERVER_ADDRESS"); envRunAddr != "" {
		*flagRunAddr = envRunAddr
	}

	if envBaseURL := os.Getenv("SERVER_ADDRESS"); envBaseURL != "" {
		*flagBaseURL = envBaseURL
	}

	baseURL := strings.TrimRight(*flagBaseURL, "/")

	cfg := &ShortenerConfig{
		CharSet:  charSet,
		LengthID: lengthID,
		BaseURL:  baseURL,
		RunAddr:  *flagRunAddr,
	}
	return cfg
}
