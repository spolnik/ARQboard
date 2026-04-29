package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
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
		Email:    "person@example.com",
		Password: "correct horse battery member",
		Role:     "manager",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid create role error = %v, want ErrValidation", err)
	}
	if _, err := store.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email: "person@example.com",
		Role:  "member",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("missing password error = %v, want ErrValidation", err)
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
