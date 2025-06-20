package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SetupLogger - Configurar logger con zap
func SetupLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.StacktraceKey = ""
	
	logger, _ := config.Build()
	return logger
}// Archivo base: logger.go
