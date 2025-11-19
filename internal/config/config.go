package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	RedisURL       string
	KindeIssuerURL string
	LogLevel       string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		KindeIssuerURL: getEnv("KINDE_ISSUER_URL", ""),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}
