// Package logger init zap.Logger
package logger

import (
	"log"

	"go.uber.org/zap"
)

// Init zap.Logger
func Init() *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize zap logger: %v", err)
	}
	return logger
}
