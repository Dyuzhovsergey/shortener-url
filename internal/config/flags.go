package config

import (
	"flag"
)

var FlagRunAddr string

func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8080", " host and port to run server")
	flag.Parse()
}
