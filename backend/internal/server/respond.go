package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFrom returns the request ID stored by the RequestID middleware, or
// an empty string when the handler runs outside the chain (e.g. in tests).
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// writeJSON is the single JSON response helper required by CONVENTIONS.md 3.4.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP returns the remote IP of a request without parsing untrusted
// X-Forwarded-For: proxy handling per LUMA_TRUSTED_PROXIES arrives with the
// ingest tasks (T-013).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
