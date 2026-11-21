package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/nocturnalismee/luma-monitoring/backend/internal/config"
)

// Version is the build version reported by GET /healthz (API.md section 2).
// ASSUMPTION(T-002): fixed string until the release pipeline stamps it.
var Version = "0.0.0-dev"

// DBCheck reports database health for /healthz. The real implementation is a
// `SELECT 1` added with task T-003 (internal/db); until then main injects an
// always-healthy check so the endpoint contract in API.md holds.
type DBCheck func(ctx context.Context) error

func healthyDB(context.Context) error { return nil }

// NewHandler builds the root handler: middleware chain (request ID -> recover
// -> logging, CONVENTIONS.md 3.4) over a ServeMux that currently serves only
// GET /healthz. Ingest and API routes are registered by later tasks.
func NewHandler(cfg *config.Config, log *slog.Logger, db DBCheck) http.Handler {
	if db == nil {
		db = healthyDB
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler(db, cfg.SessionTTL))
	return Chain(mux, RequestID, Recover(log), Logging(log))
}

// healthzHandler answers per API.md section 2:
// 200 {"status":"ok","version":...} or 503 {"status":"degraded","reason":"db"}.
func healthzHandler(db DBCheck, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		if err := db(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable,
				map[string]string{"status": "degraded", "reason": "db"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
	}
}

// Run starts the HTTP server and shuts it down gracefully on ctx cancellation
// (SIGINT/SIGTERM in main, TASKS.md T-002). Timeouts per CONVENTIONS.md 3.4.
// Optional pre-bound listeners let tests use port :0; production passes none.
func Run(ctx context.Context, cfg *config.Config, log *slog.Logger, handler http.Handler, listeners ...net.Listener) error {
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", srv.Addr, "version", Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutdown requested")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("server stopped")
	return <-errCh
}
