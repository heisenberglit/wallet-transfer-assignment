package db

import (
	"os"
	"strconv"
	"time"
)

// Config holds the connection string plus pool tuning parameters, kept
// separate from the connection logic in postgres.go so pooling behavior
// can be adjusted (or read from env/flags) without touching how the pool
// is constructed.
type Config struct {
	DSN string

	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// LoadConfigFromEnv reads connection settings from the environment,
// falling back to development-friendly defaults for local docker-compose use.
//
//	DATABASE_URL           postgres DSN
//	DB_MAX_CONNS            pool max connections (default 10)
//	DB_MIN_CONNS            pool min connections (default 0)
//	DB_MAX_CONN_LIFETIME    e.g. "1h" (default 1h)
//	DB_MAX_CONN_IDLE_TIME   e.g. "30m" (default 30m)
func LoadConfigFromEnv() Config {
	cfg := Config{
		DSN:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/wallet_transfer?sslmode=disable"),
		MaxConns:        getEnvInt32("DB_MAX_CONNS", 10),
		MinConns:        getEnvInt32("DB_MIN_CONNS", 0),
		MaxConnLifetime: getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		MaxConnIdleTime: getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt32(key string, fallback int32) int32 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(n)
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
