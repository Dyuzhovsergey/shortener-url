package main

import (
	"github.com/Dyuzhovsergey/shortener-url/internal/linter/noexit"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(noexit.Analyzer)
}
