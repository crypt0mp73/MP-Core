package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO required)

	"golang.org/x/crypto/bcrypt"
)

// DB is the shared database handle.
var DB *sql.DB

// Init opens the SQLite database, creates its directory if needed,
// and runs schema migrations.
func Init(dbPath string) error {
	// Ensure the parent directory exists (e.g. /etc/mp-core)
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create db directory: %w", err)
		}
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return migrate()
}

// migrate creates all tables if they do not exist.
func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE TABLE IF NOT EXISTS inbounds (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		remark          TEXT,
		port            INTEGER,
		protocol        TEXT,
		settings        TEXT,
		stream_settings TEXT,
		enabled         INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS clients (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		inbound_id  INTEGER,
		email       TEXT,
		uuid        TEXT,
		enabled     INTEGER DEFAULT 1,
		expiry_time INTEGER DEFAULT 0,
		total_gb    INTEGER DEFAULT 0,
		up          INTEGER DEFAULT 0,
		down        INTEGER DEFAULT 0
	);
	`
	_, err := DB.Exec(schema)
	return err
}

// SeedAdmin creates the admin user on first run only.
func SeedAdmin(username, password string) error {
	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = DB.Exec(
		"INSERT INTO users (username, password_hash) VALUES (?, ?)",
		username, string(hash),
	)
	return err
}

// CheckLogin verifies credentials. Returns (true, nil) on success.
func CheckLogin(username, password string) (bool, error) {
	var hash string
	err := DB.QueryRow(
		"SELECT password_hash FROM users WHERE username = ?", username,
	).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false, nil
	}
	return true, nil
}
