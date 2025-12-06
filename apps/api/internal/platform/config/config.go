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
	SmartyAuthID        string
	SmartyAuthToken     string
	SmartyMock          bool
	AllowedOrigins      string
}

// Load reads environment variables into a Config with sensible defaults.
func Load() (Config, error) {
	cfg := Config{
		Port:                getEnv("PORT", "8080"),
		GinMode:             getEnv("GIN_MODE", "release"),
		FirebaseProjectID:   strings.TrimSpace(os.Getenv("FIREBASE_PROJECT_ID")),
		FirebaseCredsBase64: strings.TrimSpace(os.Getenv("FIREBASE_CREDS_BASE64")),
		FirebaseCredsFile:   strings.TrimSpace(os.Getenv("FIREBASE_CREDS_FILE")),
		SmartyAuthID:        strings.TrimSpace(os.Getenv("SMARTY_AUTH_ID")),
		SmartyAuthToken:     strings.TrimSpace(os.Getenv("SMARTY_AUTH_TOKEN")),
		AllowedOrigins:      strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")),
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
