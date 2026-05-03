package db

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/spolnik/arqboard/migrations"
)

func TestSprintPlanningDashboardLifecycle(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	otherBoard, err := store.CreateBoard(ctx, CreateBoardParams{Name: "Release Train"})
	if err != nil {
		t.Fatalf("CreateBoard returned error: %v", err)
	}
	if otherBoard.TeamID != board.TeamID {
		t.Fatalf("otherBoard.TeamID = %q, want same default team %q", otherBoard.TeamID, board.TeamID)
	}

	initial, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard returned error: %v", err)
	}
	if initial.ActiveSprint == nil {
		t.Fatal("initial active sprint is nil, want existing board cards mapped to a current sprint")
	}
	activeCard := initial.ActiveSprint.Cards[0]

	otherCard, err := store.CreateCard(ctx, CreateCardParams{
		ColumnID: otherBoard.Columns[0].ID,
		Title:    "Coordinate release train backlog",
	})
	if err != nil {
		t.Fatalf("CreateCard other board returned error: %v", err)
	}
	withTwoBoards, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard with two boards returned error: %v", err)
	}
	if withTwoBoards.ActiveSprint == nil || !containsCard(withTwoBoards.ActiveSprint.Cards, otherCard.ID) {
		t.Fatalf("team active sprint cards = %#v, want card from second board", withTwoBoards.ActiveSprint)
	}
	if len(withTwoBoards.Boards) != 2 {
		t.Fatalf("team planning boards = %d, want both team boards", len(withTwoBoards.Boards))
	}

	sprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:   board.TeamID,
		Name:     "Sprint 2026-05 Platform",
		Goal:     "Ship planning foundations",
		StartsOn: "2026-05-04",
		EndsOn:   "2026-05-15",
	})
	if err != nil {
		t.Fatalf("CreateSprint returned error: %v", err)
	}
	if sprint.Status != "planned" || sprint.Name != "Sprint 2026-05 Platform" || sprint.TeamID != board.TeamID {
		t.Fatalf("created sprint = %#v, want planned named sprint", sprint)
	}

	teamStore := TeamStore{Conn: store.Conn}
	otherTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Mobile Team"})
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	otherTeamBoard, err := store.CreateBoard(ctx, CreateBoardParams{Name: "Mobile Board", TeamID: otherTeam.ID})
	if err != nil {
		t.Fatalf("CreateBoard other team returned error: %v", err)
	}
	otherSprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:  otherTeam.ID,
		BoardID: otherTeamBoard.ID,
		Name:    "Sprint 2026-05 Platform",
	})
	if err != nil {
		t.Fatalf("CreateSprint for other team returned error: %v", err)
	}
	if _, err := store.StartSprint(ctx, otherSprint.ID); err != nil {
		t.Fatalf("StartSprint for other team returned error: %v", err)
	}

	completed, err := store.CompleteSprint(ctx, CompleteSprintParams{
		SprintID: initial.ActiveSprint.Sprint.ID,
		Rollover: []SprintRolloverDecision{
			{CardID: activeCard.ID, SprintID: sprint.ID},
		},
	})
	if err != nil {
		t.Fatalf("CompleteSprint returned error: %v", err)
	}
	if completed.Status != "completed" || completed.CompletedAt == "" {
		t.Fatalf("completed sprint = %#v, want completed with completed time", completed)
	}

	afterComplete, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard after complete returned error: %v", err)
	}
	if !containsCard(afterComplete.PlannedSprints[0].Cards, activeCard.ID) {
		t.Fatal("selected rollover card was not moved to the next sprint")
	}
	if len(afterComplete.Backlog) == 0 {
		t.Fatal("remaining active sprint cards should return to backlog by default")
	}

	assigned, err := store.AssignCardToSprint(ctx, AssignCardToSprintParams{
		CardID:   afterComplete.Backlog[0].ID,
		SprintID: sprint.ID,
	})
	if err != nil {
		t.Fatalf("AssignCardToSprint returned error: %v", err)
	}
	if assigned.SprintID != sprint.ID {
		t.Fatalf("assigned card sprint = %q, want %q", assigned.SprintID, sprint.ID)
	}

	afterAssign, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard after assign returned error: %v", err)
	}
	if containsCard(afterAssign.Backlog, assigned.ID) {
		t.Fatal("assigned card still appears in backlog")
	}
	if len(afterAssign.PlannedSprints) != 1 || len(afterAssign.PlannedSprints[0].Cards) < 2 {
		t.Fatalf("planned sprint cards = %#v, want assigned cards", afterAssign.PlannedSprints)
	}

	active, err := store.StartSprint(ctx, sprint.ID)
	if err != nil {
		t.Fatalf("StartSprint returned error: %v", err)
	}
	if active.Status != "active" || active.StartedAt == "" {
		t.Fatalf("active sprint = %#v, want active with started time", active)
	}

	afterStart, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard after start returned error: %v", err)
	}
	if afterStart.ActiveSprint == nil || afterStart.ActiveSprint.Sprint.ID != sprint.ID {
		t.Fatalf("active dashboard sprint = %#v, want sprint %s", afterStart.ActiveSprint, sprint.ID)
	}

	otherDashboard, err := store.GetPlanningDashboard(ctx, otherTeam.ID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard other team returned error: %v", err)
	}
	if otherDashboard.ActiveSprint == nil || otherDashboard.ActiveSprint.Sprint.ID != otherSprint.ID {
		t.Fatalf("other team active sprint = %#v, want %s", otherDashboard.ActiveSprint, otherSprint.ID)
	}
}

