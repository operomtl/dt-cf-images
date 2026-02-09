package handler

import (
	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/storage"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	DB     database.Database
	Store  storage.Storage
	Config *config.Config
}
