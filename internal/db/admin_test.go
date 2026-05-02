package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

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
				Email:       "admin@example.com",
				Password:    "short",
				DisplayName: "Admin",
			},
		},
		{
			name: "invalid email",
			params: CreateAdminUserParams{
				Email:       "admin",
				Password:    "correct horse battery staple",
				DisplayName: "Admin",
			},
		},
		{
			name: "missing display name",
			params: CreateAdminUserParams{
				Email:    "admin@example.com",
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
		Email:       "admin@example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
	})
	if !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("error = %v, want ErrDatabaseUnavailable", err)
	}
}

func TestCreateAdminUserRejectsUnsupportedConnection(t *testing.T) {
	_, err := CreateAdminUser(context.Background(), &Connection{Driver: DriverUnknown}, CreateAdminUserParams{
		Email:       "admin@example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
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
	conn := openMigratedSQLite(t)
	defer conn.Close()

	userID, err := CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:       "Admin@Example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
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
		Email:       "admin@example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
	})
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("duplicate CreateAdminUser error = %v, want ErrUserExists", err)
	}
}

func TestSQLiteMigrationBackfillsEmailDisplayNames(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	sqliteMigrations, err := migrations.ForDriver(string(DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}
	priorMigrations := fstest.MapFS{
		"00001_initial_schema.sql":               mustReadMigration(t, sqliteMigrations, "00001_initial_schema.sql"),
		"00002_sprint_planning.sql":              mustReadMigration(t, sqliteMigrations, "00002_sprint_planning.sql"),
		"00003_card_due_dates.sql":               mustReadMigration(t, sqliteMigrations, "00003_card_due_dates.sql"),
		"00004_remaining_due_label_backfill.sql": mustReadMigration(t, sqliteMigrations, "00004_remaining_due_label_backfill.sql"),
		"00005_board_scoped_sprints.sql":         mustReadMigration(t, sqliteMigrations, "00005_board_scoped_sprints.sql"),
		"00006_labels_and_assignees.sql":         mustReadMigration(t, sqliteMigrations, "00006_labels_and_assignees.sql"),
	}
	if err := MigrateUp(ctx, databaseURL, priorMigrations); err != nil {
		t.Fatalf("prior MigrateUp returned error: %v", err)
	}

	conn, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	_, err = conn.SQL.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, is_admin)
		VALUES ('user-1', 'admin@example.com', 'hash', 'admin@example.com', 1)
	`)
	conn.Close()
	if err != nil {
		t.Fatalf("seed email display name returned error: %v", err)
	}

	if err := MigrateUp(ctx, databaseURL, sqliteMigrations); err != nil {
		t.Fatalf("full MigrateUp returned error: %v", err)
	}

	conn, err = Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	defer conn.Close()
	var displayName string
	if err := conn.SQL.QueryRowContext(ctx, "SELECT display_name FROM users WHERE id = 'user-1'").Scan(&displayName); err != nil {
		t.Fatalf("query display name returned error: %v", err)
	}
	if displayName != "admin" {
		t.Fatalf("displayName = %q, want admin", displayName)
	}
}

func TestAuthStoreLoginCurrentUserAndLogout(t *testing.T) {
	ctx := context.Background()
	conn := openMigratedSQLite(t)
	defer conn.Close()

	userID, err := CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:       "Admin@Example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin User",
	})
	if err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}

	store := AuthStore{Conn: conn}
	session, err := store.Login(ctx, LoginParams{
		Email:    " admin@example.com ",
		Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if session.Token == "" {
		t.Fatal("Login returned empty token")
	}
	if !session.ExpiresAt.After(time.Now()) {
		t.Fatalf("ExpiresAt = %v, want future time", session.ExpiresAt)
	}
	if session.User.ID != userID {
		t.Fatalf("session.User.ID = %q, want %q", session.User.ID, userID)
	}
	if session.User.Email != "admin@example.com" {
		t.Fatalf("session.User.Email = %q, want normalized admin@example.com", session.User.Email)
	}
	if session.User.DisplayName != "Admin User" {
		t.Fatalf("session.User.DisplayName = %q, want Admin User", session.User.DisplayName)
	}
	if !session.User.IsAdmin {
		t.Fatal("session.User.IsAdmin = false, want true")
	}

	user, err := store.CurrentUser(ctx, session.Token)
	if err != nil {
		t.Fatalf("CurrentUser returned error: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("CurrentUser ID = %q, want %q", user.ID, userID)
	}

	if err := store.Logout(ctx, session.Token); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	_, err = store.CurrentUser(ctx, session.Token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("CurrentUser after logout error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthStoreRejectsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	conn := openMigratedSQLite(t)
	defer conn.Close()

	if _, err := CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:       "admin@example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
	}); err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}

	store := AuthStore{Conn: conn}
	tests := []LoginParams{
		{Email: "admin@example.com", Password: "wrong password"},
		{Email: "missing@example.com", Password: "correct horse battery staple"},
		{Email: "", Password: "correct horse battery staple"},
		{Email: "admin@example.com", Password: ""},
	}

	for _, params := range tests {
		_, err := store.Login(ctx, params)
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login(%#v) error = %v, want ErrInvalidCredentials", params, err)
		}
	}
}

func TestAuthStorePreservesPasswordWhitespace(t *testing.T) {
	ctx := context.Background()
	conn := openMigratedSQLite(t)
	defer conn.Close()

	if _, err := CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:       "admin@example.com",
		Password:    "  correct horse battery staple  ",
		DisplayName: "Admin",
	}); err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}

	store := AuthStore{Conn: conn}
	if _, err := store.Login(ctx, LoginParams{
		Email:    "admin@example.com",
		Password: "  correct horse battery staple  ",
	}); err != nil {
		t.Fatalf("Login with exact password returned error: %v", err)
	}
	if _, err := store.Login(ctx, LoginParams{
		Email:    "admin@example.com",
		Password: "correct horse battery staple",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login with trimmed password error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthStoreExpiresSessions(t *testing.T) {
	ctx := context.Background()
	conn := openMigratedSQLite(t)
	defer conn.Close()

	if _, err := CreateAdminUser(ctx, conn, CreateAdminUserParams{
		Email:       "admin@example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
	}); err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}

	store := AuthStore{Conn: conn, SessionTTL: -time.Hour}
	session, err := store.Login(ctx, LoginParams{
		Email:    "admin@example.com",
		Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	_, err = store.CurrentUser(ctx, session.Token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("CurrentUser for expired session error = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthStoreRequiresDatabase(t *testing.T) {
	ctx := context.Background()
	store := AuthStore{}

	if _, err := store.Login(ctx, LoginParams{Email: "admin@example.com", Password: "correct horse battery staple"}); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("Login error = %v, want ErrDatabaseUnavailable", err)
	}
	if _, err := store.CurrentUser(ctx, "token"); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("CurrentUser error = %v, want ErrDatabaseUnavailable", err)
	}
	if err := store.Logout(ctx, "token"); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("Logout error = %v, want ErrDatabaseUnavailable", err)
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

func TestSQLiteSchemaUsesUUIDPrimaryKeysForApplicationTables(t *testing.T) {
	conn := openMigratedSQLite(t)
	defer conn.Close()

	tables := []string{
		"users",
		"sessions",
		"workspaces",
		"workspace_members",
		"boards",
		"board_members",
		"columns",
		"cards",
		"card_comments",
		"activity_events",
		"wiki_pages",
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			rows, err := conn.SQL.Query("PRAGMA table_info(" + table + ")")
			if err != nil {
				t.Fatalf("PRAGMA table_info returned error: %v", err)
			}
			defer rows.Close()

			var primaryKeyColumn string
			var primaryKeyType string
			var primaryKeyCount int
			for rows.Next() {
				var cid int
				var name string
				var columnType string
				var notNull int
				var defaultValue any
				var primaryKeyPosition int
				if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKeyPosition); err != nil {
					t.Fatalf("table info scan returned error: %v", err)
				}
				if primaryKeyPosition > 0 {
					primaryKeyCount++
					primaryKeyColumn = name
					primaryKeyType = columnType
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("table info rows returned error: %v", err)
			}
			if primaryKeyCount != 1 {
				t.Fatalf("primary key column count = %d, want 1", primaryKeyCount)
			}
			if primaryKeyColumn != "id" {
				t.Fatalf("primary key column = %q, want id", primaryKeyColumn)
			}
			if primaryKeyType != "uuid" {
				t.Fatalf("primary key type = %q, want uuid", primaryKeyType)
			}
		})
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

func openMigratedSQLite(t *testing.T) *Connection {
	t.Helper()

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
	return conn
}
