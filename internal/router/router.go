package router

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/leca/dt-cloudflare-images/internal/api"
	"github.com/leca/dt-cloudflare-images/internal/config"
	"github.com/leca/dt-cloudflare-images/internal/database"
	"github.com/leca/dt-cloudflare-images/internal/handler"
	"github.com/leca/dt-cloudflare-images/internal/storage"
)

// Server holds the application dependencies and HTTP router.
type Server struct {
	DB     database.Database
	Store  storage.Storage
	Config *config.Config
	Router chi.Router
}

// New creates a new Server with a fully configured chi router.
func New(db database.Database, store storage.Storage, cfg *config.Config) *Server {
	s := &Server{DB: db, Store: store, Config: cfg}

	h := &handler.Handler{
		DB:     db,
		Store:  store,
		Config: cfg,
	}

	r := chi.NewRouter()

	// CORS — must be before other middleware to handle preflight OPTIONS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"Content-Length", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check (no auth required).
	r.Get("/health", s.Health)

	// API routes.
	r.Route("/accounts/{account_id}/images", func(r chi.Router) {
		r.Use(api.AuthMiddleware(cfg.AuthToken))
		r.Use(api.AccountIDMiddleware)

		// V1 image endpoints.
		r.Post("/v1", h.UploadImage)
		r.Get("/v1", h.ListImages)

		// Stats must be registered before the {image_id} wildcard
		// so that /v1/stats is not interpreted as image_id="stats".
		r.Get("/v1/stats", h.GetStats)

		// Signing keys endpoints (registered before {image_id} wildcard).
		r.Get("/v1/keys", h.ListSigningKeys)
		r.Put("/v1/keys/{signing_key_name}", h.CreateSigningKey)
		r.Delete("/v1/keys/{signing_key_name}", h.DeleteSigningKey)

		// Variant endpoints.
		r.Post("/v1/variants", h.CreateVariant)
		r.Get("/v1/variants", h.ListVariants)
		r.Get("/v1/variants/{variant_id}", h.GetVariant)
		r.Patch("/v1/variants/{variant_id}", h.UpdateVariant)
		r.Delete("/v1/variants/{variant_id}", h.DeleteVariant)

		r.Get("/v1/{image_id}", h.GetImage)
		r.Patch("/v1/{image_id}", h.UpdateImage)
		r.Delete("/v1/{image_id}", h.DeleteImage)
		r.Get("/v1/{image_id}/blob", h.GetImageBlob)

		// V2 image endpoints.
		r.Get("/v2", h.ListImagesV2)
		r.Post("/v2/direct_upload", h.CreateDirectUpload)
	})

	// Direct upload endpoint (no auth required).
	r.Post("/upload/{upload_id}", h.HandleDirectUpload)

	// Image delivery endpoint (no auth required).
	r.Get("/cdn/{account_id}/{image_id}/{variant_name}", h.DeliverImage)

	s.Router = r
	return s
}

// Health returns a simple health-check response.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Health: failed to encode response: %v", err)
	}
}
