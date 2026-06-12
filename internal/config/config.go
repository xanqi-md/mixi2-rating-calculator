// Package config handles configuration loading from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// mixi2 OAuth2 credentials
	ClientID     string
	ClientSecret string
	TokenURL     string

	// mixi2 API endpoints
	APIAddress    string
	StreamAddress string

	// Webhook specific
	SignaturePublicKey string
	Port              string

	// Application settings
	AdminUserID string // The admin's mixi2 user ID who can record matches
	DBPath      string // Path to SQLite database file
}

// Load loads configuration from environment variables (and .env file if present).
func Load() (*Config, error) {
	// Try to load .env file (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		ClientID:          getEnv("CLIENT_ID", ""),
		ClientSecret:      getEnv("CLIENT_SECRET", ""),
		TokenURL:          getEnv("TOKEN_URL", "https://auth.mixi.social/oauth/token"),
		APIAddress:        getEnv("API_ADDRESS", "api.mixi.social:443"),
		StreamAddress:     getEnv("STREAM_ADDRESS", "stream.mixi.social:443"),
		SignaturePublicKey: getEnv("SIGNATURE_PUBLIC_KEY", ""),
		Port:              getEnv("PORT", "8080"),
		AdminUserID:       getEnv("ADMIN_USER_ID", ""),
		DBPath:            getEnv("DB_PATH", "./ratings.db"),
	}

	// Validate required fields
	var missing []string
	if cfg.ClientID == "" {
		missing = append(missing, "CLIENT_ID")
	}
	if cfg.ClientSecret == "" {
		missing = append(missing, "CLIENT_SECRET")
	}
	if cfg.AdminUserID == "" {
		missing = append(missing, "ADMIN_USER_ID")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return cfg, nil
}

// Validate checks if all required fields for webhook mode are set.
func (c *Config) ValidateWebhook() error {
	if c.SignaturePublicKey == "" {
		return fmt.Errorf("SIGNATURE_PUBLIC_KEY is required for webhook mode")
	}
	return nil
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt returns the integer value of an environment variable or a default.
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
