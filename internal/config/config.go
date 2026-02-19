package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	DatabaseURL string
	Port        int
	Env         string // "development", "production"
	DockerHost  string
}

// Load reads configuration from environment variables.
// In development, it loads from a .env file if present.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore errors — file is optional)
	_ = godotenv.Load()

	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	cfg := &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://orbex:orbex@localhost:5432/orbex?sslmode=disable"),
		Port:        port,
		Env:         getEnv("ENV", "development"),
		DockerHost:  getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

// Addr returns the server listen address.
func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// IsDev returns true if running in development mode.
func (c *Config) IsDev() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
