package db

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/spolnik/arqboard/migrations"
)

func TestTeamStoreListsCreatesAndUpdatesWorkspaceMembers(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	boardStore, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	adminID, err := CreateAdminUser(ctx, boardStore.Conn, CreateAdminUserParams{
		Email:       "admin@example.com",
		Password:    "correct horse battery staple",
		DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}

	store := TeamStore{Conn: boardStore.Conn}
	members, err := store.ListWorkspaceMembers(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaceMembers returned error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("len(members) = %d, want seeded admin owner", len(members))
	}
	if members[0].UserID != adminID || members[0].Role != "owner" || !members[0].IsAdmin {
		t.Fatalf("admin member = %#v, want owner admin", members[0])
	}

	created, err := store.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:       " Developer@Example.COM ",
		DisplayName: "Developer",
		Password:    "correct horse battery member",
		Role:        "member",
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceMember returned error: %v", err)
	}
	if created.Email != "developer@example.com" || created.DisplayName != "Developer" || created.Role != "member" {
		t.Fatalf("created member = %#v, want normalized member", created)
	}

	session, err := (AuthStore{Conn: boardStore.Conn}).Login(ctx, LoginParams{
		Email:    "developer@example.com",
		Password: "correct horse battery member",
	})
	if err != nil {
		t.Fatalf("created member login returned error: %v", err)
	}
	if session.User.IsAdmin {
		t.Fatal("created member is admin, want workspace member without global admin")
	}

	updated, err := store.UpdateWorkspaceMember(ctx, UpdateWorkspaceMemberParams{
		MemberID: created.ID,
		Role:     "viewer",
	})
	if err != nil {
		t.Fatalf("UpdateWorkspaceMember returned error: %v", err)
	}
	if updated.Role != "viewer" || updated.UserID != created.UserID {
		t.Fatalf("updated member = %#v, want viewer with same user", updated)
	}
}

func TestTeamStoreValidatesWorkspaceMembers(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	boardStore, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	store := TeamStore{Conn: boardStore.Conn}
	if _, err := store.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:       "person@example.com",
		DisplayName: "Person",
		Password:    "correct horse battery member",
		Role:        "manager",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid create role error = %v, want ErrValidation", err)
	}
	if _, err := store.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:       "person@example.com",
		DisplayName: "Person",
		Role:        "member",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing password error = %v, want ErrValidation", err)
	}
	if _, err := store.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:    "person@example.com",
		Password: "correct horse battery member",
		Role:     "member",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing display name error = %v, want ErrValidation", err)
	}
	if _, err := store.UpdateWorkspaceMember(ctx, UpdateWorkspaceMemberParams{
		MemberID: "missing",
		Role:     "viewer",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update error = %v, want ErrNotFound", err)
	}
	if _, err := (TeamStore{}).ListWorkspaceMembers(ctx); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("ListWorkspaceMembers without database error = %v, want ErrDatabaseUnavailable", err)
	}
}

func TestSQLiteMigrationRepairsWorkspaceMembersWithoutID(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	sqliteMigrations, err := migrations.ForDriver(string(DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}

	legacyMigrations := fstest.MapFS{
		"00001_initial_schema.sql":               legacyWorkspaceMembersMigration(t, sqliteMigrations),
		"00002_sprint_planning.sql":              mustReadMigration(t, sqliteMigrations, "00002_sprint_planning.sql"),
		"00003_card_due_dates.sql":               mustReadMigration(t, sqliteMigrations, "00003_card_due_dates.sql"),
		"00004_remaining_due_label_backfill.sql": mustReadMigration(t, sqliteMigrations, "00004_remaining_due_label_backfill.sql"),
		"00005_board_scoped_sprints.sql":         mustReadMigration(t, sqliteMigrations, "00005_board_scoped_sprints.sql"),
	}
	if err := MigrateUp(ctx, databaseURL, legacyMigrations); err != nil {
		t.Fatalf("legacy MigrateUp returned error: %v", err)
	}

	sqlDB, err := sql.Open(sqlDriverName(DriverSQLite), sqlDSN(databaseURL))
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	_, err = sqlDB.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, is_admin)
		VALUES ('user-1', 'admin@example.com', 'hash', 'Admin', 1);
		INSERT INTO workspaces (id, name, slug)
		VALUES ('workspace-1', 'Default', 'default');
		INSERT INTO workspace_members (workspace_id, user_id, role)
		VALUES ('workspace-1', 'user-1', 'owner');
	`)
	if closeErr := sqlDB.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("seed legacy workspace member returned error: %v", err)
	}

	if err := MigrateUp(ctx, databaseURL, sqliteMigrations); err != nil {
		t.Fatalf("repair MigrateUp returned error: %v", err)
	}
	conn, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()

	members, err := (TeamStore{Conn: conn}).ListWorkspaceMembers(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaceMembers returned error: %v", err)
	}
	if len(members) != 1 || members[0].ID == "" || members[0].UserID != "user-1" || members[0].Role != "owner" {
		t.Fatalf("members = %#v, want repaired owner with generated member ID", members)
	}
}

func legacyWorkspaceMembersMigration(t *testing.T, migrationFS fs.FS) *fstest.MapFile {
	t.Helper()

	migration := string(mustReadMigration(t, migrationFS, "00001_initial_schema.sql").Data)
	migration = strings.ReplaceAll(migration, "\r\n", "\n")
	current := "CREATE TABLE workspace_members (\n    id uuid PRIMARY KEY NOT NULL,\n"
	legacy := "CREATE TABLE workspace_members (\n"
	if !strings.Contains(migration, current) {
		t.Fatal("initial sqlite migration does not contain expected workspace_members id column")
	}
	return &fstest.MapFile{Data: []byte(strings.Replace(migration, current, legacy, 1))}
}
