package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	WALDir      string
	Instruments []string
	LogLevel    string
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

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func parseInstruments(instruments string) []string {
	if instruments == "" {
		return []string{}
	}
	return strings.Split(instruments, ",")
}
