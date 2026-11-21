package config

import (
	"strings"
	"testing"
)

// TestLoadFromEnv covers the os.Getenv adapter used by cmd/luma (T-002):
// a missing required variable must produce a clear error naming the variable.
func TestLoadFromEnv(t *testing.T) {
	valid := map[string]string{
		"LUMA_BASE_URL":   "https://monitor.example.com",
		"LUMA_SECRET_KEY": "tVf1l52i+8oAixhMMr/Z6c49hWVF8JdWRwHvZuVYoN4=",
	}
	cfg, err := LoadFromEnv(func(k string) string { return valid[k] })
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if cfg.BaseURL != valid["LUMA_BASE_URL"] || len(cfg.SecretKey) != 32 {
		t.Errorf("unexpected cfg: %+v", cfg)
	}

	if _, err := LoadFromEnv(func(string) string { return "" }); err == nil {
		t.Fatal("expected error when LUMA_BASE_URL is missing")
	} else if want := "LUMA_BASE_URL"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q should mention %s", err, want)
	}
}
