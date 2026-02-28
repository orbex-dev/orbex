package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	DatabaseURL       string
	Port              int
	Env               string // "development", "production"
	DockerHost        string
	MaxConcurrentRuns int

	// MinIO storage
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// GitHub OAuth
	GithubClientID     string
	GithubClientSecret string

	// Build
	MaxConcurrentBuilds int
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

	maxConcurrent, err := strconv.Atoi(getEnv("MAX_CONCURRENT_RUNS", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_CONCURRENT_RUNS: %w", err)
	}

	maxBuilds, err := strconv.Atoi(getEnv("ORBEX_MAX_BUILDS", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid ORBEX_MAX_BUILDS: %w", err)
	}

	minioSSL := getEnv("MINIO_USE_SSL", "false") == "true"

	cfg := &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://orbex:orbex@localhost:5432/orbex?sslmode=disable"),
		Port:              port,
		Env:               getEnv("ENV", "development"),
		DockerHost:        getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		MaxConcurrentRuns: maxConcurrent,

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "orbex"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "orbexsecret"),
		MinioBucket:    getEnv("MINIO_BUCKET", "orbex-storage"),
		MinioUseSSL:    minioSSL,

		GithubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),

		MaxConcurrentBuilds: maxBuilds,
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
