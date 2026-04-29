package app

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsUsageAndUnknownCommands(t *testing.T) {
	ctx := context.Background()

	var stderr bytes.Buffer
	if code := Run(ctx, nil, nil, &stderr); code != 2 {
		t.Fatalf("exported Run empty args code = %d, want 2", code)
	}

	stderr.Reset()
	if code := run(ctx, nil, emptyLookup, nil, &stderr); code != 2 {
		t.Fatalf("empty args code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage: arqboard") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}

	stderr.Reset()
	if code := run(ctx, []string{"wat"}, emptyLookup, nil, &stderr); code != 2 {
		t.Fatalf("unknown command code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "wat"`) {
		t.Fatalf("stderr = %q, want unknown command", stderr.String())
	}
}

func TestRunMigrateAndAdminCreateUserWithSQLite(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	lookup := mapLookup(map[string]string{
		"DATABASE_URL": databaseURL,
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run(ctx, []string{"migrate"}, lookup, &stdout, &stderr); code != 0 {
		t.Fatalf("migrate code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "migrations applied") {
		t.Fatalf("stdout = %q, want migration message", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code := run(ctx, []string{
		"admin",
		"create-user",
		"--email", "admin@example.com",
		"--password", "correct horse battery staple",
		"--name", "Admin",
	}, lookup, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("admin code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created admin user") {
		t.Fatalf("stdout = %q, want created user message", stdout.String())
	}
}

func TestRunAdminRejectsInvalidUsage(t *testing.T) {
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"admin"}, emptyLookup, nil, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "admin create-user") {
		t.Fatalf("stderr = %q, want admin usage", stderr.String())
	}
}

func TestRunServeRejectsInvalidConfigBeforeListening(t *testing.T) {
	lookup := mapLookup(map[string]string{
		"APP_ENV":        "production",
		"APP_URL":        "http://localhost:8080",
		"SESSION_SECRET": "0123456789abcdef0123456789abcdef",
	})

	var stderr bytes.Buffer
	code := run(context.Background(), []string{"serve"}, lookup, nil, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL") {
		t.Fatalf("stderr = %q, want database config error", stderr.String())
	}
}

func TestDatabaseURLFromEnv(t *testing.T) {
	got, err := databaseURLFromEnv(mapLookup(map[string]string{
		"DATABASE_URL": " sqlite://custom.db ",
	}))
	if err != nil {
		t.Fatalf("databaseURLFromEnv returned error: %v", err)
	}
	if got != "sqlite://custom.db" {
		t.Fatalf("database URL = %q, want trimmed explicit URL", got)
	}

	got, err = databaseURLFromEnv(emptyLookup)
	if err != nil {
		t.Fatalf("databaseURLFromEnv default returned error: %v", err)
	}
	if got != "sqlite://data/arqboard.db" {
		t.Fatalf("database URL = %q, want local default", got)
	}

	_, err = databaseURLFromEnv(mapLookup(map[string]string{"APP_ENV": "production"}))
	if err == nil {
		t.Fatal("databaseURLFromEnv returned nil error for missing production database URL")
	}
}

func emptyLookup(string) (string, bool) {
	return "", false
}

func mapLookup(values map[string]string) envLookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
