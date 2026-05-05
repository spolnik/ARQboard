package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Sprint struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	TeamID      string `json:"teamId"`
	BoardID     string `json:"boardId"`
	Name        string `json:"name"`
	Goal        string `json:"goal"`
	Status      string `json:"status"`
	StartsOn    string `json:"startsOn,omitempty"`
	EndsOn      string `json:"endsOn,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type SprintPlan struct {
	Sprint Sprint      `json:"sprint"`
	Cards  []BoardCard `json:"cards"`
}

type PlanningDashboard struct {
	BoardID          string         `json:"boardId"`
	TeamID           string         `json:"teamId"`
	TeamName         string         `json:"teamName"`
	Boards           []BoardSummary `json:"boards"`
	Backlog          []BoardCard    `json:"backlog"`
	ActiveSprint     *SprintPlan    `json:"activeSprint,omitempty"`
	PlannedSprints   []SprintPlan   `json:"plannedSprints"`
	CompletedSprints []SprintPlan   `json:"completedSprints"`
}

type CreateSprintParams struct {
	BoardID  string
	TeamID   string
	Name     string
	Goal     string
	StartsOn string
	EndsOn   string
}

type CompleteSprintParams struct {
	SprintID string
	Rollover []SprintRolloverDecision
}

type SprintRolloverDecision struct {
	CardID   string
	SprintID string
}

type AssignCardToSprintParams struct {
	CardID   string
	SprintID string
}

func (store BoardStore) GetPlanningDashboard(ctx context.Context, scopeID string) (PlanningDashboard, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return PlanningDashboard{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return PlanningDashboard{}, err
	}
	defer tx.Rollback()

	if _, err := ensureDefaultBoard(ctx, tx, driver); err != nil {
		return PlanningDashboard{}, err
	}
	teamID, err := resolvePlanningTeam(ctx, tx, driver, scopeID)
	if err != nil {
		return PlanningDashboard{}, err
	}
	if _, err := ensureCurrentSprintForTeam(ctx, tx, driver, teamID); err != nil {
		return PlanningDashboard{}, err
	}

	dashboard, err := loadPlanningDashboard(ctx, tx, driver, teamID)
	if err != nil {
		return PlanningDashboard{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlanningDashboard{}, err
	}
	return dashboard, nil
}

func (store BoardStore) CreateSprint(ctx context.Context, params CreateSprintParams) (Sprint, error) {
	boardID := strings.TrimSpace(params.BoardID)
	teamID := strings.TrimSpace(params.TeamID)
	name, startsOn, endsOn, err := weeklySprintWindowFromInput(params.StartsOn)
	if err != nil {
		return Sprint{}, err
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Sprint{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Sprint{}, err
	}
	defer tx.Rollback()

	var workspaceID string
	if teamID != "" {
		workspaceID, err = loadTeamWorkspace(ctx, tx, driver, teamID)
		if err != nil {
			return Sprint{}, err
		}
		if boardID == "" {
			boardID, err = defaultBoardIDForTeam(ctx, tx, driver, teamID)
			if err != nil {
				return Sprint{}, err
			}
		}
	} else {
		if boardID == "" {
			return Sprint{}, fmt.Errorf("%w: teamId is required", ErrValidation)
		}
		workspaceID, teamID, err = loadBoardScope(ctx, tx, driver, boardID)
		if err != nil {
			return Sprint{}, err
		}
	}
	boardWorkspaceID, boardTeamID, err := loadBoardScope(ctx, tx, driver, boardID)
	if err != nil {
		return Sprint{}, err
	}
	if boardWorkspaceID != workspaceID || boardTeamID != teamID {
		return Sprint{}, fmt.Errorf("%w: sprint board must belong to the selected team", ErrValidation)
	}
	activeCount, err := countActiveSprints(ctx, tx, driver, teamID)
	if err != nil {
		return Sprint{}, err
	}
	status := "planned"
	if startsOn == currentWeeklySprintStart() && activeCount == 0 {
		status = "active"
	}
	sprintID, err := newID()
	if err != nil {
		return Sprint{}, err
	}

	startedAt := "NULL"
	if status == "active" {
		startedAt = currentTimestamp(driver)
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO sprints (id, workspace_id, team_id, board_id, name, goal, status, starts_on, ends_on, started_at)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		placeholder(driver, 6),
		placeholder(driver, 7),
		placeholder(driver, 8),
		placeholder(driver, 9),
		startedAt,
	), sprintID, workspaceID, teamID, boardID, name, strings.TrimSpace(params.Goal), status, startsOn, endsOn)
	if err != nil {
		if isUniqueViolation(err) {
			return Sprint{}, fmt.Errorf("%w: sprint week already exists", ErrValidation)
		}
		return Sprint{}, err
	}

	sprint, err := loadSprint(ctx, tx, driver, sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if err := tx.Commit(); err != nil {
		return Sprint{}, err
	}
	return sprint, nil
}

func (store BoardStore) StartSprint(ctx context.Context, sprintID string) (Sprint, error) {
	sprintID = strings.TrimSpace(sprintID)
	if sprintID == "" {
		return Sprint{}, fmt.Errorf("%w: sprintId is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Sprint{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Sprint{}, err
	}
	defer tx.Rollback()

	sprint, err := loadSprint(ctx, tx, driver, sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if sprint.Status != "planned" {
		return Sprint{}, fmt.Errorf("%w: only planned sprints can be started", ErrValidation)
	}

	activeCount, err := countActiveSprints(ctx, tx, driver, sprint.TeamID)
	if err != nil {
		return Sprint{}, err
	}
	if activeCount > 0 {
		return Sprint{}, fmt.Errorf("%w: another sprint is already active", ErrValidation)
	}

	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE sprints
		SET status = 'active', started_at = %s, updated_at = %s
		WHERE id = %s AND status = 'planned'
	`, currentTimestamp(driver), currentTimestamp(driver), placeholder(driver, 1)), sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return Sprint{}, ErrNotFound
	}

	started, err := loadSprint(ctx, tx, driver, sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if err := tx.Commit(); err != nil {
		return Sprint{}, err
	}
	return started, nil
}

