package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shyamsundaravssb/synth/internal/server/handlers"
)

func NewRouter(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", handlers.HealthHandler(pool))
	return mux
}
