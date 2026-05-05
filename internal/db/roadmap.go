package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Epic struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	TeamID      string `json:"teamId"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Status      string `json:"status"`
	StartsOn    string `json:"startsOn,omitempty"`
	TargetOn    string `json:"targetOn,omitempty"`
}

type RoadmapDashboard struct {
	TeamID          string           `json:"teamId"`
	TeamName        string           `json:"teamName"`
	Epics           []RoadmapEpic    `json:"epics"`
	UnassignedCards []RoadmapCard    `json:"unassignedCards"`
	Dependencies    []CardDependency `json:"dependencies"`
}

type RoadmapEpic struct {
	Epic           Epic          `json:"epic"`
	Cards          []RoadmapCard `json:"cards"`
	TotalCards     int           `json:"totalCards"`
	CompletedCards int           `json:"completedCards"`
	BlockedCards   int           `json:"blockedCards"`
	Progress       int           `json:"progress"`
	Risk           string        `json:"risk"`
}

type RoadmapCard struct {
	Card        BoardCard        `json:"card"`
	ColumnTitle string           `json:"columnTitle"`
	BlockedBy   []CardDependency `json:"blockedBy"`
	Blocking    []CardDependency `json:"blocking"`
}

type CardDependency struct {
	ID            string `json:"id"`
	BlockedCardID string `json:"blockedCardId"`
	BlockedTitle  string `json:"blockedTitle"`
	BlockerCardID string `json:"blockerCardId"`
	BlockerTitle  string `json:"blockerTitle"`
	RelationType  string `json:"relationType"`
}

type CreateEpicParams struct {
	TeamID      string
	Title       string
	Description string
	Status      string
	StartsOn    string
	TargetOn    string
}

type UpdateEpicParams struct {
	EpicID      string
	Title       string
	Description string
	Status      string
	StartsOn    string
	TargetOn    string
}

type AssignCardToEpicParams struct {
	CardID string
	EpicID string
}

type CreateCardDependencyParams struct {
	BlockedCardID string
	BlockerCardID string
	RelationType  string
}

func (store BoardStore) GetRoadmapDashboard(ctx context.Context, scopeID string) (RoadmapDashboard, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return RoadmapDashboard{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return RoadmapDashboard{}, err
	}
	defer tx.Rollback()

	if _, err := ensureDefaultBoard(ctx, tx, driver); err != nil {
		return RoadmapDashboard{}, err
	}
	teamID, err := resolvePlanningTeam(ctx, tx, driver, scopeID)
	if err != nil {
		return RoadmapDashboard{}, err
	}
	team, err := loadTeam(ctx, tx, driver, teamID)
	if err != nil {
		return RoadmapDashboard{}, err
	}
	epics, err := loadEpics(ctx, tx, driver, teamID)
	if err != nil {
		return RoadmapDashboard{}, err
	}
	cards, err := loadRoadmapCards(ctx, tx, driver, teamID)
	if err != nil {
		return RoadmapDashboard{}, err
	}
	dependencies, err := loadCardDependenciesForTeam(ctx, tx, driver, teamID)
	if err != nil {
		return RoadmapDashboard{}, err
	}
	if err := tx.Commit(); err != nil {
		return RoadmapDashboard{}, err
	}
	return buildRoadmapDashboard(team, epics, cards, dependencies), nil
}

func (store BoardStore) CreateEpic(ctx context.Context, params CreateEpicParams) (Epic, error) {
	teamID := strings.TrimSpace(params.TeamID)
	title := strings.TrimSpace(params.Title)
	if teamID == "" || title == "" {
		return Epic{}, fmt.Errorf("%w: teamId and title are required", ErrValidation)
	}
	status, err := normalizeEpicStatus(params.Status)
	if err != nil {
		return Epic{}, err
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Epic{}, err
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Epic{}, err
	}
	defer tx.Rollback()

	workspaceID, err := loadTeamWorkspace(ctx, tx, driver, teamID)
	if err != nil {
		return Epic{}, err
	}
	epicID, err := newID()
	if err != nil {
		return Epic{}, err
	}
	slug, err := uniqueEpicSlug(ctx, tx, driver, teamID, title, "")
	if err != nil {
		return Epic{}, err
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO epics (id, workspace_id, team_id, title, slug, description, status, starts_on, target_on)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
	`,
		placeholder(driver, 1), placeholder(driver, 2), placeholder(driver, 3), placeholder(driver, 4), placeholder(driver, 5),
		placeholder(driver, 6), placeholder(driver, 7), placeholder(driver, 8), placeholder(driver, 9),
	), epicID, workspaceID, teamID, title, slug, strings.TrimSpace(params.Description), status, nullableString(params.StartsOn), nullableString(params.TargetOn))
	if err != nil {
		return Epic{}, err
	}
	epic, err := loadEpic(ctx, tx, driver, epicID)
	if err != nil {
		return Epic{}, err
	}
	if err := tx.Commit(); err != nil {
		return Epic{}, err
	}
	return epic, nil
}