func TestSprintPlanningValidation(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	card := findColumn(t, board, "Planned").Cards[0]

	if _, err := store.CreateSprint(ctx, CreateSprintParams{}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSprint without name error = %v, want ErrValidation", err)
	}
	if _, err := (BoardStore{}).GetPlanningDashboard(ctx, "team"); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("GetPlanningDashboard without database error = %v, want ErrDatabaseUnavailable", err)
	}
	if _, err := (BoardStore{}).CreateSprint(ctx, CreateSprintParams{TeamID: "team", Name: "Sprint"}); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("CreateSprint without database error = %v, want ErrDatabaseUnavailable", err)
	}
	if _, err := (BoardStore{}).StartSprint(ctx, "sprint"); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("StartSprint without database error = %v, want ErrDatabaseUnavailable", err)
	}
	if _, err := (BoardStore{}).CompleteSprint(ctx, CompleteSprintParams{SprintID: "sprint"}); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("CompleteSprint without database error = %v, want ErrDatabaseUnavailable", err)
	}
	if _, err := (BoardStore{}).AssignCardToSprint(ctx, AssignCardToSprintParams{CardID: "card"}); !errors.Is(err, ErrDatabaseUnavailable) {
		t.Fatalf("AssignCardToSprint without database error = %v, want ErrDatabaseUnavailable", err)
	}
	if _, err := store.AssignCardToSprint(ctx, AssignCardToSprintParams{CardID: card.ID, SprintID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AssignCardToSprint missing sprint error = %v, want ErrNotFound", err)
	}

	first, err := store.CreateSprint(ctx, CreateSprintParams{BoardID: board.ID, Name: "Sprint One"})
	if err != nil {
		t.Fatalf("CreateSprint first returned error: %v", err)
	}
	if _, err := store.CreateSprint(ctx, CreateSprintParams{BoardID: board.ID, Name: "Sprint One"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSprint duplicate name error = %v, want ErrValidation", err)
	}
	second, err := store.CreateSprint(ctx, CreateSprintParams{BoardID: board.ID, Name: "Sprint Two"})
	if err != nil {
		t.Fatalf("CreateSprint second returned error: %v", err)
	}
	if active, err := store.GetPlanningDashboard(ctx, board.ID); err != nil {
		t.Fatalf("GetPlanningDashboard returned error: %v", err)
	} else if active.ActiveSprint != nil {
		if _, err := store.CompleteSprint(ctx, CompleteSprintParams{SprintID: active.ActiveSprint.Sprint.ID}); err != nil {
			t.Fatalf("CompleteSprint initial returned error: %v", err)
		}
	}
	if _, err := store.StartSprint(ctx, first.ID); err != nil {
		t.Fatalf("StartSprint first returned error: %v", err)
	}
	if _, err := store.StartSprint(ctx, second.ID); !errors.Is(err, ErrValidation) {
		t.Fatalf("StartSprint second active error = %v, want ErrValidation", err)
	}
	if _, err := store.CompleteSprint(ctx, CompleteSprintParams{SprintID: second.ID}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CompleteSprint planned error = %v, want ErrValidation", err)
	}
}

func TestSprintPlanningResolvesScopesAndRejectsTeamMismatches(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	defaultDashboard, err := store.GetPlanningDashboard(ctx, "")
	if err != nil {
		t.Fatalf("GetPlanningDashboard default scope returned error: %v", err)
	}
	if defaultDashboard.TeamID != board.TeamID {
		t.Fatalf("default dashboard team = %q, want %q", defaultDashboard.TeamID, board.TeamID)
	}
	workspaceID, err := loadBoardWorkspace(ctx, store.Conn.SQL, store.Conn.Driver, board.ID)
	if err != nil {
		t.Fatalf("loadBoardWorkspace returned error: %v", err)
	}
	if workspaceID != board.WorkspaceID {
		t.Fatalf("loadBoardWorkspace = %q, want %q", workspaceID, board.WorkspaceID)
	}
	activeSprintID, found, err := activeSprintIDForBoard(ctx, store.Conn.SQL, store.Conn.Driver, board.ID)
	if err != nil {
		t.Fatalf("activeSprintIDForBoard returned error: %v", err)
	}
	if !found || activeSprintID == "" {
		t.Fatalf("activeSprintIDForBoard = (%q, %v), want active sprint", activeSprintID, found)
	}
	nextCurrentName, err := uniqueSprintName(ctx, store.Conn.SQL, store.Conn.Driver, board.TeamID, "Current sprint")
	if err != nil {
		t.Fatalf("uniqueSprintName returned error: %v", err)
	}
	if nextCurrentName != "Current sprint 2" {
		t.Fatalf("uniqueSprintName = %q, want Current sprint 2", nextCurrentName)
	}
	defaultName, err := uniqueSprintName(ctx, store.Conn.SQL, store.Conn.Driver, board.TeamID, "")
	if err != nil {
		t.Fatalf("uniqueSprintName empty base returned error: %v", err)
	}
	if defaultName != "Sprint" {
		t.Fatalf("uniqueSprintName empty base = %q, want Sprint", defaultName)
	}
	if _, err := store.GetPlanningDashboard(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetPlanningDashboard missing scope error = %v, want ErrNotFound", err)
	}
	if _, err := store.StartSprint(ctx, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("StartSprint empty id error = %v, want ErrValidation", err)
	}
	if _, err := store.CompleteSprint(ctx, CompleteSprintParams{}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CompleteSprint empty id error = %v, want ErrValidation", err)
	}
	if _, err := store.AssignCardToSprint(ctx, AssignCardToSprintParams{}); !errors.Is(err, ErrValidation) {
		t.Fatalf("AssignCardToSprint empty card error = %v, want ErrValidation", err)
	}

	teamStore := TeamStore{Conn: store.Conn}
	otherTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Mobile Team"})
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	if _, err := store.CreateSprint(ctx, CreateSprintParams{TeamID: otherTeam.ID, Name: "No board yet"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSprint for team without board error = %v, want ErrValidation", err)
	}
	otherBoard, err := store.CreateBoard(ctx, CreateBoardParams{Name: "Mobile Board", TeamID: otherTeam.ID})
	if err != nil {
		t.Fatalf("CreateBoard other team returned error: %v", err)
	}
	if _, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:  board.TeamID,
		BoardID: otherBoard.ID,
		Name:    "Wrong team board",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSprint mismatched team board error = %v, want ErrValidation", err)
	}

	otherSprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:  otherTeam.ID,
		BoardID: otherBoard.ID,
		Name:    "Mobile Sprint",
	})
	if err != nil {
		t.Fatalf("CreateSprint other team returned error: %v", err)
	}
	card := findColumn(t, board, "Planned").Cards[0]
	if _, err := store.AssignCardToSprint(ctx, AssignCardToSprintParams{
		CardID:   card.ID,
		SprintID: otherSprint.ID,
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("AssignCardToSprint cross-team error = %v, want ErrValidation", err)
	}
}

