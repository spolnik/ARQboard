package app

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
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

func TestRunServeAppliesMigrationsAndStopsOnContextCancel(t *testing.T) {
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	lookup := mapLookup(map[string]string{
		"DATABASE_URL": databaseURL,
		"HTTP_ADDR":    "127.0.0.1:0",
		"WEB_DIST_DIR": t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())

	var stderr lockedBuffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"serve"}, lookup, nil, &stderr)
	}()

	waitForLogEntry(t, &stderr, "starting server")
	cancel()

	var code int
	select {
	case code = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not stop after context cancellation")
	}
	if code != 0 {
		t.Fatalf("serve code = %d, stderr = %q", code, stderr.String())
	}
	waitForBoardsTable(t, databaseURL)
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForLogEntry(t *testing.T, buffer *lockedBuffer, text string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if strings.Contains(buffer.String(), text) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("log entry %q was not written before timeout; stderr = %q", text, buffer.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func waitForBoardsTable(t *testing.T, databaseURL string) {
	t.Helper()

	sqlDB, err := sql.Open("sqlite", strings.TrimPrefix(databaseURL, "sqlite://"))
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer sqlDB.Close()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var boardTable string
		err := sqlDB.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'boards'").Scan(&boardTable)
		if err == nil && boardTable == "boards" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("boards table was not created before timeout; last error = %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestRunServeReportsListenErrors(t *testing.T) {
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	lookup := mapLookup(map[string]string{
		"DATABASE_URL": databaseURL,
		"HTTP_ADDR":    "127.0.0.1:-1",
		"WEB_DIST_DIR": t.TempDir(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	if code := run(ctx, []string{"serve"}, lookup, nil, &stderr); code != 1 {
		t.Fatalf("serve code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "serve:") {
		t.Fatalf("stderr = %q, want serve error", stderr.String())
	}
}

func TestRunRejectsInvalidServeAndMCPFlags(t *testing.T) {
	tests := []string{"serve", "mcp"}
	for _, command := range tests {
		t.Run(command, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(context.Background(), []string{command, "--unknown"}, emptyLookup, nil, &stderr); code != 1 {
				t.Fatalf("code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "flag provided but not defined") {
				t.Fatalf("stderr = %q, want flag error", stderr.String())
			}
		})
	}
}

func TestRunMCPRejectsInvalidConfigBeforeConnecting(t *testing.T) {
	lookup := mapLookup(map[string]string{"APP_ENV": "production"})

	var stderr bytes.Buffer
	code := run(context.Background(), []string{"mcp"}, lookup, nil, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "DATABASE_URL") {
		t.Fatalf("stderr = %q, want database config error", stderr.String())
	}
}

func TestPrepareDatabaseRunsMigrationsIdempotently(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))

	if err := prepareDatabase(ctx, databaseURL); err != nil {
		t.Fatalf("prepareDatabase returned error: %v", err)
	}
	if err := prepareDatabase(ctx, databaseURL); err != nil {
		t.Fatalf("second prepareDatabase returned error: %v", err)
	}

	sqlDB, err := sql.Open("sqlite", strings.TrimPrefix(databaseURL, "sqlite://"))
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer sqlDB.Close()

	var boardTable string
	if err := sqlDB.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'boards'").Scan(&boardTable); err != nil {
		t.Fatalf("boards table lookup returned error: %v", err)
	}
	if boardTable != "boards" {
		t.Fatalf("board table = %q, want boards", boardTable)
	}

	var versions int
	if err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE is_applied = 1").Scan(&versions); err != nil {
		t.Fatalf("goose version lookup returned error: %v", err)
	}
	if versions == 0 {
		t.Fatal("goose version table has no applied migrations")
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
