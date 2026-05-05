package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestAccessStoreFiltersTeamsBoardsAndWikiByMembership(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	boardStore, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()
	teamStore := TeamStore{Conn: boardStore.Conn}
	accessStore := AccessStore{Conn: boardStore.Conn}

	adminID, err := CreateAdminUser(ctx, boardStore.Conn, CreateAdminUserParams{
		Email:       "admin@example.com",
		Password:    "correct horse battery admin",
		DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("CreateAdminUser returned error: %v", err)
	}
	member, err := teamStore.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:       "member@example.com",
		DisplayName: "Member",
		Password:    "correct horse battery member",
		Role:        "member",
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceMember returned error: %v", err)
	}

	defaultBoard, err := boardStore.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	otherTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Mobile Team"})
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	otherBoard, err := singleBoardForTeam(ctx, boardStore, otherTeam.ID)
	if err != nil {
		t.Fatalf("singleBoardForTeam returned error: %v", err)
	}
	otherPage, err := boardStore.CreateWikiPage(ctx, CreateWikiPageParams{
		BoardID:      otherBoard.ID,
		Title:        "Mobile Runbook",
		BodyMarkdown: "# Mobile",
	})
	if err != nil {
		t.Fatalf("CreateWikiPage other board returned error: %v", err)
	}

	adminUser := User{ID: adminID, Email: "admin@example.com", DisplayName: "Admin", IsAdmin: true}
	memberUser := User{ID: member.UserID, Email: member.Email, DisplayName: member.DisplayName}

	adminTeams, err := accessStore.ListTeamsForUser(ctx, adminUser)
	if err != nil {
		t.Fatalf("ListTeamsForUser admin returned error: %v", err)
	}
	if len(adminTeams) != 2 {
		t.Fatalf("admin teams = %#v, want both teams", adminTeams)
	}
	memberTeams, err := accessStore.ListTeamsForUser(ctx, memberUser)
	if err != nil {
		t.Fatalf("ListTeamsForUser member returned error: %v", err)
	}
	if len(memberTeams) != 1 || memberTeams[0].ID != defaultBoard.TeamID {
		t.Fatalf("member teams = %#v, want only default team", memberTeams)
	}

	adminBoards, err := accessStore.ListBoardsForUser(ctx, adminUser)
	if err != nil {
		t.Fatalf("ListBoardsForUser admin returned error: %v", err)
	}
	if len(adminBoards) != 2 {
		t.Fatalf("admin boards = %#v, want both boards", adminBoards)
	}
	memberBoards, err := accessStore.ListBoardsForUser(ctx, memberUser)
	if err != nil {
		t.Fatalf("ListBoardsForUser member returned error: %v", err)
	}
	if len(memberBoards) != 1 || memberBoards[0].ID != defaultBoard.ID {
		t.Fatalf("member boards = %#v, want only default board", memberBoards)
	}

	memberPages, err := accessStore.ListWikiPagesForUser(ctx, memberUser)
	if err != nil {
		t.Fatalf("ListWikiPagesForUser member returned error: %v", err)
	}
	if containsWikiPage(memberPages, otherPage.ID) {
		t.Fatalf("member pages include inaccessible other team page: %#v", memberPages)
	}
}

func TestAccessStoreAuthorizesRolesForTeamScopedResources(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	boardStore, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()
	teamStore := TeamStore{Conn: boardStore.Conn}
	accessStore := AccessStore{Conn: boardStore.Conn}

	viewer, err := teamStore.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:       "viewer@example.com",
		DisplayName: "Viewer",
		Password:    "correct horse battery viewer",
		Role:        "viewer",
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceMember viewer returned error: %v", err)
	}
	member, err := teamStore.CreateWorkspaceMember(ctx, CreateWorkspaceMemberParams{
		Email:       "member@example.com",
		DisplayName: "Member",
		Password:    "correct horse battery member",
		Role:        "member",
	})
	if err != nil {
		t.Fatalf("CreateWorkspaceMember member returned error: %v", err)
	}

	board, err := boardStore.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	card := findColumn(t, board, "Planned").Cards[0]
	page := board.WikiPages[0]
	sprint := PlanningDashboard{}
	sprint, err = boardStore.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard returned error: %v", err)
	}

	viewerUser := User{ID: viewer.UserID, Email: viewer.Email, DisplayName: viewer.DisplayName}
	memberUser := User{ID: member.UserID, Email: member.Email, DisplayName: member.DisplayName}

	if err := accessStore.AuthorizeBoard(ctx, viewerUser, board.ID, AccessRead); err != nil {
		t.Fatalf("viewer read board error = %v", err)
	}
	if err := accessStore.AuthorizeCard(ctx, viewerUser, card.ID, AccessWrite); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer write card error = %v, want ErrForbidden", err)
	}
	if err := accessStore.AuthorizeWikiPage(ctx, viewerUser, page.ID, AccessWrite); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer write wiki error = %v, want ErrForbidden", err)
	}
	if err := accessStore.AuthorizeCard(ctx, memberUser, card.ID, AccessWrite); err != nil {
		t.Fatalf("member write card error = %v", err)
	}
	if err := accessStore.AuthorizeSprint(ctx, memberUser, sprint.ActiveSprint.Sprint.ID, AccessManage); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member manage sprint error = %v, want ErrForbidden", err)
	}
	if err := accessStore.AuthorizeTeam(ctx, memberUser, "missing", AccessRead); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing team access error = %v, want ErrNotFound", err)
	}
	if err := (AccessStore{}).AuthorizeTeam(ctx, memberUser, board.TeamID, AccessRead); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("missing access database error = %v, want ErrDatabaseUnavailable", err)
	}
}

func containsWikiPage(pages []WikiPage, pageID string) bool {
	for _, page := range pages {
		if page.ID == pageID {
			return true
		}
	}
	return false
}
