package config

import (
	"os"
	"strconv"
)

// Config holds all panel configuration.
// Values come from environment variables with safe defaults.
type Config struct {
	Port      int
	BasePath  string
	DBPath    string
	AdminUser string
	AdminPass string
}

// Load reads configuration from environment variables,
// falling back to defaults suitable for local testing.
func Load() *Config {
	cfg := &Config{
		Port:      2053,
		BasePath:  "/",
		DBPath:    "mp-core.db", // local by default so `go run .` works without root
		AdminUser: "admin",
		AdminPass: "admin",
	}

	if p := os.Getenv("MP_CORE_PORT"); p != "" {
		if port, err := strconv.Atoi(p); err == nil && port > 0 && port < 65536 {
			cfg.Port = port
		}
	}
	if bp := os.Getenv("MP_CORE_BASE_PATH"); bp != "" {
		cfg.BasePath = bp
	}
	if db := os.Getenv("MP_CORE_DB_PATH"); db != "" {
		cfg.DBPath = db
	}
	if u := os.Getenv("MP_CORE_ADMIN_USER"); u != "" {
		cfg.AdminUser = u
	}
	if pw := os.Getenv("MP_CORE_ADMIN_PASS"); pw != "" {
		cfg.AdminPass = pw
	}
	return cfg
}
