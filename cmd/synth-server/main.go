package main

import (
	"context"
	"log"
	"os"

	"github.com/shyamsundaravssb/synth/internal/server"
)

func main() {
	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.DatabaseURL == "" {
		log.Fatalf("DATABASE_URL environment variable is required")
	}

	ctx := context.Background()

	pool, err := server.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := server.RunMigrations(ctx, pool, server.MigrationsFS); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	log.Println("server foundation ready")
	os.Exit(0)
}