func (store BoardStore) UpdateEpic(ctx context.Context, params UpdateEpicParams) (Epic, error) {
	epicID := strings.TrimSpace(params.EpicID)
	title := strings.TrimSpace(params.Title)
	if epicID == "" || title == "" {
		return Epic{}, fmt.Errorf("%w: epicId and title are required", ErrValidation)
	}
	status, err := normalizeEpicStatus(params.Status)
	if err != nil {
		return Epic{}, err
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Epic{}, err
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Epic{}, err
	}
	defer tx.Rollback()

	current, err := loadEpic(ctx, tx, driver, epicID)
	if err != nil {
		return Epic{}, err
	}
	slug := current.Slug
	if title != current.Title {
		slug, err = uniqueEpicSlug(ctx, tx, driver, current.TeamID, title, epicID)
		if err != nil {
			return Epic{}, err
		}
	}
	result, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE epics
		SET title = %s, slug = %s, description = %s, status = %s, starts_on = %s, target_on = %s, updated_at = %s
		WHERE id = %s
	`,
		placeholder(driver, 1), placeholder(driver, 2), placeholder(driver, 3), placeholder(driver, 4),
		placeholder(driver, 5), placeholder(driver, 6), currentTimestamp(driver), placeholder(driver, 7),
	), title, slug, strings.TrimSpace(params.Description), status, nullableString(params.StartsOn), nullableString(params.TargetOn), epicID)
	if err != nil {
		return Epic{}, err
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return Epic{}, ErrNotFound
	}
	epic, err := loadEpic(ctx, tx, driver, epicID)
	if err != nil {
		return Epic{}, err
	}
	if err := tx.Commit(); err != nil {
		return Epic{}, err
	}
	return epic, nil
}

func (store BoardStore) AssignCardToEpic(ctx context.Context, params AssignCardToEpicParams) (BoardCard, error) {
	cardID := strings.TrimSpace(params.CardID)
	epicID := strings.TrimSpace(params.EpicID)
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

	workspaceID, boardID, teamID, err := loadCardTeamScope(ctx, tx, driver, cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if epicID != "" {
		epic, err := loadEpic(ctx, tx, driver, epicID)
		if err != nil {
			return BoardCard{}, err
		}
		if epic.WorkspaceID != workspaceID || epic.TeamID != teamID {
			return BoardCard{}, fmt.Errorf("%w: epic must belong to the card team", ErrValidation)
		}
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE cards SET epic_id = %s, updated_at = %s WHERE id = %s
	`, placeholder(driver, 1), currentTimestamp(driver), placeholder(driver, 2)), nullableString(epicID), cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if err := appendActivity(ctx, tx, driver, workspaceID, boardID, cardID, "card.updated"); err != nil {
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

func (store BoardStore) CreateCardDependency(ctx context.Context, params CreateCardDependencyParams) (CardDependency, error) {
	blockedCardID := strings.TrimSpace(params.BlockedCardID)
	blockerCardID := strings.TrimSpace(params.BlockerCardID)
	if blockedCardID == "" || blockerCardID == "" {
		return CardDependency{}, fmt.Errorf("%w: blockedCardId and blockerCardId are required", ErrValidation)
	}
	if blockedCardID == blockerCardID {
		return CardDependency{}, fmt.Errorf("%w: a card cannot block itself", ErrValidation)
	}
	relationType := strings.TrimSpace(params.RelationType)
	if relationType == "" {
		relationType = "blocks"
	}
	if relationType != "blocks" {
		return CardDependency{}, fmt.Errorf("%w: relationType must be blocks", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return CardDependency{}, err
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return CardDependency{}, err
	}
	defer tx.Rollback()

	_, _, blockedTeamID, err := loadCardTeamScope(ctx, tx, driver, blockedCardID)
	if err != nil {
		return CardDependency{}, err
	}
	_, _, blockerTeamID, err := loadCardTeamScope(ctx, tx, driver, blockerCardID)
	if err != nil {
		return CardDependency{}, err
	}
	if blockedTeamID != blockerTeamID {
		return CardDependency{}, fmt.Errorf("%w: dependency cards must belong to the same team", ErrValidation)
	}
	dependencyID, err := newID()
	if err != nil {
		return CardDependency{}, err
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO card_dependencies (id, blocked_card_id, blocker_card_id, relation_type)
		VALUES (%s, %s, %s, %s)
	`,
		placeholder(driver, 1), placeholder(driver, 2), placeholder(driver, 3), placeholder(driver, 4),
	), dependencyID, blockedCardID, blockerCardID, relationType)
	if err != nil {
		if isUniqueViolation(err) {
			return CardDependency{}, fmt.Errorf("%w: dependency already exists", ErrValidation)
		}
		return CardDependency{}, err
	}
	dependency, err := loadCardDependency(ctx, tx, driver, dependencyID)
	if err != nil {
		return CardDependency{}, err
	}
	if err := tx.Commit(); err != nil {
		return CardDependency{}, err
	}
	return dependency, nil
}

func (store BoardStore) DeleteCardDependency(ctx context.Context, dependencyID string) error {
	dependencyID = strings.TrimSpace(dependencyID)
	if dependencyID == "" {
		return fmt.Errorf("%w: dependencyId is required", ErrValidation)
	}
	sqlDB, driver, err := store.database()
	if err != nil {
		return err
	}
	result, err := sqlDB.ExecContext(ctx, fmt.Sprintf("DELETE FROM card_dependencies WHERE id = %s", placeholder(driver, 1)), dependencyID)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func buildRoadmapDashboard(team Team, epics []Epic, cards []RoadmapCard, dependencies []CardDependency) RoadmapDashboard {
	blockedBy := make(map[string][]CardDependency)
	blocking := make(map[string][]CardDependency)
	for _, dependency := range dependencies {
		blockedBy[dependency.BlockedCardID] = append(blockedBy[dependency.BlockedCardID], dependency)
		blocking[dependency.BlockerCardID] = append(blocking[dependency.BlockerCardID], dependency)
	}

	cardsByEpic := make(map[string][]RoadmapCard)
	unassigned := make([]RoadmapCard, 0)
	for _, card := range cards {
		card.BlockedBy = blockedBy[card.Card.ID]
		card.Blocking = blocking[card.Card.ID]
		if card.Card.EpicID == "" {
			unassigned = append(unassigned, card)
			continue
		}
		cardsByEpic[card.Card.EpicID] = append(cardsByEpic[card.Card.EpicID], card)
	}

	roadmapEpics := make([]RoadmapEpic, 0, len(epics))
	for _, epic := range epics {
		epicCards := cardsByEpic[epic.ID]
		roadmapEpic := RoadmapEpic{
			Epic:       epic,
			Cards:      epicCards,
			TotalCards: len(epicCards),
			Risk:       "on_track",
		}
		for _, card := range epicCards {
			if strings.EqualFold(card.ColumnTitle, "Done") {
				roadmapEpic.CompletedCards++
			}
			if len(card.BlockedBy) > 0 {
				roadmapEpic.BlockedCards++
			}
		}
		if roadmapEpic.TotalCards > 0 {
			roadmapEpic.Progress = roadmapEpic.CompletedCards * 100 / roadmapEpic.TotalCards
		}
		if roadmapEpic.BlockedCards > 0 {
			roadmapEpic.Risk = "blocked"
		} else if roadmapEpic.Epic.Status == "done" || roadmapEpic.Progress == 100 {
			roadmapEpic.Risk = "complete"
		}
		roadmapEpics = append(roadmapEpics, roadmapEpic)
	}
	return RoadmapDashboard{
		TeamID:          team.ID,
		TeamName:        team.Name,
		Epics:           roadmapEpics,
		UnassignedCards: unassigned,
		Dependencies:    dependencies,
	}
}

func loadEpics(ctx context.Context, q sqlQueryer, driver Driver, teamID string) ([]Epic, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, workspace_id, team_id, title, slug, description, status, starts_on, target_on
		FROM epics
		WHERE team_id = %s
		ORDER BY COALESCE(target_on, starts_on, created_at), title, id
	`, placeholder(driver, 1)), teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	epics := make([]Epic, 0)
	for rows.Next() {
		epic, err := scanEpic(rows)
		if err != nil {
			return nil, err
		}
		epics = append(epics, epic)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return epics, nil
}

func loadEpic(ctx context.Context, q sqlQueryer, driver Driver, epicID string) (Epic, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, workspace_id, team_id, title, slug, description, status, starts_on, target_on
		FROM epics
		WHERE id = %s
	`, placeholder(driver, 1)), epicID)
	epic, err := scanEpic(row)
	if err != nil {
		return Epic{}, notFoundOrErr(err)
	}
	return epic, nil
}

func scanEpic(scanner cardScanner) (Epic, error) {
	var epic Epic
	var startsOn sql.NullString
	var targetOn sql.NullString
	if err := scanner.Scan(&epic.ID, &epic.WorkspaceID, &epic.TeamID, &epic.Title, &epic.Slug, &epic.Description, &epic.Status, &startsOn, &targetOn); err != nil {
		return Epic{}, err
	}
	epic.StartsOn = startsOn.String
	epic.TargetOn = targetOn.String
	return epic, nil
}

func loadRoadmapCards(ctx context.Context, q sqlQueryer, driver Driver, teamID string) ([]RoadmapCard, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT cards.id, cards.column_id, %s, COALESCE(boards.name, ''), cards.sprint_id, cards.epic_id, cards.title, cards.description, cards.owner_initials, cards.priority, cards.position, %s,
			COALESCE(%s, ''), COALESCE(users.display_name, ''), COALESCE(users.email, ''), columns.name
		FROM cards
		JOIN boards ON boards.id = cards.board_id
		JOIN columns ON columns.id = cards.column_id
		LEFT JOIN users ON users.id = cards.assignee_id
		WHERE boards.team_id = %s
		ORDER BY COALESCE(cards.epic_id, ''), boards.name, columns.position, cards.position, cards.created_at, cards.id
	`, idText(driver, "cards.board_id"), cardDueText(driver), idText(driver, "cards.assignee_id"), placeholder(driver, 1)), teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]RoadmapCard, 0)
	boardCards := make([]BoardCard, 0)
	for rows.Next() {
		card, columnTitle, err := scanRoadmapCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, RoadmapCard{Card: card, ColumnTitle: columnTitle})
		boardCards = append(boardCards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachLabelsToCards(ctx, q, driver, boardCards); err != nil {
		return nil, err
	}
	labelsByCard := make(map[string][]CardLabel)
	for _, card := range boardCards {
		labelsByCard[card.ID] = card.Labels
	}
	for index := range cards {
		cards[index].Card.Labels = labelsByCard[cards[index].Card.ID]
	}
	return cards, nil
}

func scanRoadmapCard(scanner cardScanner) (BoardCard, string, error) {
	var card BoardCard
	var priority string
	var sprintID sql.NullString
	var epicID sql.NullString
	var assigneeID sql.NullString
	var assigneeName sql.NullString
	var assigneeEmail sql.NullString
	var columnTitle string
	if err := scanner.Scan(
		&card.ID,
		&card.ColumnID,
		&card.BoardID,
		&card.BoardName,
		&sprintID,
		&epicID,
		&card.Title,
		&card.Description,
		&card.Owner,
		&priority,
		&card.Position,
		&card.Due,
		&assigneeID,
		&assigneeName,
		&assigneeEmail,
		&columnTitle,
	); err != nil {
		return BoardCard{}, "", err
	}
	card.SprintID = sprintID.String
	card.EpicID = epicID.String
	card.AssigneeID = assigneeID.String
	card.AssigneeName = assigneeName.String
	card.AssigneeEmail = assigneeEmail.String
	if card.AssigneeID != "" {
		card.Owner = card.AssigneeName
		if card.Owner == "" {
			card.Owner = card.AssigneeEmail
		}
	} else {
		card.Owner = ""
	}
	card.Priority = displayPriority(priority)
	card.Labels = []CardLabel{}
	return card, columnTitle, nil
}

func loadCardDependenciesForTeam(ctx context.Context, q sqlQueryer, driver Driver, teamID string) ([]CardDependency, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT card_dependencies.id, card_dependencies.blocked_card_id, blocked.title, card_dependencies.blocker_card_id, blocker.title, card_dependencies.relation_type
		FROM card_dependencies
		JOIN cards blocked ON blocked.id = card_dependencies.blocked_card_id
		JOIN boards blocked_boards ON blocked_boards.id = blocked.board_id
		JOIN cards blocker ON blocker.id = card_dependencies.blocker_card_id
		JOIN boards blocker_boards ON blocker_boards.id = blocker.board_id
		WHERE blocked_boards.team_id = %s AND blocker_boards.team_id = %s
		ORDER BY blocked.title, blocker.title, card_dependencies.id
	`, placeholder(driver, 1), placeholder(driver, 2)), teamID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dependencies := make([]CardDependency, 0)
	for rows.Next() {
		dependency, err := scanCardDependency(rows)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dependencies, nil
}

func loadCardDependency(ctx context.Context, q sqlQueryer, driver Driver, dependencyID string) (CardDependency, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT card_dependencies.id, card_dependencies.blocked_card_id, blocked.title, card_dependencies.blocker_card_id, blocker.title, card_dependencies.relation_type
		FROM card_dependencies
		JOIN cards blocked ON blocked.id = card_dependencies.blocked_card_id
		JOIN cards blocker ON blocker.id = card_dependencies.blocker_card_id
		WHERE card_dependencies.id = %s
	`, placeholder(driver, 1)), dependencyID)
	dependency, err := scanCardDependency(row)
	if err != nil {
		return CardDependency{}, notFoundOrErr(err)
	}
	return dependency, nil
}