func (store BoardStore) CompleteSprint(ctx context.Context, params CompleteSprintParams) (Sprint, error) {
	sprintID := strings.TrimSpace(params.SprintID)
	if sprintID == "" {
		return Sprint{}, fmt.Errorf("%w: sprintId is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Sprint{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Sprint{}, err
	}
	defer tx.Rollback()

	sprint, err := loadSprint(ctx, tx, driver, sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if sprint.Status != "active" {
		return Sprint{}, fmt.Errorf("%w: only active sprints can be completed", ErrValidation)
	}
	rollover, err := validateSprintRollover(ctx, tx, driver, sprint, params.Rollover)
	if err != nil {
		return Sprint{}, err
	}
	activeCards, err := loadSprintCards(ctx, tx, driver, sprint.ID)
	if err != nil {
		return Sprint{}, err
	}
	for _, card := range activeCards {
		nextSprintID := rollover[card.ID]
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE cards
			SET sprint_id = %s, updated_at = %s
			WHERE id = %s
		`, placeholder(driver, 1), currentTimestamp(driver), placeholder(driver, 2)), nullableString(nextSprintID), card.ID)
		if err != nil {
			return Sprint{}, err
		}
		if err := appendActivity(ctx, tx, driver, sprint.WorkspaceID, sprint.BoardID, card.ID, "card.sprint_assigned"); err != nil {
			return Sprint{}, err
		}
	}

	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE sprints
		SET status = 'completed', completed_at = %s, updated_at = %s
		WHERE id = %s AND status = 'active'
	`, currentTimestamp(driver), currentTimestamp(driver), placeholder(driver, 1)), sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return Sprint{}, ErrNotFound
	}

	completed, err := loadSprint(ctx, tx, driver, sprintID)
	if err != nil {
		return Sprint{}, err
	}
	if err := tx.Commit(); err != nil {
		return Sprint{}, err
	}
	return completed, nil
}

func (store BoardStore) AssignCardToSprint(ctx context.Context, params AssignCardToSprintParams) (BoardCard, error) {
	cardID := strings.TrimSpace(params.CardID)
	sprintID := strings.TrimSpace(params.SprintID)
	if cardID == "" {
		return BoardCard{}, fmt.Errorf("%w: cardId is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return BoardCard{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return BoardCard{}, err
	}
	defer tx.Rollback()

	cardWorkspaceID, boardID, err := loadCardScope(ctx, tx, driver, cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if sprintID != "" {
		sprint, err := loadSprint(ctx, tx, driver, sprintID)
		if err != nil {
			return BoardCard{}, err
		}
		_, cardTeamID, err := loadBoardScope(ctx, tx, driver, boardID)
		if err != nil {
			return BoardCard{}, err
		}
		if sprint.WorkspaceID != cardWorkspaceID || sprint.TeamID != cardTeamID {
			return BoardCard{}, fmt.Errorf("%w: sprint and card belong to different teams", ErrValidation)
		}
		if sprint.Status == "completed" {
			return BoardCard{}, fmt.Errorf("%w: completed sprints cannot accept cards", ErrValidation)
		}
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE cards
		SET sprint_id = %s, updated_at = %s
		WHERE id = %s
	`, placeholder(driver, 1), currentTimestamp(driver), placeholder(driver, 2)), nullableString(sprintID), cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if err := appendActivity(ctx, tx, driver, cardWorkspaceID, boardID, cardID, "card.sprint_assigned"); err != nil {
		return BoardCard{}, err
	}

	card, err := loadCard(ctx, tx, driver, cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if err := tx.Commit(); err != nil {
		return BoardCard{}, err
	}
	return card, nil
}

func resolvePlanningTeam(ctx context.Context, q sqlQueryer, driver Driver, scopeID string) (string, error) {
	scopeID = strings.TrimSpace(scopeID)
	if scopeID != "" {
		if teamID, found, err := lookupID(ctx, q, driver, "SELECT id FROM teams WHERE id = "+placeholder(driver, 1), scopeID); err != nil {
			return "", err
		} else if found {
			return teamID, nil
		}
		if teamID, found, err := lookupID(ctx, q, driver, "SELECT team_id FROM boards WHERE id = "+placeholder(driver, 1), scopeID); err != nil {
			return "", err
		} else if found {
			return teamID, nil
		}
		return "", ErrNotFound
	}

	workspaceID, err := ensureWorkspace(ctx, q, driver)
	if err != nil {
		return "", err
	}
	return ensureDefaultTeam(ctx, q, driver, workspaceID)
}

func loadPlanningDashboard(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (PlanningDashboard, error) {
	var teamName string
	var workspaceID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT name, workspace_id
		FROM teams
		WHERE id = %s
	`, placeholder(driver, 1)), teamID).Scan(&teamName, &workspaceID); err != nil {
		return PlanningDashboard{}, notFoundOrErr(err)
	}
	boards, err := listBoardSummariesForTeam(ctx, q, driver, teamID)
	if err != nil {
		return PlanningDashboard{}, err
	}
	backlog, err := loadBacklogCards(ctx, q, driver, teamID)
	if err != nil {
		return PlanningDashboard{}, err
	}
	planned, err := loadSprintPlans(ctx, q, driver, teamID, "planned")
	if err != nil {
		return PlanningDashboard{}, err
	}
	active, err := loadSprintPlans(ctx, q, driver, teamID, "active")
	if err != nil {
		return PlanningDashboard{}, err
	}
	completed, err := loadSprintPlans(ctx, q, driver, teamID, "completed")
	if err != nil {
		return PlanningDashboard{}, err
	}

	var activeSprint *SprintPlan
	if len(active) > 0 {
		activeSprint = &active[0]
	}
	boardID := ""
	if len(boards) > 0 {
		boardID = boards[0].ID
	}
	return PlanningDashboard{
		TeamID:           teamID,
		TeamName:         teamName,
		BoardID:          boardID,
		Boards:           boards,
		Backlog:          backlog,
		ActiveSprint:     activeSprint,
		PlannedSprints:   planned,
		CompletedSprints: completed,
	}, nil
}

func loadBacklogCards(ctx context.Context, q sqlQueryer, driver Driver, teamID string) ([]BoardCard, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT cards.id, cards.column_id, %s, COALESCE(boards.name, ''), cards.sprint_id, cards.epic_id, cards.title, cards.description, cards.owner_initials, cards.priority, cards.position, %s,
			COALESCE(%s, ''), COALESCE(users.display_name, ''), COALESCE(users.email, '')
		FROM cards
		JOIN boards ON boards.id = cards.board_id
		LEFT JOIN users ON users.id = cards.assignee_id
		WHERE boards.team_id = %s AND cards.sprint_id IS NULL
		ORDER BY boards.name, cards.created_at, cards.position, cards.id
	`, idText(driver, "cards.board_id"), cardDueText(driver), idText(driver, "cards.assignee_id"), placeholder(driver, 1)), teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]BoardCard, 0)
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachLabelsToCards(ctx, q, driver, cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func loadSprintPlans(ctx context.Context, q sqlQueryer, driver Driver, teamID string, status string) ([]SprintPlan, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, workspace_id, team_id, board_id, name, goal, status, starts_on, ends_on, %s, %s
		FROM sprints
		WHERE team_id = %s AND status = %s
		ORDER BY COALESCE(starts_on, started_at, completed_at, created_at), name, id
	`,
		timeText(driver, "started_at"),
		timeText(driver, "completed_at"),
		placeholder(driver, 1),
		placeholder(driver, 2),
	), teamID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sprints := make([]Sprint, 0)
	for rows.Next() {
		sprint, err := scanSprint(rows)
		if err != nil {
			return nil, err
		}
		sprints = append(sprints, sprint)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	plans := make([]SprintPlan, 0, len(sprints))
	for _, sprint := range sprints {
		cards, err := loadSprintCards(ctx, q, driver, sprint.ID)
		if err != nil {
			return nil, err
		}
		plans = append(plans, SprintPlan{Sprint: sprint, Cards: cards})
	}
	return plans, nil
}

func loadSprintCards(ctx context.Context, q sqlQueryer, driver Driver, sprintID string) ([]BoardCard, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT cards.id, cards.column_id, %s, COALESCE((SELECT boards.name FROM boards WHERE boards.id = cards.board_id), ''), cards.sprint_id, cards.epic_id, cards.title, cards.description, cards.owner_initials, cards.priority, cards.position, %s,
			COALESCE(%s, ''), COALESCE(users.display_name, ''), COALESCE(users.email, '')
		FROM cards
		LEFT JOIN users ON users.id = cards.assignee_id
		WHERE cards.sprint_id = %s
		ORDER BY cards.position, cards.created_at, cards.id
	`, idText(driver, "cards.board_id"), cardDueText(driver), idText(driver, "cards.assignee_id"), placeholder(driver, 1)), sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]BoardCard, 0)
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachLabelsToCards(ctx, q, driver, cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func loadSprint(ctx context.Context, q sqlQueryer, driver Driver, sprintID string) (Sprint, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, workspace_id, team_id, board_id, name, goal, status, starts_on, ends_on, %s, %s
		FROM sprints
		WHERE id = %s
	`, timeText(driver, "started_at"), timeText(driver, "completed_at"), placeholder(driver, 1)), sprintID)

	sprint, err := scanSprint(row)
	if err != nil {
		return Sprint{}, notFoundOrErr(err)
	}
	return sprint, nil
}

