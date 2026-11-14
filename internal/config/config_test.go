package config

import (
	"strings"
	"testing"
)

// The tests below manipulate environment variables with t.Setenv,
// which mutates process-wide state; it cannot be combined with
// t.Parallel(), so these tests intentionally run serially.

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ADDY_API_KEY", "secret")
	// Ensure no stale base URL leaks in from the environment.
	t.Setenv("ADDY_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want the default %q", cfg.BaseURL, defaultBaseURL)
	}
	if cfg.APIKey != "secret" {
		t.Errorf("APIKey = %q, want \"secret\"", cfg.APIKey)
	}
}

func TestLoadCustomBaseURL(t *testing.T) {
	t.Setenv("ADDY_API_KEY", "secret")
	t.Setenv("ADDY_BASE_URL", "http://127.0.0.1:18099")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.BaseURL != "http://127.0.0.1:18099" {
		t.Errorf("BaseURL = %q, want \"http://127.0.0.1:18099\"", cfg.BaseURL)
	}
	if cfg.APIKey != "secret" {
		t.Errorf("APIKey = %q, want \"secret\"", cfg.APIKey)
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	t.Setenv("ADDY_API_KEY", "")
	t.Setenv("ADDY_BASE_URL", "")

	cfg, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want an error when ADDY_API_KEY is not set")
	}
	if !strings.Contains(err.Error(), "ADDY_API_KEY is not set") {
		t.Errorf("error = %q, want it to mention the missing variable", err.Error())
	}
	if !strings.Contains(err.Error(), "https://app.addy.io/settings/api") {
		t.Errorf("error = %q, want it to point to the token settings page", err.Error())
	}
	// A failed load must not return a half-filled config.
	if cfg != (Config{}) {
		t.Errorf("Load() = %+v on error, want the zero Config", cfg)
	}
}

func TestLoadDefaultPageSize(t *testing.T) {
	t.Setenv("ADDY_API_KEY", "secret")
	t.Setenv("ADDY_BASE_URL", "")
	t.Setenv("ADDY_PAGE_SIZE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.PageSize != 100 {
		t.Errorf("PageSize = %d, want the default 100", cfg.PageSize)
	}
}

func TestLoadCustomPageSize(t *testing.T) {
	t.Setenv("ADDY_API_KEY", "secret")
	t.Setenv("ADDY_BASE_URL", "")
	t.Setenv("ADDY_PAGE_SIZE", "25")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.PageSize != 25 {
		t.Errorf("PageSize = %d, want 25", cfg.PageSize)
	}
}

func TestLoadInvalidPageSize(t *testing.T) {
	for name, value := range map[string]string{
		"not a number": "abc",
		"zero":         "0",
		"negative":     "-5",
		"above max":    "101",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("ADDY_API_KEY", "secret")
			t.Setenv("ADDY_BASE_URL", "")
			t.Setenv("ADDY_PAGE_SIZE", value)

			cfg, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want an error for ADDY_PAGE_SIZE=%q", value)
			}
			if !strings.Contains(err.Error(), "ADDY_PAGE_SIZE") {
				t.Errorf("error = %q, want it to mention the variable", err.Error())
			}
			// A failed load must not return a half-filled config.
			if cfg != (Config{}) {
				t.Errorf("Load() = %+v on error, want the zero Config", cfg)
			}
		})
	}
}
