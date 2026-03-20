package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	LogLevel string

	HTTPPort string

	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
}

func Load() *Config {
	return &Config{
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		HTTPPort:         getEnv("HTTP_PORT", "8090"),
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnvInt("POSTGRES_PORT", 5432),
		PostgresUser:     getEnv("POSTGRES_USER", "exchangeuser"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "securepassword"),
		PostgresDB:       getEnv("POSTGRES_DB", "exchangedb"),
		PostgresSSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),

		KafkaBrokers: parseCSV(getEnv("KAFKA_BROKERS", "localhost:29092")),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "trade.executed"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "settlement-v1"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func parseCSV(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
