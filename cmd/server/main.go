package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/router"
	"github.com/leca/dt-cloudflare-images/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	db, err := database.NewSQLiteDB(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	store := storage.NewFileSystem(cfg.StoragePath)

	srv := router.New(db, store, cfg)

	slog.Info("starting server", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Router); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
