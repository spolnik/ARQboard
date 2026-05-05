package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/spolnik/arqboard/migrations"
)

func TestTeamOwnedBoardAndWeeklySprintDefaults(t *testing.T) {
	ctx := context.Background()
	databaseURL := "sqlite://" + filepath.ToSlash(filepath.Join(t.TempDir(), "arqboard.db"))
	store, cleanup := setupBoardStore(t, ctx, databaseURL)
	defer cleanup()

	teamStore := TeamStore{Conn: store.Conn}
	team, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Godforge"})
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	boards, err := store.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards returned error: %v", err)
	}
	teamBoards := boardsForTeam(boards, team.ID)
	if len(teamBoards) != 1 || teamBoards[0].Name != "Godforge" {
		t.Fatalf("team boards = %#v, want one Godforge board", teamBoards)
	}
	if _, err := store.CreateBoard(ctx, CreateBoardParams{Name: "Extra Godforge Board", TeamID: team.ID}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateBoard second team board error = %v, want ErrValidation", err)
	}

	weekSprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:   team.ID,
		StartsOn: "2026-05-04",
	})
	if err != nil {
		t.Fatalf("CreateSprint weekly returned error: %v", err)
	}
	if weekSprint.Name != "Sprint 2026-W19" || weekSprint.StartsOn != "2026-05-04" || weekSprint.EndsOn != "2026-05-10" {
		t.Fatalf("week sprint = %#v, want ISO week 19 window", weekSprint)
	}

	currentName, currentStartsOn, currentEndsOn := weeklySprintWindow(time.Now())

	autoTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Auto Week Team"})
	if err != nil {
		t.Fatalf("CreateTeam auto returned error: %v", err)
	}
	autoDashboard, err := store.GetPlanningDashboard(ctx, autoTeam.ID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard auto team returned error: %v", err)
	}
	if autoDashboard.ActiveSprint == nil {
		t.Fatal("auto team active sprint is nil, want current week sprint")
	}
	if autoDashboard.ActiveSprint.Sprint.Name != currentName ||
		autoDashboard.ActiveSprint.Sprint.StartsOn != currentStartsOn ||
		autoDashboard.ActiveSprint.Sprint.EndsOn != currentEndsOn ||
		autoDashboard.ActiveSprint.Sprint.Status != "active" ||
		autoDashboard.ActiveSprint.Sprint.StartedAt == "" {
		t.Fatalf("auto sprint = %#v, want active %s %s-%s", autoDashboard.ActiveSprint.Sprint, currentName, currentStartsOn, currentEndsOn)
	}

	currentTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Current Week Team"})
	if err != nil {
		t.Fatalf("CreateTeam current returned error: %v", err)
	}
	currentSprint, err := store.CreateSprint(ctx, CreateSprintParams{TeamID: currentTeam.ID})
	if err != nil {
		t.Fatalf("CreateSprint current week returned error: %v", err)
	}
	if currentSprint.Name != currentName || currentSprint.StartsOn != currentStartsOn || currentSprint.EndsOn != currentEndsOn || currentSprint.Status != "active" || currentSprint.StartedAt == "" {
		t.Fatalf("current sprint = %#v, want active %s %s-%s", currentSprint, currentName, currentStartsOn, currentEndsOn)
	}
}

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
	if otherBoard.TeamID == board.TeamID {
		t.Fatalf("otherBoard.TeamID = %q, want a separate team-owned board", otherBoard.TeamID)
	}

	initial, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard returned error: %v", err)
	}
	if initial.ActiveSprint == nil {
		t.Fatal("initial active sprint is nil, want existing board cards mapped to a current sprint")
	}
	activeCard := initial.ActiveSprint.Cards[0]

	defaultDashboard, err := store.GetPlanningDashboard(ctx, board.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard default team returned error: %v", err)
	}
	if len(defaultDashboard.Boards) != 1 || defaultDashboard.Boards[0].TeamID != board.TeamID {
		t.Fatalf("default team boards = %#v, want exactly one team-owned board", defaultDashboard.Boards)
	}
	otherDashboardBefore, err := store.GetPlanningDashboard(ctx, otherBoard.TeamID)
	if err != nil {
		t.Fatalf("GetPlanningDashboard other team returned error: %v", err)
	}
	if len(otherDashboardBefore.Boards) != 1 || otherDashboardBefore.Boards[0].ID != otherBoard.ID {
		t.Fatalf("other team boards = %#v, want release train board", otherDashboardBefore.Boards)
	}

	sprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:   board.TeamID,
		Goal:     "Ship planning foundations",
		StartsOn: "2026-05-11",
	})
	if err != nil {
		t.Fatalf("CreateSprint returned error: %v", err)
	}
	if sprint.Status != "planned" || sprint.Name != "Sprint 2026-W20" || sprint.StartsOn != "2026-05-11" || sprint.EndsOn != "2026-05-17" || sprint.TeamID != board.TeamID {
		t.Fatalf("created sprint = %#v, want planned ISO week sprint", sprint)
	}

	teamStore := TeamStore{Conn: store.Conn}
	otherTeam, err := teamStore.CreateTeam(ctx, CreateTeamParams{Name: "Mobile Team"})
	if err != nil {
		t.Fatalf("CreateTeam returned error: %v", err)
	}
	otherTeamBoard, err := singleBoardForTeam(ctx, store, otherTeam.ID)
	if err != nil {
		t.Fatalf("singleBoardForTeam other team returned error: %v", err)
	}
	otherSprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:   otherTeam.ID,
		BoardID:  otherTeamBoard.ID,
		StartsOn: "2026-05-11",
	})
	if err != nil {
		t.Fatalf("CreateSprint for other team returned error: %v", err)
	}
	if otherSprint.Status != "planned" {
		t.Fatalf("other sprint status = %q, want planned because current week autostarted", otherSprint.Status)
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
	currentName, _, _ := weeklySprintWindow(time.Now())
	if otherDashboard.ActiveSprint == nil || otherDashboard.ActiveSprint.Sprint.Name != currentName {
		t.Fatalf("other team active sprint = %#v, want autogenerated %s", otherDashboard.ActiveSprint, currentName)
	}
	if len(otherDashboard.PlannedSprints) != 1 || otherDashboard.PlannedSprints[0].Sprint.ID != otherSprint.ID {
		t.Fatalf("other team planned sprints = %#v, want future sprint %s", otherDashboard.PlannedSprints, otherSprint.ID)
	}
}