func countActiveSprints(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (int, error) {
	var count int
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM sprints
		WHERE team_id = %s AND status = 'active'
	`, placeholder(driver, 1)), teamID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func loadBoardWorkspace(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (string, error) {
	workspaceID, _, err := loadBoardScope(ctx, q, driver, boardID)
	return workspaceID, err
}

func loadBoardScope(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (string, string, error) {
	var workspaceID string
	var teamID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT workspace_id, team_id
		FROM boards
		WHERE id = %s
	`, placeholder(driver, 1)), boardID).Scan(&workspaceID, &teamID); err != nil {
		return "", "", notFoundOrErr(err)
	}
	return workspaceID, teamID, nil
}

func defaultBoardIDForTeam(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (string, error) {
	boardID, found, err := boardIDForTeam(ctx, q, driver, teamID)
	if err != nil {
		return "", err
	}
	if found {
		return boardID, nil
	}
	return ensureBoardForTeam(ctx, q, driver, teamID)
}

func listBoardSummariesForTeam(ctx context.Context, q sqlQueryer, driver Driver, teamID string) ([]BoardSummary, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s,
			%s,
			%s,
			boards.name,
			boards.slug,
			COUNT(DISTINCT columns.id) AS column_count,
			COUNT(DISTINCT cards.id) AS card_count
		FROM boards
		LEFT JOIN columns ON columns.board_id = boards.id
		LEFT JOIN cards ON cards.board_id = boards.id
		WHERE boards.team_id = %s
		GROUP BY boards.id, boards.workspace_id, boards.team_id, boards.name, boards.slug, boards.created_at
		ORDER BY boards.name, boards.created_at, boards.id
	`,
		idText(driver, "boards.id"),
		idText(driver, "boards.workspace_id"),
		idText(driver, "boards.team_id"),
		placeholder(driver, 1),
	), teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	boards := make([]BoardSummary, 0)
	for rows.Next() {
		var board BoardSummary
		if err := rows.Scan(&board.ID, &board.WorkspaceID, &board.TeamID, &board.Name, &board.Slug, &board.ColumnCount, &board.CardCount); err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return boards, nil
}

func ensureCurrentSprintForTeam(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (string, error) {
	workspaceID, err := loadTeamWorkspace(ctx, q, driver, teamID)
	if err != nil {
		return "", err
	}
	boardID, err := defaultBoardIDForTeam(ctx, q, driver, teamID)
	if err != nil {
		return "", err
	}
	return ensureActiveSprintForBoard(ctx, q, driver, workspaceID, boardID)
}

func ensureActiveSprintForBoard(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, boardID string) (string, error) {
	boardWorkspaceID, teamID, err := loadBoardScope(ctx, q, driver, boardID)
	if err != nil {
		return "", err
	}
	if boardWorkspaceID != workspaceID {
		return "", fmt.Errorf("%w: board must belong to workspace", ErrValidation)
	}
	sprintID, found, err := activeSprintIDForTeam(ctx, q, driver, teamID)
	if err != nil || found {
		return sprintID, err
	}

	sprintID, err = newID()
	if err != nil {
		return "", err
	}
	name, startsOn, endsOn := weeklySprintWindow(time.Now())
	existingSprintID, found, err := lookupID(ctx, q, driver, fmt.Sprintf(`
		SELECT id
		FROM sprints
		WHERE team_id = %s AND name = %s
	`, placeholder(driver, 1), placeholder(driver, 2)), teamID, name)
	if err != nil {
		return "", err
	}
	if found {
		result, err := q.ExecContext(ctx, fmt.Sprintf(`
			UPDATE sprints
			SET status = 'active',
				started_at = COALESCE(started_at, %s),
				updated_at = %s
			WHERE id = %s AND status = 'planned'
		`, currentTimestamp(driver), currentTimestamp(driver), placeholder(driver, 1)), existingSprintID)
		if err != nil {
			return "", err
		}
		if rows, err := result.RowsAffected(); err == nil && rows > 0 {
			return existingSprintID, nil
		}
		return "", nil
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO sprints (id, workspace_id, team_id, board_id, name, goal, status, starts_on, ends_on, started_at)
		VALUES (%s, %s, %s, %s, %s, %s, 'active', %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		placeholder(driver, 6),
		placeholder(driver, 7),
		placeholder(driver, 8),
		currentTimestamp(driver),
	), sprintID, workspaceID, teamID, boardID, name, "Current work for this team.", startsOn, endsOn)
	if err != nil {
		return "", err
	}
	return sprintID, nil
}

func weeklySprintWindowFromInput(startsOn string) (string, string, string, error) {
	startsOn = strings.TrimSpace(startsOn)
	if startsOn == "" {
		name, start, end := weeklySprintWindow(time.Now())
		return name, start, end, nil
	}
	start, err := time.ParseInLocation("2006-01-02", startsOn, time.Local)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: startsOn must be a YYYY-MM-DD date", ErrValidation)
	}
	name, startDate, endDate := weeklySprintWindow(start)
	return name, startDate, endDate, nil
}

func weeklySprintWindow(reference time.Time) (string, string, string) {
	if reference.IsZero() {
		reference = time.Now()
	}
	date := time.Date(reference.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, reference.Location())
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := date.AddDate(0, 0, -(weekday - 1))
	end := start.AddDate(0, 0, 6)
	year, week := start.ISOWeek()
	return fmt.Sprintf("Sprint %04d-W%02d", year, week), start.Format("2006-01-02"), end.Format("2006-01-02")
}

func currentWeeklySprintStart() string {
	_, startsOn, _ := weeklySprintWindow(time.Now())
	return startsOn
}

func activeSprintIDForBoard(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (string, bool, error) {
	_, teamID, err := loadBoardScope(ctx, q, driver, boardID)
	if err != nil {
		return "", false, err
	}
	return activeSprintIDForTeam(ctx, q, driver, teamID)
}

func activeSprintIDForTeam(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (string, bool, error) {
	return lookupID(ctx, q, driver, fmt.Sprintf(`
		SELECT id
		FROM sprints
		WHERE team_id = %s AND status = 'active'
		ORDER BY started_at DESC, created_at DESC, id
		LIMIT 1
	`, placeholder(driver, 1)), teamID)
}

func uniqueSprintName(ctx context.Context, q sqlQueryer, driver Driver, teamID string, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Sprint"
	}

	for index := 0; index < 100; index++ {
		name := base
		if index > 0 {
			name = fmt.Sprintf("%s %d", base, index+1)
		}

		var existingID string
		if err := q.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT id
			FROM sprints
			WHERE team_id = %s AND name = %s
		`, placeholder(driver, 1), placeholder(driver, 2)), teamID, name).Scan(&existingID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return name, nil
			}
			return "", err
		}
	}

	return "", fmt.Errorf("%w: sprint name is unavailable", ErrValidation)
}

