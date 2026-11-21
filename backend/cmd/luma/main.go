// Command luma is the LUMA Monitoring server: a single binary that serves the
// agent ingest endpoints, the REST API and the embedded frontend.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nocturnalismee/luma-monitoring/backend/internal/config"
	"github.com/nocturnalismee/luma-monitoring/backend/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "luma:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadFromEnv(os.Getenv)
	if err != nil {
		// Error strings from config never contain secret values
		// (DEPLOYMENT.md section 1, CONVENTIONS.md section 3.3).
		return err
	}

	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler := server.NewHandler(cfg, log, nil) // DB check wired in T-003.
	return server.Run(ctx, cfg, log, handler)
}

// newLogger builds the JSON slog handler required by DEPLOYMENT.md section 1
// (structured logs; LUMA_LOG_LEVEL selects the level).
func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
