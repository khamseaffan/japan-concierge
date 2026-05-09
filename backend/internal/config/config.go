// Package config loads runtime configuration from environment variables.
//
// Lookups are explicit (no struct tag magic) so it's obvious at the call site
// what keys exist and what their defaults are. The config struct is immutable
// once built; pass it down by value or by pointer-to-const.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration. Populated by Load().
type Config struct {
	// DatabaseURL is a postgres connection string suitable for pgxpool.
	DatabaseURL string

	// HTTPAddr is the listen address for the HTTP server (e.g. ":8080").
	HTTPAddr string

	// SingleUserMode hardcodes user_id=1 for every request, skipping auth.
	// Flip to false once auth (Clerk) is wired up.
	SingleUserMode bool

	// SingleUserID is the user id used when SingleUserMode is true.
	SingleUserID int64
}

// Load reads environment variables and returns a Config. Returns an error
// if a required value is missing or malformed.
func Load() (*Config, error) {
	c := &Config{
		DatabaseURL:    getenv("DATABASE_URL", "postgresql://japan_concierge:dev_password@localhost:5432/japan_concierge?sslmode=disable"),
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		SingleUserMode: getenvBool("SINGLE_USER_MODE", true),
		SingleUserID:   1,
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
