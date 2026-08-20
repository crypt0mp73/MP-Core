package main

import (
	"log"

	"github.com/mp-core/panel/config"
	"github.com/mp-core/panel/database"
	"github.com/mp-core/panel/web"
)

func main() {
	log.SetFlags(log.LstdFlags)
	log.Println("MP-CORE Panel starting...")

	// Load configuration (env vars with sane defaults)
	cfg := config.Load()

	// Initialize SQLite database + run migrations
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Seed default admin on first run
	if err := database.SeedAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("Failed to seed admin user: %v", err)
	}

	// Start web server
	server := web.NewServer(cfg)
	log.Printf("MP-CORE listening on :%d", cfg.Port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
