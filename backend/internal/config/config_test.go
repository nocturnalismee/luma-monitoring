package config

import (
	"strings"
	"testing"
)

// validEnv returns the minimal set of required variables with valid values.
func validEnv() map[string]string {
	return map[string]string{
		"LUMA_BASE_URL":   "https://monitor.example.com",
		// 32 random bytes, base64. Test-only fixture value, never a real secret.
		"LUMA_SECRET_KEY": "tVf1l52i+8oAixhMMr/Z6c49hWVF8JdWRwHvZuVYoN4=",
	}
}

func TestLoadValid(t *testing.T) {
	cfg, err := Load(validEnv())
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://monitor.example.com" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if len(cfg.SecretKey) != 32 {
		t.Errorf("len(SecretKey) = %d, want 32", len(cfg.SecretKey))
	}
	if cfg.DBPath != "/data/luma.db" {
		t.Errorf("DBPath default = %q", cfg.DBPath)
	}
	if cfg.Listen != ":8080" {
		t.Errorf("Listen default = %q", cfg.Listen)
	}
	if cfg.IngestMaxBody != 5242880 || cfg.IngestMaxDecompressed != 20971520 {
		t.Errorf("ingest limits = %d/%d", cfg.IngestMaxBody, cfg.IngestMaxDecompressed)
	}
	if cfg.SessionTTL.String() != "12h0m0s" {
		t.Errorf("SessionTTL default = %q", cfg.SessionTTL)
	}
	if len(cfg.TrustedProxies) != 4 {
		t.Errorf("TrustedProxies default = %v", cfg.TrustedProxies)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(env map[string]string)
		wantSub string
	}{
		{"missing base url", func(e map[string]string) { delete(e, "LUMA_BASE_URL") }, "LUMA_BASE_URL"},
		{"base url not absolute", func(e map[string]string) { e["LUMA_BASE_URL"] = "monitor.example.com" }, "LUMA_BASE_URL"},
		{"base url wrong scheme", func(e map[string]string) { e["LUMA_BASE_URL"] = "ftp://monitor.example.com" }, "scheme"},
		{"base url trailing slash", func(e map[string]string) { e["LUMA_BASE_URL"] = "https://monitor.example.com/" }, "slash"},
		{"missing secret key", func(e map[string]string) { delete(e, "LUMA_SECRET_KEY") }, "LUMA_SECRET_KEY"},
		{"secret key not base64", func(e map[string]string) { e["LUMA_SECRET_KEY"] = "not base64!!" }, "base64"},
		{"secret key wrong length", func(e map[string]string) { e["LUMA_SECRET_KEY"] = "aGVsbG8=" }, "32 bytes"},
		{"bad session ttl", func(e map[string]string) { e["LUMA_SESSION_TTL"] = "twelve hours" }, "LUMA_SESSION_TTL"},
		{"bad log level", func(e map[string]string) { e["LUMA_LOG_LEVEL"] = "verbose" }, "LUMA_LOG_LEVEL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			tc.mutate(env)
			_, err := Load(env)
			if err == nil {
				t.Fatalf("Load() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not mention %q", err, tc.wantSub)
			}
			// Errors must never echo secret values back.
			if strings.Contains(err.Error(), env["LUMA_SECRET_KEY"]) {
				t.Errorf("error leaks secret value: %v", err)
			}
		})
	}
}

func TestSecretErrorDoesNotLeakValue(t *testing.T) {
	env := validEnv()
	env["LUMA_SECRET_KEY"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 33 bytes
	_, err := Load(env)
	if err == nil {
		t.Fatal("expected error for 33-byte key")
	}
	if strings.Contains(err.Error(), "AAAA") {
		t.Errorf("error leaks key material: %v", err)
	}
}
