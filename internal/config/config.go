package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppName          string
	AppPort          string
	DatabaseURL      string
	AuthIssuer       string
	TokenTTL         time.Duration
	ResendAPIKey     string
	EmailFrom        string
	FrontendURL      string
	OTPExpiresIn     time.Duration
	ArgonMemory      uint32
	ArgonIterations  uint32
	ArgonParallelism uint8
	ArgonSaltLength  uint32
	ArgonKeyLength   uint32
}

func Load() Config {
	return Config{
		AppName:          getEnv("APP_NAME", "Fiber Banking API"),
		AppPort:          getEnv("APP_PORT", "3000"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/fiber_banking?sslmode=disable"),
		AuthIssuer:       getEnv("AUTH_ISSUER", "fiber-banking-api"),
		TokenTTL:         time.Duration(getEnvInt("ACCESS_TOKEN_TTL_HOURS", 24)) * time.Hour,
		ResendAPIKey:     getEnv("RESEND_API_KEY", ""),
		EmailFrom:        getEnv("EMAIL_FROM", "noreply@example.com"),
		FrontendURL:      getEnv("FRONTEND_URL", "http://localhost:3000"),
		OTPExpiresIn:     time.Duration(getEnvInt("OTP_EXPIRES_MINUTES", 10)) * time.Minute,
		ArgonMemory:      uint32(getEnvInt("ARGON_MEMORY", 64*1024)),
		ArgonIterations:  uint32(getEnvInt("ARGON_ITERATIONS", 3)),
		ArgonParallelism: uint8(getEnvInt("ARGON_PARALLELISM", 2)),
		ArgonSaltLength:  uint32(getEnvInt("ARGON_SALT_LENGTH", 16)),
		ArgonKeyLength:   uint32(getEnvInt("ARGON_KEY_LENGTH", 32)),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
