package server

import (
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
}

func LoadConfig() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
	}, nil
}
