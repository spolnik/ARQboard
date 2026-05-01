package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Sprint struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
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
	BoardID          string       `json:"boardId"`
	Backlog          []BoardCard  `json:"backlog"`
	ActiveSprint     *SprintPlan  `json:"activeSprint,omitempty"`
	PlannedSprints   []SprintPlan `json:"plannedSprints"`
	CompletedSprints []SprintPlan `json:"completedSprints"`
}

type CreateSprintParams struct {
	BoardID  string
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

func (store BoardStore) GetPlanningDashboard(ctx context.Context, boardID string) (PlanningDashboard, error) {
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
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID, err = ensureDefaultBoard(ctx, tx, driver)
		if err != nil {
			return PlanningDashboard{}, err
		}
	}
	workspaceID, err := loadBoardWorkspace(ctx, tx, driver, boardID)
	if err != nil {
		return PlanningDashboard{}, err
	}

	dashboard, err := loadPlanningDashboard(ctx, tx, driver, workspaceID, boardID)
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
	if boardID == "" {
		return Sprint{}, fmt.Errorf("%w: boardId is required", ErrValidation)
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Sprint{}, fmt.Errorf("%w: sprint name is required", ErrValidation)
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

	workspaceID, err := loadBoardWorkspace(ctx, tx, driver, boardID)
	if err != nil {
		return Sprint{}, err
	}
	sprintID, err := newID()
	if err != nil {
		return Sprint{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO sprints (id, workspace_id, board_id, name, goal, starts_on, ends_on)
		VALUES (%s, %s, %s, %s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		placeholder(driver, 6),
		placeholder(driver, 7),
	), sprintID, workspaceID, boardID, name, strings.TrimSpace(params.Goal), nullableString(params.StartsOn), nullableString(params.EndsOn))
	if err != nil {
		if isUniqueViolation(err) {
			return Sprint{}, fmt.Errorf("%w: sprint name already exists", ErrValidation)
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

	activeCount, err := countActiveSprints(ctx, tx, driver, sprint.BoardID)
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
		if sprint.WorkspaceID != cardWorkspaceID || sprint.BoardID != boardID {
			return BoardCard{}, fmt.Errorf("%w: sprint and card belong to different boards", ErrValidation)
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

func loadPlanningDashboard(ctx context.Context, q sqlQueryer, driver Driver, _ string, boardID string) (PlanningDashboard, error) {
	backlog, err := loadBacklogCards(ctx, q, driver, boardID)
	if err != nil {
		return PlanningDashboard{}, err
	}
	planned, err := loadSprintPlans(ctx, q, driver, boardID, "planned")
	if err != nil {
		return PlanningDashboard{}, err
	}
	active, err := loadSprintPlans(ctx, q, driver, boardID, "active")
	if err != nil {
		return PlanningDashboard{}, err
	}
	completed, err := loadSprintPlans(ctx, q, driver, boardID, "completed")
	if err != nil {
		return PlanningDashboard{}, err
	}

	var activeSprint *SprintPlan
	if len(active) > 0 {
		activeSprint = &active[0]
	}
	return PlanningDashboard{
		BoardID:          boardID,
		Backlog:          backlog,
		ActiveSprint:     activeSprint,
		PlannedSprints:   planned,
		CompletedSprints: completed,
	}, nil
}

func loadBacklogCards(ctx context.Context, q sqlQueryer, driver Driver, boardID string) ([]BoardCard, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT cards.id, cards.column_id, cards.sprint_id, cards.title, cards.description, cards.owner_initials, cards.priority, cards.position, %s
		FROM cards
		WHERE cards.board_id = %s AND cards.sprint_id IS NULL
		ORDER BY cards.created_at, cards.position, cards.id
	`, cardDueText(driver), placeholder(driver, 1)), boardID)
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
	return cards, nil
}

func loadSprintPlans(ctx context.Context, q sqlQueryer, driver Driver, boardID string, status string) ([]SprintPlan, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, workspace_id, board_id, name, goal, status, starts_on, ends_on, %s, %s
		FROM sprints
		WHERE board_id = %s AND status = %s
		ORDER BY created_at, name, id
	`,
		timeText(driver, "started_at"),
		timeText(driver, "completed_at"),
		placeholder(driver, 1),
		placeholder(driver, 2),
	), boardID, status)
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
		SELECT id, column_id, sprint_id, title, description, owner_initials, priority, position, %s
		FROM cards
		WHERE sprint_id = %s
		ORDER BY position, created_at, id
	`, cardDueText(driver), placeholder(driver, 1)), sprintID)
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
	return cards, nil
}

func loadSprint(ctx context.Context, q sqlQueryer, driver Driver, sprintID string) (Sprint, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, workspace_id, board_id, name, goal, status, starts_on, ends_on, %s, %s
		FROM sprints
		WHERE id = %s
	`, timeText(driver, "started_at"), timeText(driver, "completed_at"), placeholder(driver, 1)), sprintID)

	sprint, err := scanSprint(row)
	if err != nil {
		return Sprint{}, notFoundOrErr(err)
	}
	return sprint, nil
}

func countActiveSprints(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (int, error) {
	var count int
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM sprints
		WHERE board_id = %s AND status = 'active'
	`, placeholder(driver, 1)), boardID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func loadBoardWorkspace(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (string, error) {
	var workspaceID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT workspace_id
		FROM boards
		WHERE id = %s
	`, placeholder(driver, 1)), boardID).Scan(&workspaceID); err != nil {
		return "", notFoundOrErr(err)
	}
	return workspaceID, nil
}

func ensureActiveSprintForBoard(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, boardID string) (string, error) {
	sprintID, found, err := activeSprintIDForBoard(ctx, q, driver, boardID)
	if err != nil || found {
		return sprintID, err
	}

	sprintID, err = newID()
	if err != nil {
		return "", err
	}
	name, err := uniqueSprintName(ctx, q, driver, boardID, "Current sprint")
	if err != nil {
		return "", err
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO sprints (id, workspace_id, board_id, name, goal, status, started_at)
		VALUES (%s, %s, %s, %s, %s, 'active', %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		currentTimestamp(driver),
	), sprintID, workspaceID, boardID, name, "Current work for this board.")
	if err != nil {
		return "", err
	}
	return sprintID, nil
}

func activeSprintIDForBoard(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (string, bool, error) {
	return lookupID(ctx, q, driver, fmt.Sprintf(`
		SELECT id
		FROM sprints
		WHERE board_id = %s AND status = 'active'
		ORDER BY started_at DESC, created_at DESC, id
		LIMIT 1
	`, placeholder(driver, 1)), boardID)
}

func uniqueSprintName(ctx context.Context, q sqlQueryer, driver Driver, boardID string, base string) (string, error) {
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
			WHERE board_id = %s AND name = %s
		`, placeholder(driver, 1), placeholder(driver, 2)), boardID, name).Scan(&existingID); err != nil {
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
			if nextSprint.WorkspaceID != sprint.WorkspaceID || nextSprint.BoardID != sprint.BoardID {
				return nil, fmt.Errorf("%w: rollover sprint must belong to the same board", ErrValidation)
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
