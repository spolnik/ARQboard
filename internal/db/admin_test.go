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
}
