// Package config loads addyui's runtime configuration from the environment.
package config

import (
	"errors"
	"os"
)

const defaultBaseURL = "https://app.addy.io"

// Config holds Addy API client configuration.
type Config struct {
	BaseURL string
	APIKey  string
}

// Load retrieves app configuration from env vars.
func Load() (Config, error) {
	cfg := Config{
		BaseURL: os.Getenv("ADDY_BASE_URL"),
		APIKey:  os.Getenv("ADDY_API_KEY"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("ADDY_API_KEY is not set; create one at https://app.addy.io/settings/api")
	}
	return cfg, nil
}
