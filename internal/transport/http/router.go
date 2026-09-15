// Package http wires the HTTP transport: routing, middleware, and the
// handlers that don't yet belong to a specific domain (health, readiness).
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	httpmw "github.com/sbezhuk/beebase-common/authmw"
	"github.com/sbezhuk/beebase-common/internalauth"
	inspectionhttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http/inspection"
)

// NewRouter builds the root HTTP handler for the service.
func NewRouter(
	log *slog.Logger,
	db *pgxpool.Pool,
	inspectionHandler *inspectionhttp.Handler,
	tokenParser httpmw.AccessTokenParser,
	internalTokens ...string,
) http.Handler {
	internalToken := ""
	if len(internalTokens) > 0 {
		internalToken = internalTokens[0]
	}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestLogger(log))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", HealthHandler)
	r.Get("/ready", ReadyHandler(db))
	r.With(internalauth.RequireAuth(internalToken)).Get("/internal/api/v1/inspections/{id}/exists", existsHandler(db, "inspections", true))

	r.Group(func(r chi.Router) {
		r.Use(httpmw.RequireAuth(tokenParser))

		r.Route("/api/v1/inspections", func(r chi.Router) {
			r.Post("/", inspectionHandler.Create)
			r.Get("/", inspectionHandler.List)
			// Internal primitive: called by hive-service (to filter hive
			// listings by "needs inspection") and statistics-service (to
			// report the Dashboard's Needs Attention section), never
			// directly by an end-user client. Registered as a static
			// sibling of "/{inspectionID}" rather than under it, so it
			// can never be confused with an inspection id.
			r.Get("/hive-status", inspectionHandler.HiveInspectionStatus)
			r.Get("/{inspectionId}", inspectionHandler.Get)
			r.Put("/{inspectionId}", inspectionHandler.Update)
			r.Delete("/{inspectionId}", inspectionHandler.Delete)
		})

		// Listing scoped to one hive stays a separate endpoint from the
		// flat "list everything I own" above - it's what hive/inspection
		// detail screens actually want, and statistics-service uses the
		// flat form instead of fanning this out per hive.
		r.Get("/api/v1/hives/{hiveId}/inspections", inspectionHandler.ListByHive)
		// Internal cascade primitive: called by hive-service when it
		// deletes a hive, forwarding the caller's own access token.
		r.Delete("/api/v1/hives/{hiveId}/inspections", inspectionHandler.DeleteByHive)
	})

	return r
}
func existsHandler(db *pgxpool.Pool, table string, soft bool) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(chi.URLParam(r, "id"))
		if e != nil {
			http.NotFound(w, r)
			return
		}
		q := "SELECT EXISTS(SELECT 1 FROM " + table + " WHERE id=$1"
		if soft {
			q += " AND deleted_at IS NULL"
		}
		q += ")"
		var ok bool
		if e = db.QueryRow(r.Context(), q, id).Scan(&ok); e != nil {
			http.Error(w, "", 500)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
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
