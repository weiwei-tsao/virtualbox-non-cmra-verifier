package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port                string
	GinMode             string
	FirebaseProjectID   string
	FirebaseCredsBase64 string
	FirebaseCredsFile   string
	SmartyAuthIDs       []string // Multiple auth IDs for load balancing
	SmartyAuthTokens    []string // Multiple auth tokens (must match IDs length)
	SmartyMock          bool
	AllowedOrigins      string
	CrawlLinkSeeds      []string
}

// Load reads environment variables into a Config with sensible defaults.
func Load() (Config, error) {
	cfg := Config{
		Port:                getEnv("PORT", "8080"),
		GinMode:             getEnv("GIN_MODE", "release"),
		FirebaseProjectID:   strings.TrimSpace(os.Getenv("FIREBASE_PROJECT_ID")),
		FirebaseCredsBase64: strings.TrimSpace(os.Getenv("FIREBASE_CREDS_BASE64")),
		FirebaseCredsFile:   strings.TrimSpace(os.Getenv("FIREBASE_CREDS_FILE")),
		SmartyAuthIDs:       splitCSV(os.Getenv("SMARTY_AUTH_ID")),      // Parse comma-separated IDs
		SmartyAuthTokens:    splitCSV(os.Getenv("SMARTY_AUTH_TOKEN")),   // Parse comma-separated tokens
		AllowedOrigins:      strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")),
		CrawlLinkSeeds:      splitCSV(os.Getenv("CRAWL_LINK_SEEDS")),
	}

	mock, err := parseBoolEnv("SMARTY_MOCK", false)
	if err != nil {
		return Config{}, fmt.Errorf("parse SMARTY_MOCK: %w", err)
	}
	cfg.SmartyMock = mock

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate ensures required fields are present.
func (c Config) Validate() error {
	if c.Port == "" {
		return errors.New("PORT is required")
	}
	if c.FirebaseProjectID == "" {
		return errors.New("FIREBASE_PROJECT_ID is required")
	}
	if c.FirebaseCredsBase64 == "" && c.FirebaseCredsFile == "" {
		return errors.New("provide FIREBASE_CREDS_BASE64 or FIREBASE_CREDS_FILE for Firestore auth")
	}
	// Validate Smarty credentials count matches (when not in mock mode)
	if !c.SmartyMock && len(c.SmartyAuthIDs) != len(c.SmartyAuthTokens) {
		return fmt.Errorf("SMARTY_AUTH_ID count (%d) must match SMARTY_AUTH_TOKEN count (%d)",
			len(c.SmartyAuthIDs), len(c.SmartyAuthTokens))
	}
	if !c.SmartyMock && len(c.SmartyAuthIDs) == 0 {
		return errors.New("SMARTY_AUTH_ID and SMARTY_AUTH_TOKEN are required when SMARTY_MOCK=false")
	}
	return nil
}

// FirebaseCredentialsJSON returns the service account JSON bytes and the source used.
func (c Config) FirebaseCredentialsJSON() ([]byte, string, error) {
	if c.FirebaseCredsBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(c.FirebaseCredsBase64)
		if err != nil {
			return nil, "base64", fmt.Errorf("decode FIREBASE_CREDS_BASE64: %w", err)
		}
		return decoded, "base64", nil
	}
	if c.FirebaseCredsFile != "" {
		data, err := os.ReadFile(c.FirebaseCredsFile)
		if err != nil {
			return nil, "file", fmt.Errorf("read FIREBASE_CREDS_FILE: %w", err)
		}
		return data, "file", nil
	}
	return nil, "", errors.New("no firebase credentials found")
}

func getEnv(key, defaultVal string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return defaultVal
}

func parseBoolEnv(key string, defaultVal bool) (bool, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal, nil
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return false, err
	}
	return parsed, nil
}

func splitCSV(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
