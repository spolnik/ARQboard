package config

import "testing"

func TestLoadUsesLocalDefaultsOutsideProduction(t *testing.T) {
	cfg, err := Load(func(key string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.DatabaseURL != "sqlite://data/arqboard.db" {
		t.Fatalf("DatabaseURL = %q, want local sqlite default", cfg.DatabaseURL)
	}
	if cfg.AppURL != "http://localhost:8080" {
		t.Fatalf("AppURL = %q, want http://localhost:8080", cfg.AppURL)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
}

func TestLoadProductionRequiresDatabaseURL(t *testing.T) {
	_, err := Load(func(key string) (string, bool) {
		values := map[string]string{
			"APP_ENV":        "production",
			"APP_URL":        "http://localhost:8080",
			"SESSION_SECRET": "0123456789abcdef0123456789abcdef",
		}
		value, ok := values[key]
		return value, ok
	})
	if err == nil {
		t.Fatal("Load returned nil error, want missing DATABASE_URL error")
	}
}

func TestLoadRequiresStrongSessionSecret(t *testing.T) {
	_, err := Load(func(key string) (string, bool) {
		values := map[string]string{
			"APP_ENV":        "production",
			"DATABASE_URL":   "postgres://user:pass@localhost:5432/arqboard?sslmode=disable",
			"APP_URL":        "http://localhost:8080",
			"SESSION_SECRET": "too-short",
		}
		value, ok := values[key]
		return value, ok
	})
	if err == nil {
		t.Fatal("Load returned nil error, want weak SESSION_SECRET error")
	}
}
