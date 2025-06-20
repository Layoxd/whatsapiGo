package config

import (
	"os"
)

// Config - Estructura de configuración de la aplicación
type Config struct {
	DatabaseURL  string
	RedisURL     string
	JWTSecret    string
	Port         string
	Environment  string
	LogLevel     string
}

// LoadConfig - Cargar configuración desde variables de entorno
func LoadConfig() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/whatsapp_api?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   getEnv("JWT_SECRET", "tu-jwt-secret-muy-seguro"),
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv - Obtener variable de entorno con valor por defecto
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}// Archivo base: config.go
