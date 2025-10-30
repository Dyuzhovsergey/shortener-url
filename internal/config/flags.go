package config

import (
	"flag"
)

var FlagRunAddr string

func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8080", " port to run server")
	flag.Parse()
}
