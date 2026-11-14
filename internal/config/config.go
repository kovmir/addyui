// SPDX-License-Identifier: MIT

// Package config loads addyui's runtime configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const defaultBaseURL = "https://app.addy.io"

// defaultPageSize is used when ADDY_PAGE_SIZE is not set; maxPageSize is
// the largest page size the addy.io API accepts.
const (
	defaultPageSize = 100
	maxPageSize     = 100
)

// Config holds Addy API client configuration.
type Config struct {
	BaseURL string
	APIKey  string
	// PageSize is the number of aliases fetched per API request.
	PageSize int
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
	pageSize := defaultPageSize
	if v := os.Getenv("ADDY_PAGE_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("ADDY_PAGE_SIZE must be a number, got %q", v)
		}
		if n < 1 || n > maxPageSize {
			return Config{}, fmt.Errorf("ADDY_PAGE_SIZE must be between 1 and %d, got %d", maxPageSize, n)
		}
		pageSize = n
	}
	cfg.PageSize = pageSize
	return cfg, nil
}