func boardsForTeam(boards []BoardSummary, teamID string) []BoardSummary {
	matches := make([]BoardSummary, 0)
	for _, board := range boards {
		if board.TeamID == teamID {
			matches = append(matches, board)
		}
	}
	return matches
}

func singleBoardForTeam(ctx context.Context, store BoardStore, teamID string) (Board, error) {
	boards, err := store.ListBoards(ctx)
	if err != nil {
		return Board{}, err
	}
	teamBoards := boardsForTeam(boards, teamID)
	if len(teamBoards) != 1 {
		return Board{}, fmt.Errorf("%w: expected one board for team", ErrValidation)
	}
	return store.GetBoard(ctx, teamBoards[0].ID)
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

	first, err := store.CreateSprint(ctx, CreateSprintParams{BoardID: board.ID, StartsOn: "2026-05-11"})
	if err != nil {
		t.Fatalf("CreateSprint first returned error: %v", err)
	}
	if first.Name != "Sprint 2026-W20" {
		t.Fatalf("first sprint name = %q, want Sprint 2026-W20", first.Name)
	}
	if _, err := store.CreateSprint(ctx, CreateSprintParams{BoardID: board.ID, StartsOn: "2026-05-11"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSprint duplicate week error = %v, want ErrValidation", err)
	}
	second, err := store.CreateSprint(ctx, CreateSprintParams{BoardID: board.ID, StartsOn: "2026-05-18"})
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
	currentName, currentStartsOn, currentEndsOn := weeklySprintWindow(time.Now())
	if currentName == "" || currentStartsOn == "" || currentEndsOn == "" {
		t.Fatalf("weeklySprintWindow returned empty values: %q %q %q", currentName, currentStartsOn, currentEndsOn)
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
	otherBoard, err := singleBoardForTeam(ctx, store, otherTeam.ID)
	if err != nil {
		t.Fatalf("singleBoardForTeam other team returned error: %v", err)
	}
	if _, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:   board.TeamID,
		BoardID:  otherBoard.ID,
		StartsOn: "2026-05-11",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("CreateSprint mismatched team board error = %v, want ErrValidation", err)
	}

	otherSprint, err := store.CreateSprint(ctx, CreateSprintParams{
		TeamID:   otherTeam.ID,
		BoardID:  otherBoard.ID,
		StartsOn: "2026-05-11",
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
	nextSprint, err := store.CreateSprint(ctx, CreateSprintParams{TeamID: board.TeamID, StartsOn: "2026-05-11"})
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
	otherBoard, err := singleBoardForTeam(ctx, store, otherTeam.ID)
	if err != nil {
		t.Fatalf("singleBoardForTeam other team returned error: %v", err)
	}
	otherSprint, err := store.CreateSprint(ctx, CreateSprintParams{TeamID: otherTeam.ID, BoardID: otherBoard.ID, StartsOn: "2026-05-11"})
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
