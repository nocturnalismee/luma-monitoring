// Package config loads and validates the process configuration from
// environment variables, as specified in docs/DEPLOYMENT.md section 1.
package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Config holds every runtime setting read from the environment at startup.
type Config struct {
	BaseURL     string        // LUMA_BASE_URL, no trailing slash
	SecretKey   []byte        // LUMA_SECRET_KEY, exactly 32 raw bytes
	DBPath      string        // LUMA_DB_PATH
	Listen      string        // LUMA_LISTEN
	TrustedProxies []string   // LUMA_TRUSTED_PROXIES, comma-separated CIDRs
	IngestMaxBody int64       // LUMA_INGEST_MAX_BODY
	IngestMaxDecompressed int64 // LUMA_INGEST_MAX_DECOMPRESSED
	SessionTTL  time.Duration // LUMA_SESSION_TTL
	LogLevel    string        // LUMA_LOG_LEVEL
}

// Load reads the environment described by DEPLOYMENT.md section 1 and returns a
// validated Config. It never includes secret values in its error messages.
func Load(env map[string]string) (*Config, error) {
	cfg := &Config{
		DBPath:              valueOr(env, "LUMA_DB_PATH", "/data/luma.db"),
		Listen:              valueOr(env, "LUMA_LISTEN", ":8080"),
		IngestMaxBody:       defaultInt64(valueOr(env, "LUMA_INGEST_MAX_BODY", "5242880")),
		IngestMaxDecompressed: defaultInt64(valueOr(env, "LUMA_INGEST_MAX_DECOMPRESSED", "20971520")),
		LogLevel:            strings.ToLower(valueOr(env, "LUMA_LOG_LEVEL", "info")),
	}

	base, err := parseBaseURL(env["LUMA_BASE_URL"])
	if err != nil {
		return nil, err
	}
	cfg.BaseURL = base

	key, err := parseSecretKey(env["LUMA_SECRET_KEY"])
	if err != nil {
		return nil, err
	}
	cfg.SecretKey = key

	ttl, err := time.ParseDuration(valueOr(env, "LUMA_SESSION_TTL", "12h"))
	if err != nil {
		return nil, fmt.Errorf("invalid LUMA_SESSION_TTL: %w", err)
	}
	cfg.SessionTTL = ttl

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return nil, fmt.Errorf("invalid LUMA_LOG_LEVEL %q: want debug, info, warn or error", cfg.LogLevel)
	}

	for _, cidr := range splitList(valueOr(env, "LUMA_TRUSTED_PROXIES",
		"127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")) {
		cfg.TrustedProxies = append(cfg.TrustedProxies, cidr)
	}

	return cfg, nil
}

// parseBaseURL validates LUMA_BASE_URL: absolute http(s) URL without trailing slash.
func parseBaseURL(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("missing required variable LUMA_BASE_URL")
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return "", fmt.Errorf("invalid LUMA_BASE_URL: must be an absolute URL with scheme and host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid LUMA_BASE_URL: scheme must be http or https")
	}
	if strings.HasSuffix(raw, "/") {
		return "", fmt.Errorf("invalid LUMA_BASE_URL: must not end with a slash")
	}
	return raw, nil
}

// parseSecretKey validates LUMA_SECRET_KEY decodes to exactly 32 bytes.
func parseSecretKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("missing required variable LUMA_SECRET_KEY")
	}
	key, err := base64Decode(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid LUMA_SECRET_KEY: must be base64 of 32 random bytes")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid LUMA_SECRET_KEY: must decode to 32 bytes, got %d", len(key))
	}
	return key, nil
}
