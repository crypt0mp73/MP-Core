package main

import (
	"log"
	"os"

	"github.com/crypt0mp73/mp-core/config"
	"github.com/crypt0mp73/mp-core/database"
	"github.com/crypt0mp73/mp-core/web"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags)
	log.Printf("MP-CORE Panel %s starting...", version)

	cfg := config.Load()

	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if err := database.SeedAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("Failed to seed admin user: %v", err)
	}

	if err := database.SeedDefaults(); err != nil {
		log.Printf("Warning: could not seed defaults: %v", err)
	}

	server := web.NewServer(cfg, version)
	log.Printf("MP-CORE listening on :%d (base path: %s)", cfg.Port, cfg.BasePath)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
		os.Exit(1)
	}
}
