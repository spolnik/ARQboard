package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spolnik/arqboard/migrations"
)

func TestValidateAdminUserRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		params CreateAdminUserParams
	}{
		{
			name: "missing email",
			params: CreateAdminUserParams{
				Password: "correct horse battery staple",
			},
		},
		{
			name: "short password",
			params: CreateAdminUserParams{
				Email:    "admin@example.com",
				Password: "short",
			},
		},
		{
			name: "invalid email",
			params: CreateAdminUserParams{
				Email:    "admin",
				Password: "correct horse battery staple",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminUser(tt.params)
			if err == nil {
				t.Fatal("validateAdminUser returned nil error")
			}
		})
	}
}

func TestCreateAdminUserRequiresPool(t *testing.T) {
	_, err := CreateAdminUser(context.Background(), nil, CreateAdminUserParams{
		Email:    "admin@example.com",
		Password: "correct horse battery staple",
	})
	if !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("error = %v, want ErrDatabaseUnavailable", err)
	}
}

func TestCreateAdminUserRejectsUnsupportedConnection(t *testing.T) {
	_, err := CreateAdminUser(context.Background(), &Connection{Driver: DriverUnknown}, CreateAdminUserParams{
		Email:    "admin@example.com",
		Password: "correct horse battery staple",
	})
	if !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("error = %v, want ErrDatabaseUnavailable", err)
	}
}

func TestExplicitUnsupportedDatabaseURLIsRejected(t *testing.T) {
	if driver := DriverForURL("mysql://user:pass@localhost/arqboard"); driver != DriverUnknown {
		t.Fatalf("DriverForURL returned %q, want DriverUnknown", driver)
	}

	err := MigrateUp(context.Background(), "mysql://user:pass@localhost/arqboard", nil)
	if err == nil {
		t.Fatal("MigrateUp returned nil error for unsupported database URL")
	}
}

func TestSQLiteMigrationAndAdminUser(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	migrationFS, err := migrations.ForDriver(string(DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}
	if err := MigrateUp(ctx, databaseURL, migrationFS); err != nil {
		t.Fatalf("MigrateUp returned error: %v", err)
	}

	conn, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()

	userID, err := CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:    "Admin@Example.com",
		Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}
	if userID == "" {
		t.Fatal("CreateAdminUser returned empty user id")
	}

	var email string
	var isAdmin bool
	if err := conn.SQL.QueryRowContext(ctx, "SELECT email, is_admin FROM users WHERE id = ?", userID).Scan(&email, &isAdmin); err != nil {
		t.Fatalf("query admin user: %v", err)
	}
	if email != "admin@example.com" {
		t.Fatalf("email = %q, want normalized admin@example.com", email)
	}
	if !isAdmin {
		t.Fatal("is_admin = false, want true")
	}

	_, err = CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:    "admin@example.com",
		Password: "correct horse battery staple",
	})
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("duplicate CreateAdminUser error = %v, want ErrUserExists", err)
	}
}

func TestReadinessChecker(t *testing.T) {
	ctx := context.Background()
	if err := (ReadinessChecker{}).Ready(ctx); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("nil readiness error = %v, want ErrDatabaseUnavailable", err)
	}
	if err := (ReadinessChecker{Conn: &Connection{Driver: DriverSQLite}}).Ready(ctx); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("missing sqlite readiness error = %v, want ErrDatabaseUnavailable", err)
	}
	if err := (ReadinessChecker{Conn: &Connection{Driver: DriverUnknown}}).Ready(ctx); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("unknown readiness error = %v, want ErrDatabaseUnavailable", err)
	}

	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	migrationFS, err := migrations.ForDriver(string(DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}
	if err := MigrateUp(ctx, databaseURL, migrationFS); err != nil {
		t.Fatalf("MigrateUp returned error: %v", err)
	}
	conn, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()

	if err := (ReadinessChecker{Conn: conn}).Ready(ctx); err != nil {
		t.Fatalf("sqlite readiness returned error: %v", err)
	}
}

func TestDriverAndSQLiteHelpers(t *testing.T) {
	if DriverForURL("postgres://user:pass@localhost/db") != DriverPostgres {
		t.Fatal("postgres URL did not select postgres driver")
	}
	if DriverForURL("postgresql://user:pass@localhost/db") != DriverPostgres {
		t.Fatal("postgresql URL did not select postgres driver")
	}
	if DriverForURL("sqlite://data/arqboard.db") != DriverSQLite {
		t.Fatal("sqlite URL did not select sqlite driver")
	}
	if DriverForURL("data/arqboard.db") != DriverSQLite {
		t.Fatal("plain path did not select sqlite driver")
	}
	if sqlDriverName(DriverPostgres) != "pgx" || sqlDriverName(DriverSQLite) != "sqlite" || sqlDriverName(DriverUnknown) != "" {
		t.Fatal("sqlDriverName returned unexpected values")
	}
	if gooseDialect(DriverPostgres) != "postgres" || gooseDialect(DriverSQLite) != "sqlite3" || gooseDialect(DriverUnknown) != "" {
		t.Fatal("gooseDialect returned unexpected values")
	}
	if sqlDSN("postgres://user:pass@localhost/db") != "postgres://user:pass@localhost/db" {
		t.Fatal("sqlDSN changed postgres URL")
	}
	if sqliteDSN("sqlite::memory:") != ":memory:" {
		t.Fatal("sqlite in-memory DSN changed")
	}
	if sqlitePath("") != "data/arqboard.db" {
		t.Fatal("empty sqlite path did not use default data path")
	}
	if sqlitePath("sqlite:///C:/tmp/arqboard.db") != filepath.FromSlash("C:/tmp/arqboard.db") {
		t.Fatal("windows sqlite path was not normalized")
	}
	if err := ensureSQLiteDir("postgres://user:pass@localhost/db"); err != nil {
		t.Fatalf("ensureSQLiteDir for postgres returned error: %v", err)
	}
	if err := ensureSQLiteDir("sqlite::memory:"); err != nil {
		t.Fatalf("ensureSQLiteDir for memory returned error: %v", err)
	}
	if !isUniqueViolation(errors.New("UNIQUE constraint failed: users.email")) {
		t.Fatal("sqlite unique error was not detected")
	}
	if isUniqueViolation(errors.New("ordinary failure")) {
		t.Fatal("ordinary error was treated as unique violation")
	}
}