func scanCardDependency(scanner cardScanner) (CardDependency, error) {
	var dependency CardDependency
	if err := scanner.Scan(
		&dependency.ID,
		&dependency.BlockedCardID,
		&dependency.BlockedTitle,
		&dependency.BlockerCardID,
		&dependency.BlockerTitle,
		&dependency.RelationType,
	); err != nil {
		return CardDependency{}, err
	}
	return dependency, nil
}

func loadCardTeamScope(ctx context.Context, q sqlQueryer, driver Driver, cardID string) (string, string, string, error) {
	var workspaceID string
	var boardID string
	var teamID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT boards.workspace_id, cards.board_id, boards.team_id
		FROM cards
		JOIN boards ON boards.id = cards.board_id
		WHERE cards.id = %s
	`, placeholder(driver, 1)), cardID).Scan(&workspaceID, &boardID, &teamID); err != nil {
		return "", "", "", notFoundOrErr(err)
	}
	return workspaceID, boardID, teamID, nil
}

func loadCardDependencyTeamID(ctx context.Context, q sqlQueryer, driver Driver, dependencyID string) (string, error) {
	var teamID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT boards.team_id
		FROM card_dependencies
		JOIN cards ON cards.id = card_dependencies.blocked_card_id
		JOIN boards ON boards.id = cards.board_id
		WHERE card_dependencies.id = %s
	`, placeholder(driver, 1)), dependencyID).Scan(&teamID); err != nil {
		return "", notFoundOrErr(err)
	}
	return teamID, nil
}

func normalizeEpicStatus(value string) (string, error) {
	status := strings.ToLower(strings.TrimSpace(value))
	if status == "" {
		status = "planned"
	}
	switch status {
	case "planned", "active", "done":
		return status, nil
	default:
		return "", fmt.Errorf("%w: status must be planned, active, or done", ErrValidation)
	}
}

func uniqueEpicSlug(ctx context.Context, q sqlQueryer, driver Driver, teamID string, title string, excludeEpicID string) (string, error) {
	base := slugify(title)
	if base == "" {
		base = "epic"
	}
	slug := base
	for suffix := 2; ; suffix++ {
		query := fmt.Sprintf("SELECT COUNT(*) FROM epics WHERE team_id = %s AND slug = %s", placeholder(driver, 1), placeholder(driver, 2))
		args := []any{teamID, slug}
		if strings.TrimSpace(excludeEpicID) != "" {
			query += fmt.Sprintf(" AND id <> %s", placeholder(driver, 3))
			args = append(args, excludeEpicID)
		}
		var count int
		if err := q.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, suffix)
	}
}
