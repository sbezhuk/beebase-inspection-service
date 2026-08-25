// Package http wires the HTTP transport: routing, middleware, and the
// handlers that don't yet belong to a specific domain (health, readiness).
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	httpmw "github.com/sbezhuk/beebase-common/authmw"
	inspectionhttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http/inspection"
)

// NewRouter builds the root HTTP handler for the service.
func NewRouter(
	log *slog.Logger,
	db *pgxpool.Pool,
	inspectionHandler *inspectionhttp.Handler,
	tokenParser httpmw.AccessTokenParser,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestLogger(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", HealthHandler)
	r.Get("/ready", ReadyHandler(db))

	r.Group(func(r chi.Router) {
		r.Use(httpmw.RequireAuth(tokenParser))

		r.Route("/api/v1/inspections", func(r chi.Router) {
			r.Post("/", inspectionHandler.Create)
			r.Get("/{inspectionID}", inspectionHandler.Get)
			r.Put("/{inspectionID}", inspectionHandler.Update)
			r.Delete("/{inspectionID}", inspectionHandler.Delete)
		})

		// Nested under its parent hive, since listing inspections is
		// always "for a hive", not a flat "list everything I own" like
		// apiaries/hives support.
		r.Get("/api/v1/hives/{hiveID}/inspections", inspectionHandler.ListByHive)
	})

	return r
}

// requestLogger logs each request's method, path, status, and duration
// through slog instead of chi's default stdlib logger.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			log.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