func validateSprintRollover(ctx context.Context, q sqlQueryer, driver Driver, sprint Sprint, decisions []SprintRolloverDecision) (map[string]string, error) {
	activeCards, err := loadSprintCards(ctx, q, driver, sprint.ID)
	if err != nil {
		return nil, err
	}
	activeCardIDs := make(map[string]bool, len(activeCards))
	for _, card := range activeCards {
		activeCardIDs[card.ID] = true
	}

	rollover := make(map[string]string, len(decisions))
	for _, decision := range decisions {
		cardID := strings.TrimSpace(decision.CardID)
		nextSprintID := strings.TrimSpace(decision.SprintID)
		if cardID == "" {
			return nil, fmt.Errorf("%w: rollover cardId is required", ErrValidation)
		}
		if !activeCardIDs[cardID] {
			return nil, fmt.Errorf("%w: rollover card must belong to the active sprint", ErrValidation)
		}
		if _, exists := rollover[cardID]; exists {
			return nil, fmt.Errorf("%w: duplicate rollover card", ErrValidation)
		}
		if nextSprintID != "" {
			nextSprint, err := loadSprint(ctx, q, driver, nextSprintID)
			if err != nil {
				return nil, err
			}
			if nextSprint.WorkspaceID != sprint.WorkspaceID || nextSprint.TeamID != sprint.TeamID {
				return nil, fmt.Errorf("%w: rollover sprint must belong to the same team", ErrValidation)
			}
			if nextSprint.ID == sprint.ID || nextSprint.Status != "planned" {
				return nil, fmt.Errorf("%w: rollover sprint must be planned", ErrValidation)
			}
		}
		rollover[cardID] = nextSprintID
	}
	return rollover, nil
}

func scanSprint(scanner cardScanner) (Sprint, error) {
	var sprint Sprint
	var startsOn sql.NullString
	var endsOn sql.NullString
	var startedAt sql.NullString
	var completedAt sql.NullString
	if err := scanner.Scan(
		&sprint.ID,
		&sprint.WorkspaceID,
		&sprint.TeamID,
		&sprint.BoardID,
		&sprint.Name,
		&sprint.Goal,
		&sprint.Status,
		&startsOn,
		&endsOn,
		&startedAt,
		&completedAt,
	); err != nil {
		return Sprint{}, err
	}
	sprint.StartsOn = startsOn.String
	sprint.EndsOn = endsOn.String
	sprint.StartedAt = startedAt.String
	sprint.CompletedAt = completedAt.String
	return sprint, nil
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
