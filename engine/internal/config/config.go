package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	WALDir       string
	Instruments  []string
	LogLevel     string
	KafkaBrokers []string
	KafkaEnabled bool
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:     getEnv("MATCHING_ENGINE_PORT", "8081"),
		WALDir:   getEnv("WAL_DIR", "./data/wal"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
	instruments := getEnv("INSTRUMENTS", "")
	cfg.Instruments = parseInstruments(instruments)

	cfg.KafkaEnabled = strings.ToLower(getEnv("KAFKA_ENABLED", "false")) == "true"
	cfg.KafkaBrokers = parseCSV(getEnv("KAFKA_BROKERS", ""))

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInstruments(instruments string) []string {
	if instruments == "" {
		return []string{}
	}
	return strings.Split(instruments, ",")
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
