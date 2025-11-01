// Package config for config program and flags
package config

import (
	"flag"
	"strings"
)

type ShortenerConfig struct {
	CharSet  string
	LengthID int
	BaseURL  string
	RunAddr  string
}

func Load() *ShortenerConfig {

	charSet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	lengthID := 8
	defaultBaseURL := "http://localhost:8080"
	defaultRunAddr := "localhost:8080"

	runAddr := flag.String("a", defaultRunAddr, "server address, e.g. ':8080'")
	baseURL := flag.String("b", defaultBaseURL, "base URL for short links")

	flag.Parse()

	cfg := &ShortenerConfig{
		CharSet:  charSet,
		LengthID: lengthID,
		BaseURL:  strings.TrimRight(*baseURL, "/"),
		RunAddr:  *runAddr,
	}
	return cfg
}