func TestSprintPlanningRolloverValidationAndCompletedAssignments(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	dashboard, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard returned error: %v", err)
	}
	if dashboard.ActiveSprint == nil || len(dashboard.ActiveSprint.Cards) == 0 {
		t.Fatalf("active sprint = %#v, want seeded current sprint with cards", dashboard.ActiveSprint)
	}
	activeSprint := dashboard.ActiveSprint.Sprint
	activeCard := dashboard.ActiveSprint.Cards[0]
	nextSprint, err := store.CreateSprint(ctx, CreateSprintParams{TeamID: board.TeamID, Name: "Next Sprint"})
	if err != nil {
		t.Fatalf("CreateSprint next returned error: %v", err)
	}

	invalidRollovers := []struct {
		name     string
		rollover []SprintRolloverDecision
	}{
		{name: "missing card id", rollover: []SprintRolloverDecision{{SprintID: nextSprint.ID}}},
		{name: "unknown active card", rollover: []SprintRolloverDecision{{CardID: "missing", SprintID: nextSprint.ID}}},
		{name: "duplicate card", rollover: []SprintRolloverDecision{{CardID: activeCard.ID, SprintID: nextSprint.ID}, {CardID: activeCard.ID}}},
		{name: "active target sprint", rollover: []SprintRolloverDecision{{CardID: activeCard.ID, SprintID: activeSprint.ID}}},
	}
	for _, tt := range invalidRollovers {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.CompleteSprint(ctx, CompleteSprintParams{
				SprintID: activeSprint.ID,
				Rollover: tt.rollover,
			}); !errors.Is(err, ErrValidation) {
				t.Fatalf("CompleteSprint rollover error = %v, want ErrValidation", err)
			}
		})
	}

	teamStore := TeamStore{Conn: store.Conn}
	otherTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Infrastructure Team"})
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	otherBoard, err := store.CreateBoard(ctx, CreateBoardParams{Name: "Infrastructure Board", TeamID: otherTeam.ID})
	if err != nil {
		t.Fatalf("CreateBoard other team returned error: %v", err)
	}
	otherSprint, err := store.CreateSprint(ctx, CreateSprintParams{TeamID: otherTeam.ID, BoardID: otherBoard.ID, Name: "Infra Sprint"})
	if err != nil {
		t.Fatalf("CreateSprint other team returned error: %v", err)
	}
	if _, err := store.CompleteSprint(ctx, CompleteSprintParams{
		SprintID: activeSprint.ID,
		Rollover: []SprintRolloverDecision{{CardID: activeCard.ID, SprintID: otherSprint.ID}},
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CompleteSprint cross-team rollover error = %v, want ErrValidation", err)
	}

	if _, err := store.CompleteSprint(ctx, CompleteSprintParams{
		SprintID: activeSprint.ID,
		Rollover: []SprintRolloverDecision{{CardID: activeCard.ID, SprintID: nextSprint.ID}},
	}); err != nil {
		t.Fatalf("CompleteSprint valid rollover returned error: %v", err)
	}
	if _, err := store.StartSprint(ctx, nextSprint.ID); err != nil {
		t.Fatalf("StartSprint next returned error: %v", err)
	}
	if _, err := store.CompleteSprint(ctx, CompleteSprintParams{SprintID: nextSprint.ID}); err != nil {
		t.Fatalf("CompleteSprint next returned error: %v", err)
	}
	if _, err := store.AssignCardToSprint(ctx, AssignCardToSprintParams{
		CardID:   activeCard.ID,
		SprintID: nextSprint.ID,
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("AssignCardToSprint completed sprint error = %v, want ErrValidation", err)
	}
}

func TestSprintPlanningMigrationRepairsAppliedVersionWithoutSprintTable(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	sqliteMigrations, err := migrations.ForDriver(string(DriverSQLite))
	if err != nil {
		t.Fatalf("ForDriver returned error: %v", err)
	}

	phaseOne := fstest.MapFS{
		"00001_initial_schema.sql": mustReadMigration(t, sqliteMigrations, "00001_initial_schema.sql"),
		"00002_sprint_planning.sql": {
			Data: []byte("-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n"),
		},
		"00003_card_due_dates.sql":               mustReadMigration(t, sqliteMigrations, "00003_card_due_dates.sql"),
		"00004_remaining_due_label_backfill.sql": mustReadMigration(t, sqliteMigrations, "00004_remaining_due_label_backfill.sql"),
	}
	if err := MigrateUp(ctx, databaseURL, phaseOne); err != nil {
		t.Fatalf("phase one MigrateUp returned error: %v", err)
	}
	if err := MigrateUp(ctx, databaseURL, sqliteMigrations); err != nil {
		t.Fatalf("repair MigrateUp returned error: %v", err)
	}

	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()
	board, err := store.GetDefaultBoard(ctx)
	if err != nil {
		t.Fatalf("GetDefaultBoard returned error: %v", err)
	}
	dashboard, err := store.GetPlanningDashboard(ctx, board.ID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard returned error: %v", err)
	}
	if dashboard.ActiveSprint == nil || len(dashboard.ActiveSprint.Cards) == 0 {
		t.Fatalf("dashboard.ActiveSprint = %#v, want repaired current sprint with existing cards", dashboard.ActiveSprint)
	}
}

func containsCard(cards []BoardCard, cardID string) bool {
	for _, card := range cards {
		if card.ID == cardID {
			return true
		}
	}
	return false
}

func mustReadMigration(t *testing.T, migrationFS fs.FS, name string) *fstest.MapFile {
	t.Helper()

	data, err := fs.ReadFile(migrationFS, name)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", name, err)
	}
	return &fstest.MapFile{Data: data}
}
