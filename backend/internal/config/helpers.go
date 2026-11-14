package config

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// base64Decode accepts standard and URL-safe base64 with or without padding.
func base64Decode(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func valueOr(env map[string]string, key, def string) string {
	if v, ok := env[key]; ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// defaultInt64 parses a positive integer; invalid input falls back to 0 so the
// caller's validation reports it. Used only for defaults documented in
// DEPLOYMENT.md section 1.
func defaultInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
