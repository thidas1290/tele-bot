package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// Telegram credentials
	APIID    int
	APIHash  string
	BotToken string

	// HTTP server
	HTTPPort int
	BaseURL  string

	// Storage
	DBPath      string
	SessionPath string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if it doesn't exist)
	_ = godotenv.Load()

	apiID, err := strconv.Atoi(getEnv("API_ID", "0"))
	if err != nil {
		return nil, err
	}

	httpPort, err := strconv.Atoi(getEnv("HTTP_PORT", "8080"))
	if err != nil {
		return nil, err
	}

	return &Config{
		APIID:       apiID,
		APIHash:     getEnv("API_HASH", ""),
		BotToken:    getEnv("BOT_TOKEN", ""),
		HTTPPort:    httpPort,
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
		DBPath:      getEnv("DB_PATH", "./data/metadata.db"),
		SessionPath: getEnv("SESSION_PATH", "./data/session"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
