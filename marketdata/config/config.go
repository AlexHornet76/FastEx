package config

import (
	"os"
	"strings"
)

type Config struct {
	LogLevel     string
	HTTPPort     string
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
}

func Load() *Config {
	return &Config{
		LogLevel: getEnv("LOG_LEVEL", ""),

		HTTPPort: getEnv("MARKETDATA_PORT", ""),

		KafkaBrokers: parseCSV(getEnv("KAFKA_BROKERS", "")),
		KafkaTopic:   getEnv("KAFKA_TOPIC", ""),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", ""),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
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
