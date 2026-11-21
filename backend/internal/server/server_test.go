package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nocturnalismee/luma-monitoring/backend/internal/config"
)

func testCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load(map[string]string{
		"LUMA_BASE_URL":   "https://monitor.example.com",
		"LUMA_SECRET_KEY": "tVf1l52i+8oAixhMMr/Z6c49hWVF8JdWRwHvZuVYoN4=",
	})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestHealthzOK(t *testing.T) {
	h := NewHandler(testCfg(t), testLogger(), nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
	}
	if body["status"] != "ok" || body["version"] == "" {
		t.Errorf("body = %v, want status ok and non-empty version", body)
	}
	if rec.Header().Get(HeaderRequestID) == "" {
		t.Error("missing X-Request-Id header")
	}
}

func TestHealthzDegraded(t *testing.T) {
	h := NewHandler(testCfg(t), testLogger(), func(context.Context) error {
		return errors.New("db down")
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
	}
	if body["status"] != "degraded" || body["reason"] != "db" {
		t.Errorf("body = %v, want degraded/db", body)
	}
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	h := NewHandler(testCfg(t), testLogger(), nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	id1 := rec.Header().Get(HeaderRequestID)
	if len(id1) != 16 {
		t.Errorf("generated request id %q, want 16 hex chars", id1)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(HeaderRequestID, "abc-123")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get(HeaderRequestID); got != "abc-123" {
		t.Errorf("inbound request id = %q, want echoed abc-123", got)
	}

	// Header with control characters must be replaced, not echoed (log injection).
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(HeaderRequestID, "bad\nid")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get(HeaderRequestID); got == "bad\nid" || len(got) != 16 {
		t.Errorf("invalid inbound id not replaced: %q", got)
	}
}

func TestRecoverTurnsPanicInto500(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}), RequestID, Recover(log), Logging(log))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestLoggingEmitsStructuredLine(t *testing.T) {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}), RequestID, Recover(log), Logging(log))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/teapot", nil))

	line := buf.String()
	for _, want := range []string{`"msg":"request"`, `"path":"/teapot"`, `"status":418`, `"request_id"`} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q missing %s", line, want)
		}
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	cfg := testCfg(t)
	cfg.Listen = "127.0.0.1:0"
	// Listen on a fixed free port instead: :0 is not supported by Run's Addr.
	cfg.Listen = "127.0.0.1:18080"

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, testLogger(), NewHandler(cfg, testLogger(), nil)) }()

	deadline := time.Now().Add(3 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		resp, e := http.Get("http://127.0.0.1:18080/healthz")
		if e == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				err = nil
				break
			}
		}
		err = e
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never became ready: %v", err)
	}

	cancel()
	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("Run returned %v after shutdown, want nil", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
	_ = os.Stdout
}
