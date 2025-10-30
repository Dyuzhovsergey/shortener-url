package config

import (
	"flag"
	"strings"
)

var (
	FlagRunAddr string // -a
	FlagBaseURL string // -b
)

func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8080", " host and port to run server")
	flag.StringVar(&FlagBaseURL, "b", Param.BaseURL, "base URL for generated short links, 'http://localhost:8080'")

	flag.Parse()

	Param.BaseURL = strings.TrimRight(FlagBaseURL, "/")
}
