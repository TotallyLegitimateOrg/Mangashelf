package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	HTTPPort           string
	JWTSecret          string
	JWTSecretGenerated bool
	DatabasePath       string
	CatboxUserHash     string
	DevWebURL          string
	DevExtensionURL    string
}

func Load() (Config, error) {
	jwtSecret := env("MANGASHELF_JWT_SECRET")
	jwtSecretGenerated := false
	if jwtSecret == "" {
		generated, err := generateSecret()
		if err != nil {
			return Config{}, fmt.Errorf("generate JWT secret: %w", err)
		}
		jwtSecret = generated
		jwtSecretGenerated = true
	}

	cfg := Config{
		HTTPPort:           envOrDefault("MANGASHELF_HTTP_PORT", "3001"),
		JWTSecret:          jwtSecret,
		JWTSecretGenerated: jwtSecretGenerated,
		DatabasePath:       envOrDefault("MANGASHELF_DATABASE_PATH", "./data/mangashelf.db"),
		CatboxUserHash:     env("MANGASHELF_CATBOX_USERHASH"),
		DevWebURL:          trimURL(os.Getenv("MANGASHELF_DEV_WEB_URL")),
		DevExtensionURL:    trimURL(os.Getenv("MANGASHELF_DEV_EXTENSION_URL")),
	}

	if cfg.HTTPPort == "" {
		return Config{}, fmt.Errorf("MANGASHELF_HTTP_PORT must not be empty")
	}
	if cfg.DatabasePath == "" {
		return Config{}, fmt.Errorf("MANGASHELF_DATABASE_PATH must not be empty")
	}

	return cfg, nil
}

func (c Config) EnsureDatabaseDir() error {
	if c.DatabasePath == "" || c.DatabasePath == ":memory:" {
		return nil
	}

	dir := filepath.Dir(c.DatabasePath)
	if dir == "" || dir == "." {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}

func envOrDefault(key string, fallback string) string {
	if value := env(key); value != "" {
		return value
	}
	return fallback
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func trimURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func generateSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
