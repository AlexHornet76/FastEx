package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	GatewayPort            string
	PostgresHost           string
	PostgresPort           string
	PostgresUser           string
	PostgresPassword       string
	PostgresDB             string
	PostgresSSLMode        string
	JWTSecret              string
	JWTExpiryMinutes       int
	CORSAllowedOrigins     []string
	LogLevel               string
	ChallengeTTLMinutes    int
	MatchingEngineURL      string
	KafkaBrokers           []string
	KafkaTopicTradeSettled string
	KafkaGroupIDMarketWS   string
	MarketDataURL          string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try loading .env file (ignore error if not exists)
	_ = godotenv.Load()

	cfg := &Config{
		GatewayPort:            getEnv("GATEWAY_PORT", "8080"),
		PostgresHost:           getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:           getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:           getEnv("POSTGRES_USER", "exchangeuser"),
		PostgresPassword:       getEnv("POSTGRES_PASSWORD", "securepassword"),
		PostgresDB:             getEnv("POSTGRES_DB", "exchangedb"),
		PostgresSSLMode:        getEnv("POSTGRES_SSLMODE", "disable"),
		JWTSecret:              getEnv("JWT_SECRET", ""),
		JWTExpiryMinutes:       getEnvInt("JWT_EXPIRY_MINUTES", 15),
		CORSAllowedOrigins:     strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"), ","),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		ChallengeTTLMinutes:    getEnvInt("CHALLENGE_TTL_MINUTES", 5),
		MatchingEngineURL:      getEnv("MATCHING_ENGINE_URL", "http://localhost:8081"),
		KafkaBrokers:           parseCSV(getEnv("KAFKA_BROKERS", "localhost:29092")),
		KafkaTopicTradeSettled: getEnv("KAFKA_TOPIC_TRADE_SETTLED", "trade.settled"),
		KafkaGroupIDMarketWS:   getEnv("KAFKA_GROUP_ID_MARKET_WS", "gateway-market-ws"),
		MarketDataURL:          getEnv("MARKETDATA_URL", "http://localhost:8091"),
	}

	// Validate JWT secret
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters, got %d", len(cfg.JWTSecret))
	}

	return cfg, nil
}

// DatabaseURL returns the PostgreSQL connection string
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresSSLMode,
	)
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
