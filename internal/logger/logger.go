package logger

import (
	"log"

	"go.uber.org/zap"
)

// Init инициализирует zap-логгер приложения.
//
// Вызывается один раз в main, после чего логгер используется во всём приложении.
func Init() *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize zap logger: %v", err)
	}
	return logger
}
