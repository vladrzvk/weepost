package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL      string
	JWTSecret        string
	JWTRefreshSecret string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration
	Argon2Pepper     string
	CryptoKey        string
	AppPort          string
	AppEnv           string
	CORSOrigins      string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		DatabaseURL:      mustEnv("DATABASE_URL"),
		JWTSecret:        mustEnv("JWT_SECRET"),
		JWTRefreshSecret: mustEnv("JWT_REFRESH_SECRET"),
		JWTAccessExpiry:  parseDuration(os.Getenv("JWT_ACCESS_EXPIRY"), 15*time.Minute),
		JWTRefreshExpiry: parseDuration(os.Getenv("JWT_REFRESH_EXPIRY"), 168*time.Hour),
		Argon2Pepper:     mustEnv("ARGON2_PEPPER"),
		CryptoKey:        mustEnv("CRYPTO_KEY"),
		AppPort:          envOrDefault("APP_PORT", "3000"),
		AppEnv:           envOrDefault("APP_ENV", "development"),
		CORSOrigins:      envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3001"),
	}, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing required env var: " + key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
