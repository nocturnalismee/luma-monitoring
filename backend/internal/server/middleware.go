// Package server wires the HTTP listener, middleware chain and the base
// handlers that exist before the API tasks (T-020+): /healthz only.
// Middleware order per CONVENTIONS.md 3.4: request ID -> recover -> logging.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type contextKey int

const requestIDKey contextKey = iota

// HeaderRequestID is the correlation header read from proxies and set on every
// response (CONVENTIONS.md 3.4).
const HeaderRequestID = "X-Request-Id"

// RequestID assigns a short random hex ID to every request, reusing an
// inbound X-Request-Id when present.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

// Recover turns panics into 500s so one bad handler cannot kill the process
// (CONVENTIONS.md 3.2: handlers are protected by a recover middleware).
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "error", rec, "path", r.URL.Path,
						"request_id", RequestIDFrom(r.Context()))
					writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter records the response status and byte count for access logging.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	n, err := sw.ResponseWriter.Write(b)
	sw.bytes += n
	return n, err
}

// Logging writes one structured line per request. It never logs headers,
// cookies or bodies (CONVENTIONS.md 3.3).
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"bytes", sw.bytes,
				"dur_ms", time.Since(start).Milliseconds(),
				"remote", clientIP(r),
				"request_id", RequestIDFrom(r.Context()),
			)
		})
	}
}

// Chain applies middlewares outside-in: Chain(h, m1, m2) runs m1 then m2 then h.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// validRequestID accepts only short printable tokens to prevent log injection
// via a crafted inbound header.
func validRequestID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}
