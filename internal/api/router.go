package api

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/docker"
)

// NewRouter creates and configures the HTTP router with all routes.
func NewRouter(db *database.DB, dockerClient *docker.Client) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(corsMiddleware)

	// Health check (no auth)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "orbex",
		})
	})

	// Handlers
	authHandler := NewAuthHandler(db)
	jobHandler := NewJobHandler(db)
	runHandler := NewRunHandler(db, dockerClient)

	// Webhook trigger (no auth — uses webhook token in URL)
	r.Post("/api/v1/webhooks/{token}/trigger", runHandler.WebhookTrigger)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {

		// Public auth routes (no API key needed)
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/api-keys", authHandler.GenerateBootstrapKey)

		// Protected routes (require API key)
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(db))

			// API key management
			r.Post("/auth/keys", authHandler.CreateAPIKey)

			// Jobs CRUD
			r.Post("/jobs", jobHandler.Create)
			r.Get("/jobs", jobHandler.List)
			r.Get("/jobs/{jobID}", jobHandler.Get)
			r.Delete("/jobs/{jobID}", jobHandler.Delete)

			// Job runs
			r.Post("/jobs/{jobID}/run", runHandler.TriggerRun)
			r.Post("/jobs/{jobID}/webhook", jobHandler.GenerateWebhookToken)
			r.Get("/jobs/{jobID}/runs", runHandler.ListRuns)

			// Run management
			r.Get("/runs/{runID}", runHandler.GetRun)
			r.Post("/runs/{runID}/pause", runHandler.PauseRun)
			r.Post("/runs/{runID}/resume", runHandler.ResumeRun)
			r.Post("/runs/{runID}/kill", runHandler.KillRun)
			r.Get("/runs/{runID}/logs", runHandler.GetRunLogs)
		})
	})

	// Log registered routes
	chi.Walk(r, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("  %s %s", method, route)
		return nil
	})

	return r
}

// corsMiddleware adds CORS headers for development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
