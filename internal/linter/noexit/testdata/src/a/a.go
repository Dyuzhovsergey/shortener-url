package main

import (
	"log"
	o "os"
)

func helperExit() {
	o.Exit(1) // want "os.Exit must not be called outside main"
}

func helperPanic() {
	panic("boom") // want "panic must not be called outside main"
}

func helperFatal() {
	log.Fatal("fatal") // want "log.Fatal/Fatalf/Fatalln must not be called outside main"
}

func helperFatalf() {
	log.Fatalf("fatal: %d", 1) // want "log.Fatal/Fatalf/Fatalln must not be called outside main"
}

func helperFatalln() {
	log.Fatalln("fatal") // want "log.Fatal/Fatalf/Fatalln must not be called outside main"
}

func main() {
	o.Exit(1)
	panic("allowed in main")
	log.Fatal("allowed in main")
	log.Fatalf("allowed in main")
	log.Fatalln("allowed in main")
}
