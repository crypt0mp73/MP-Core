package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
)

type Config struct {
	Port      int
	BasePath  string
	DBPath    string
	AdminUser string
	AdminPass string
	Domain    string
}

func Load() *Config {
	cfg := &Config{
		Port:      2053,
		BasePath:  "/",
		DBPath:    "mp-core.db",
		AdminUser: "admin",
		AdminPass: "admin",
		Domain:    "",
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
	if d := os.Getenv("MP_CORE_DOMAIN"); d != "" {
		cfg.Domain = d
	}

	return cfg
}

func RandomToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}
