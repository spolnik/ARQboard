package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrValidation = errors.New("validation failed")
var ErrForbidden = errors.New("forbidden")

type Board struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspaceId"`
	TeamID      string            `json:"teamId"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Members     []WorkspaceMember `json:"members"`
	Labels      []CardLabel       `json:"labels"`
	Columns     []BoardColumn     `json:"columns"`
	WikiPages   []WikiPage        `json:"wikiPages"`
}

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type BoardSummary struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	TeamID      string `json:"teamId"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	ColumnCount int    `json:"columnCount"`
	CardCount   int    `json:"cardCount"`
}

type BoardColumn struct {
	ID       string      `json:"id"`
	Title    string      `json:"title"`
	Position int         `json:"position"`
	Cards    []BoardCard `json:"cards"`
}

type BoardCard struct {
	ID            string      `json:"id"`
	ColumnID      string      `json:"columnId"`
	BoardID       string      `json:"boardId"`
	BoardName     string      `json:"boardName,omitempty"`
	SprintID      string      `json:"sprintId,omitempty"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Owner         string      `json:"owner"`
	AssigneeID    string      `json:"assigneeId"`
	AssigneeName  string      `json:"assigneeName"`
	AssigneeEmail string      `json:"assigneeEmail"`
	Labels        []CardLabel `json:"labels"`
	Priority      string      `json:"priority"`
	Due           string      `json:"due"`
	Position      int         `json:"position"`
}

type CardLabel struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Name        string `json:"name"`
	Color       string `json:"color"`
}

type WikiPage struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Slug         string `json:"slug"`
	BodyMarkdown string `json:"bodyMarkdown"`
}

type CardDetail struct {
	Card     BoardCard       `json:"card"`
	Comments []CardComment   `json:"comments"`
	Activity []ActivityEvent `json:"activity"`
}

type CardComment struct {
	ID        string `json:"id"`
	CardID    string `json:"cardId"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

type ActivityEvent struct {
	ID        string `json:"id"`
	CardID    string `json:"cardId"`
	EventType string `json:"eventType"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"createdAt"`
}

type CreateCardParams struct {
	ColumnID   string
	Title      string
	AssigneeID string
	LabelNames []string
}

type UpdateCardParams struct {
	CardID      string
	Title       string
	Description string
	Priority    string
	AssigneeID  *string
	Due         string
	LabelNames  []string
}

type MoveCardParams struct {
	CardID   string
	ColumnID string
	Position int
}

type CreateBoardParams struct {
	Name   string
	TeamID string
}

type CreateColumnParams struct {
	BoardID string
	Title   string
}

type UpdateColumnParams struct {
	ColumnID string
	Title    string
}

type CreateCardCommentParams struct {
	CardID string
	Body   string
}

type CreateWikiPageParams struct {
	BoardID      string
	Title        string
	BodyMarkdown string
}

type UpdateWikiPageParams struct {
	PageID       string
	Title        string
	BodyMarkdown string
}

type BoardStore struct {
	Conn *Connection
}

type sqlQueryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type seedColumn struct {
	key      string
	title    string
	position int
}

type seedCard struct {
	columnKey   string
	title       string
	description string
	owner       string
	priority    string
	due         string
	position    int
}

type seedWikiPage struct {
	title string
	slug  string
	body  string
}

var defaultColumns = []seedColumn{
	{key: "planned", title: "Planned", position: 0},
	{key: "in_progress", title: "In progress", position: 1},
	{key: "ready_for_review", title: "Ready for review", position: 2},
	{key: "done", title: "Done", position: 3},
}

var defaultCards = []seedCard{
	{
		columnKey:   "planned",
		title:       "Wire auth session cookie flow",
		description: "Map the session cookie lifecycle, expiry behavior, and local fallback for the first auth pass.",
		owner:       "MS",
		priority:    "high",
		due:         "2026-04-30",
		position:    0,
	},
	{
		columnKey:   "planned",
		title:       "Draft workspace migration fixtures",
		description: "Keep migration examples tiny and readable so the test database can be recreated quickly.",
		owner:       "JR",
		priority:    "normal",
		due:         "2026-05-02",
		position:    1,
	},
	{
		columnKey:   "in_progress",
		title:       "Ready for review API shape",
		description: "Lock the first JSON contracts for boards, columns, cards, and move operations before wiring the UI.",
		owner:       "AK",
		priority:    "urgent",
		due:         "2026-05-01",
		position:    0,
	},
	{
		columnKey:   "ready_for_review",
		title:       "Deployment checklist",
		description: "Document the minimum local and container checks before a branch is pushed for review.",
		owner:       "JL",
		priority:    "normal",
		due:         "2026-05-03",
		position:    0,
	},
}

var defaultWikiPages = []seedWikiPage{
	{title: "Deployment checklist", slug: "deployment-checklist", body: "Deployment checks for local and container releases."},
	{title: "Onboarding notes", slug: "onboarding-notes", body: "Notes for new workspace members."},
	{title: "Incident response", slug: "incident-response", body: "First-response notes for production incidents."},
}

func (store BoardStore) GetDefaultBoard(ctx context.Context) (Board, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return Board{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback()

	boardID, err := ensureDefaultBoard(ctx, tx, driver)
	if err != nil {
		return Board{}, err
	}

	board, err := loadBoard(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	if err := tx.Commit(); err != nil {
		return Board{}, err
	}

	return board, nil
}

func (store BoardStore) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := ensureDefaultBoard(ctx, tx, driver); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s, name, slug
		FROM workspaces
		ORDER BY name, id
	`, idText(driver, "id")))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	workspaces := make([]Workspace, 0)
	for rows.Next() {
		var workspace Workspace
		if err := rows.Scan(&workspace.ID, &workspace.Name, &workspace.Slug); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return workspaces, nil
}

func (store BoardStore) ListBoards(ctx context.Context) ([]BoardSummary, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := ensureDefaultBoard(ctx, tx, driver); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`
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
		GROUP BY boards.id, boards.workspace_id, boards.team_id, boards.name, boards.slug, boards.created_at
		ORDER BY boards.name, boards.created_at, boards.id
	`,
		idText(driver, "boards.id"),
		idText(driver, "boards.workspace_id"),
		idText(driver, "boards.team_id"),
	))
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
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return boards, nil
}

func (store BoardStore) GetBoard(ctx context.Context, boardID string) (Board, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		return Board{}, fmt.Errorf("%w: boardId is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Board{}, err
	}

	return loadBoard(ctx, sqlDB, driver, boardID)
}

func (store BoardStore) CreateBoard(ctx context.Context, params CreateBoardParams) (Board, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Board{}, fmt.Errorf("%w: name is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Board{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback()

	workspaceID, err := ensureWorkspace(ctx, tx, driver)
	if err != nil {
		return Board{}, err
	}
	teamID := strings.TrimSpace(params.TeamID)
	if teamID == "" {
		teamID, err = ensureDefaultTeam(ctx, tx, driver, workspaceID)
		if err != nil {
			return Board{}, err
		}
	} else {
		teamWorkspaceID, err := loadTeamWorkspace(ctx, tx, driver, teamID)
		if err != nil {
			return Board{}, err
		}
		if teamWorkspaceID != workspaceID {
			return Board{}, fmt.Errorf("%w: team must belong to the default workspace", ErrValidation)
		}
	}
	boardID, err := newID()
	if err != nil {
		return Board{}, err
	}
	slug, err := uniqueBoardSlug(ctx, tx, driver, workspaceID, slugify(name))
	if err != nil {
		return Board{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO boards (id, workspace_id, team_id, name, slug, description)
		VALUES (%s, %s, %s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		placeholder(driver, 6),
	), boardID, workspaceID, teamID, name, slug, "")
	if err != nil {
		return Board{}, err
	}

	if _, err := ensureColumns(ctx, tx, driver, boardID); err != nil {
		return Board{}, err
	}
	board, err := loadBoard(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	if err := tx.Commit(); err != nil {
		return Board{}, err
	}

	return board, nil
}

func (store BoardStore) CreateColumn(ctx context.Context, params CreateColumnParams) (Board, error) {
	boardID := strings.TrimSpace(params.BoardID)
	if boardID == "" {
		return Board{}, fmt.Errorf("%w: boardId is required", ErrValidation)
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Board{}, fmt.Errorf("%w: title is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Board{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback()

	var existingBoardID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM boards WHERE id = %s", idText(driver, "id"), placeholder(driver, 1)), boardID).Scan(&existingBoardID); err != nil {
		return Board{}, notFoundOrErr(err)
	}

	var position int
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(MAX(position), -1) + 1 FROM columns WHERE board_id = %s", placeholder(driver, 1)), boardID).Scan(&position); err != nil {
		return Board{}, err
	}
	columnID, err := newID()
	if err != nil {
		return Board{}, err
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO columns (id, board_id, name, position)
		VALUES (%s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
	), columnID, boardID, title, position)
	if err != nil {
		return Board{}, err
	}

	board, err := loadBoard(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	if err := tx.Commit(); err != nil {
		return Board{}, err
	}

	return board, nil
}

func (store BoardStore) UpdateColumn(ctx context.Context, params UpdateColumnParams) (Board, error) {
	columnID := strings.TrimSpace(params.ColumnID)
	if columnID == "" {
		return Board{}, fmt.Errorf("%w: columnId is required", ErrValidation)
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return Board{}, fmt.Errorf("%w: title is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Board{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback()

	var boardID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM columns WHERE id = %s", idText(driver, "board_id"), placeholder(driver, 1)), columnID).Scan(&boardID); err != nil {
		return Board{}, notFoundOrErr(err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE columns
		SET name = %s,
			updated_at = %s
		WHERE id = %s
	`, placeholder(driver, 1), currentTimestamp(driver), placeholder(driver, 2)), title, columnID)
	if err != nil {
		return Board{}, err
	}

	board, err := loadBoard(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	if err := tx.Commit(); err != nil {
		return Board{}, err
	}

	return board, nil
}

func (store BoardStore) CreateCard(ctx context.Context, params CreateCardParams) (BoardCard, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return BoardCard{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	columnID := strings.TrimSpace(params.ColumnID)
	if columnID == "" {
		return BoardCard{}, fmt.Errorf("%w: columnId is required", ErrValidation)
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

	var boardID string
	var workspaceID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT columns.board_id, boards.workspace_id
		FROM columns
		JOIN boards ON boards.id = columns.board_id
		WHERE columns.id = %s
	`, placeholder(driver, 1)), columnID).Scan(&boardID, &workspaceID); err != nil {
		return BoardCard{}, notFoundOrErr(err)
	}
	if err := ensureAdminWorkspaceMembers(ctx, tx, driver, workspaceID); err != nil {
		return BoardCard{}, err
	}

	var position int
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(MAX(position), -1) + 1 FROM cards WHERE column_id = %s", placeholder(driver, 1)), columnID).Scan(&position); err != nil {
		return BoardCard{}, err
	}

	cardID, err := newID()
	if err != nil {
		return BoardCard{}, err
	}
	due := defaultDueDate()
	sprintID, err := ensureActiveSprintForBoard(ctx, tx, driver, workspaceID, boardID)
	if err != nil {
		return BoardCard{}, err
	}
	assigneeID, err := validateCardAssignee(ctx, tx, driver, workspaceID, params.AssigneeID)
	if err != nil {
		return BoardCard{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO cards (id, board_id, column_id, sprint_id, assignee_id, title, description, priority, position, due_at, due_label)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, '')
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
		placeholder(driver, 10),
	), cardID, boardID, columnID, sprintID, nullString(assigneeID), title, "New card created locally and persisted in the board database.", "normal", position, due)
	if err != nil {
		return BoardCard{}, err
	}
	if err := replaceCardLabels(ctx, tx, driver, workspaceID, cardID, params.LabelNames); err != nil {
		return BoardCard{}, err
	}
	if err := appendActivity(ctx, tx, driver, workspaceID, boardID, cardID, "card.created"); err != nil {
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

func (store BoardStore) GetCardDetail(ctx context.Context, cardID string) (CardDetail, error) {
	if strings.TrimSpace(cardID) == "" {
		return CardDetail{}, fmt.Errorf("%w: cardId is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return CardDetail{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return CardDetail{}, err
	}
	defer tx.Rollback()

	detail, err := loadCardDetail(ctx, tx, driver, cardID)
	if err != nil {
		return CardDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return CardDetail{}, err
	}

	return detail, nil
}

func (store BoardStore) UpdateCard(ctx context.Context, params UpdateCardParams) (BoardCard, error) {
	cardID := strings.TrimSpace(params.CardID)
	if cardID == "" {
		return BoardCard{}, fmt.Errorf("%w: cardId is required", ErrValidation)
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return BoardCard{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	priority, err := normalizePriority(params.Priority)
	if err != nil {
		return BoardCard{}, err
	}
	due, err := normalizeDueDate(params.Due)
	if err != nil {
		return BoardCard{}, err
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

	workspaceID, boardID, err := loadCardScope(ctx, tx, driver, cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if err := ensureAdminWorkspaceMembers(ctx, tx, driver, workspaceID); err != nil {
		return BoardCard{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE cards
		SET title = %s,
			description = %s,
			priority = %s,
			due_at = %s,
			due_label = '',
			updated_at = %s
		WHERE id = %s
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		currentTimestamp(driver),
		placeholder(driver, 5),
	), title, strings.TrimSpace(params.Description), priority, due, cardID)
	if err != nil {
		return BoardCard{}, err
	}
	if params.AssigneeID != nil {
		assigneeID, err := validateCardAssignee(ctx, tx, driver, workspaceID, *params.AssigneeID)
		if err != nil {
			return BoardCard{}, err
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE cards
			SET assignee_id = %s,
				updated_at = %s
			WHERE id = %s
		`, placeholder(driver, 1), currentTimestamp(driver), placeholder(driver, 2)), nullString(assigneeID), cardID)
		if err != nil {
			return BoardCard{}, err
		}
	}
	if params.LabelNames != nil {
		if err := replaceCardLabels(ctx, tx, driver, workspaceID, cardID, params.LabelNames); err != nil {
			return BoardCard{}, err
		}
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

func (store BoardStore) MoveCard(ctx context.Context, params MoveCardParams) (Board, error) {
	if strings.TrimSpace(params.CardID) == "" {
		return Board{}, fmt.Errorf("%w: cardId is required", ErrValidation)
	}
	if strings.TrimSpace(params.ColumnID) == "" {
		return Board{}, fmt.Errorf("%w: columnId is required", ErrValidation)
	}
	if params.Position < 0 {
		return Board{}, fmt.Errorf("%w: position must be zero or greater", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Board{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback()

	var boardID string
	var workspaceID string
	var currentColumnID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT cards.board_id, boards.workspace_id, cards.column_id
		FROM cards
		JOIN boards ON boards.id = cards.board_id
		WHERE cards.id = %s
	`, placeholder(driver, 1)), params.CardID).Scan(&boardID, &workspaceID, &currentColumnID); err != nil {
		return Board{}, notFoundOrErr(err)
	}

	var targetBoardID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT board_id FROM columns WHERE id = %s", placeholder(driver, 1)), params.ColumnID).Scan(&targetBoardID); err != nil {
		return Board{}, notFoundOrErr(err)
	}
	if targetBoardID != boardID {
		return Board{}, fmt.Errorf("%w: target column is not on the card board", ErrValidation)
	}

	cardOrder, err := loadCardOrder(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}

	cardOrder[currentColumnID] = removeCardID(cardOrder[currentColumnID], params.CardID)
	targetCards := cardOrder[params.ColumnID]
	position := params.Position
	if position > len(targetCards) {
		position = len(targetCards)
	}
	targetCards = append(targetCards, "")
	copy(targetCards[position+1:], targetCards[position:])
	targetCards[position] = params.CardID
	cardOrder[params.ColumnID] = targetCards

	for columnID, cardIDs := range cardOrder {
		for position, cardID := range cardIDs {
			_, err := tx.ExecContext(ctx, fmt.Sprintf(
				"UPDATE cards SET column_id = %s, position = %s WHERE id = %s",
				placeholder(driver, 1),
				placeholder(driver, 2),
				placeholder(driver, 3),
			), columnID, position, cardID)
			if err != nil {
				return Board{}, err
			}
		}
	}
	if err := appendActivity(ctx, tx, driver, workspaceID, boardID, params.CardID, "card.moved"); err != nil {
		return Board{}, err
	}

	board, err := loadBoard(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	if err := tx.Commit(); err != nil {
		return Board{}, err
	}

	return board, nil
}

func (store BoardStore) CreateCardComment(ctx context.Context, params CreateCardCommentParams) (CardDetail, error) {
	cardID := strings.TrimSpace(params.CardID)
	if cardID == "" {
		return CardDetail{}, fmt.Errorf("%w: cardId is required", ErrValidation)
	}
	body := strings.TrimSpace(params.Body)
	if body == "" {
		return CardDetail{}, fmt.Errorf("%w: body is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return CardDetail{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return CardDetail{}, err
	}
	defer tx.Rollback()

	workspaceID, boardID, err := loadCardScope(ctx, tx, driver, cardID)
	if err != nil {
		return CardDetail{}, err
	}

	commentID, err := newID()
	if err != nil {
		return CardDetail{}, err
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO card_comments (id, card_id, body)
		VALUES (%s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
	), commentID, cardID, body)
	if err != nil {
		return CardDetail{}, err
	}
	if err := appendActivity(ctx, tx, driver, workspaceID, boardID, cardID, "card.commented"); err != nil {
		return CardDetail{}, err
	}

	detail, err := loadCardDetail(ctx, tx, driver, cardID)
	if err != nil {
		return CardDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return CardDetail{}, err
	}

	return detail, nil
}

func (store BoardStore) ListWikiPages(ctx context.Context) ([]WikiPage, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	boardID, err := ensureDefaultBoard(ctx, tx, driver)
	if err != nil {
		return nil, err
	}
	pages, err := loadWikiPages(ctx, tx, driver, boardID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return pages, nil
}

func (store BoardStore) GetWikiPage(ctx context.Context, pageID string) (WikiPage, error) {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return WikiPage{}, fmt.Errorf("%w: pageId is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return WikiPage{}, err
	}

	return loadWikiPage(ctx, sqlDB, driver, pageID)
}

func (store BoardStore) CreateWikiPage(ctx context.Context, params CreateWikiPageParams) (WikiPage, error) {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return WikiPage{}, fmt.Errorf("%w: title is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return WikiPage{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return WikiPage{}, err
	}
	defer tx.Rollback()

	boardID := strings.TrimSpace(params.BoardID)
	if boardID == "" {
		boardID, err = ensureDefaultBoard(ctx, tx, driver)
		if err != nil {
			return WikiPage{}, err
		}
	}
	var workspaceID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT workspace_id FROM boards WHERE id = %s", placeholder(driver, 1)), boardID).Scan(&workspaceID); err != nil {
		return WikiPage{}, notFoundOrErr(err)
	}

	pageID, err := newID()
	if err != nil {
		return WikiPage{}, err
	}
	slug, err := uniqueWikiSlug(ctx, tx, driver, workspaceID, "", slugify(title))
	if err != nil {
		return WikiPage{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO wiki_pages (id, workspace_id, board_id, title, slug, body_markdown)
		VALUES (%s, %s, %s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		placeholder(driver, 6),
	), pageID, workspaceID, boardID, title, slug, strings.TrimSpace(params.BodyMarkdown))
	if err != nil {
		return WikiPage{}, err
	}

	page, err := loadWikiPage(ctx, tx, driver, pageID)
	if err != nil {
		return WikiPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return WikiPage{}, err
	}

	return page, nil
}

func (store BoardStore) UpdateWikiPage(ctx context.Context, params UpdateWikiPageParams) (WikiPage, error) {
	pageID := strings.TrimSpace(params.PageID)
	if pageID == "" {
		return WikiPage{}, fmt.Errorf("%w: pageId is required", ErrValidation)
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return WikiPage{}, fmt.Errorf("%w: title is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return WikiPage{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return WikiPage{}, err
	}
	defer tx.Rollback()

	var workspaceID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT workspace_id FROM wiki_pages WHERE id = %s", placeholder(driver, 1)), pageID).Scan(&workspaceID); err != nil {
		return WikiPage{}, notFoundOrErr(err)
	}
	slug, err := uniqueWikiSlug(ctx, tx, driver, workspaceID, pageID, slugify(title))
	if err != nil {
		return WikiPage{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE wiki_pages
		SET title = %s,
			slug = %s,
			body_markdown = %s,
			updated_at = %s
		WHERE id = %s
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		currentTimestamp(driver),
		placeholder(driver, 4),
	), title, slug, strings.TrimSpace(params.BodyMarkdown), pageID)
	if err != nil {
		return WikiPage{}, err
	}

	page, err := loadWikiPage(ctx, tx, driver, pageID)
	if err != nil {
		return WikiPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return WikiPage{}, err
	}

	return page, nil
}

func (store BoardStore) database() (*sql.DB, Driver, error) {
	if store.Conn == nil || store.Conn.SQL == nil {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	if store.Conn.Driver != DriverPostgres && store.Conn.Driver != DriverSQLite {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	return store.Conn.SQL, store.Conn.Driver, nil
}

func ensureDefaultBoard(ctx context.Context, q sqlQueryer, driver Driver) (string, error) {
	workspaceID, err := ensureWorkspace(ctx, q, driver)
	if err != nil {
		return "", err
	}
	if _, err := ensureDefaultTeam(ctx, q, driver, workspaceID); err != nil {
		return "", err
	}

	boardID, err := ensureBoard(ctx, q, driver, workspaceID)
	if err != nil {
		return "", err
	}

	columnIDs, err := ensureColumns(ctx, q, driver, boardID)
	if err != nil {
		return "", err
	}

	cardCount, err := countRows(ctx, q, driver, "cards", "board_id", boardID)
	if err != nil {
		return "", err
	}
	if cardCount == 0 {
		if err := seedCards(ctx, q, driver, workspaceID, boardID, columnIDs); err != nil {
			return "", err
		}
	}

	wikiCount, err := countRows(ctx, q, driver, "wiki_pages", "workspace_id", workspaceID)
	if err != nil {
		return "", err
	}
	if wikiCount == 0 {
		if err := seedWikiPages(ctx, q, driver, workspaceID, boardID); err != nil {
			return "", err
		}
	}

	return boardID, nil
}

func ensureWorkspace(ctx context.Context, q sqlQueryer, driver Driver) (string, error) {
	id, found, err := lookupID(ctx, q, driver, "SELECT id FROM workspaces WHERE slug = "+placeholder(driver, 1), "platform-engineering")
	if err != nil || found {
		return id, err
	}

	id, err = newID()
	if err != nil {
		return "", err
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO workspaces (id, name, slug) VALUES (%s, %s, %s)",
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
	), id, "Platform Engineering", "platform-engineering")
	return id, err
}

func ensureBoard(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string) (string, error) {
	teamID, err := ensureDefaultTeam(ctx, q, driver, workspaceID)
	if err != nil {
		return "", err
	}
	id, found, err := lookupID(ctx, q, driver, fmt.Sprintf(
		"SELECT id FROM boards WHERE workspace_id = %s AND slug = %s",
		placeholder(driver, 1),
		placeholder(driver, 2),
	), workspaceID, "platform")
	if err != nil {
		return "", err
	}
	if found {
		_, err = q.ExecContext(ctx, fmt.Sprintf("UPDATE boards SET team_id = %s WHERE id = %s AND team_id IS NULL", placeholder(driver, 1), placeholder(driver, 2)), teamID, id)
		return id, err
	}

	id, err = newID()
	if err != nil {
		return "", err
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO boards (id, workspace_id, team_id, name, slug, description) VALUES (%s, %s, %s, %s, %s, %s)",
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		placeholder(driver, 6),
	), id, workspaceID, teamID, "Platform Board", "platform", "Default ARQboard workspace board.")
	return id, err
}

func ensureColumns(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (map[string]string, error) {
	columnIDs := make(map[string]string)
	occupiedPositions := make(map[int]bool)
	nextPosition := 0
	rows, err := q.QueryContext(ctx, fmt.Sprintf(
		"SELECT id, name, position, COALESCE(system_key, '') FROM columns WHERE board_id = %s",
		placeholder(driver, 1),
	), boardID)
	if err != nil {
		return nil, err
	}

	type systemKeyBackfill struct {
		id  string
		key string
	}
	var backfills []systemKeyBackfill
	for rows.Next() {
		var id string
		var name string
		var position int
		var systemKey string
		if err := rows.Scan(&id, &name, &position, &systemKey); err != nil {
			return nil, err
		}
		occupiedPositions[position] = true
		if position >= nextPosition {
			nextPosition = position + 1
		}
		if systemKey == "" {
			systemKey = inferDefaultColumnKey(name, position)
			if systemKey != "" && columnIDs[systemKey] == "" {
				backfills = append(backfills, systemKeyBackfill{id: id, key: systemKey})
			}
		}
		if systemKey != "" && columnIDs[systemKey] == "" {
			columnIDs[systemKey] = id
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	for _, backfill := range backfills {
		_, err := q.ExecContext(ctx, fmt.Sprintf(
			"UPDATE columns SET system_key = %s, updated_at = %s WHERE id = %s",
			placeholder(driver, 1),
			currentTimestamp(driver),
			placeholder(driver, 2),
		), backfill.key, backfill.id)
		if err != nil {
			return nil, err
		}
	}

	for _, column := range defaultColumns {
		if _, ok := columnIDs[column.key]; ok {
			continue
		}

		id, err := newID()
		if err != nil {
			return nil, err
		}
		_, err = q.ExecContext(ctx, fmt.Sprintf(
			"INSERT INTO columns (id, board_id, name, system_key, position) VALUES (%s, %s, %s, %s, %s)",
			placeholder(driver, 1),
			placeholder(driver, 2),
			placeholder(driver, 3),
			placeholder(driver, 4),
			placeholder(driver, 5),
		), id, boardID, column.title, column.key, availableColumnPosition(column.position, occupiedPositions, &nextPosition))
		if err != nil {
			return nil, err
		}
		columnIDs[column.key] = id
	}

	return columnIDs, nil
}

func inferDefaultColumnKey(name string, position int) string {
	for _, column := range defaultColumns {
		if column.position == position {
			return column.key
		}
	}

	switch strings.ToLower(strings.TrimSpace(name)) {
	case "planned", "todo":
		return "planned"
	case "in progress":
		return "in_progress"
	case "ready for review":
		return "ready_for_review"
	case "done":
		return "done"
	default:
		return ""
	}
}

func availableColumnPosition(preferred int, occupied map[int]bool, next *int) int {
	if !occupied[preferred] {
		occupied[preferred] = true
		if preferred >= *next {
			*next = preferred + 1
		}
		return preferred
	}

	for occupied[*next] {
		*next = *next + 1
	}
	position := *next
	occupied[position] = true
	*next = position + 1
	return position
}

func seedCards(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, boardID string, columnIDs map[string]string) error {
	sprintID, err := ensureActiveSprintForBoard(ctx, q, driver, workspaceID, boardID)
	if err != nil {
		return err
	}

	for _, card := range defaultCards {
		columnID := columnIDs[card.columnKey]
		if columnID == "" {
			return fmt.Errorf("%w: seeded column %q not found", ErrValidation, card.columnKey)
		}

		cardID, err := newID()
		if err != nil {
			return err
		}
		_, err = q.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO cards (id, board_id, column_id, sprint_id, title, description, priority, position, owner_initials, due_at, due_label)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, '')
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
			placeholder(driver, 10),
		), cardID, boardID, columnID, sprintID, card.title, card.description, card.priority, card.position, card.owner, card.due)
		if err != nil {
			return err
		}
	}
	return nil
}

func seedWikiPages(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, boardID string) error {
	for _, page := range defaultWikiPages {
		pageID, err := newID()
		if err != nil {
			return err
		}
		_, err = q.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO wiki_pages (id, workspace_id, board_id, title, slug, body_markdown)
			VALUES (%s, %s, %s, %s, %s, %s)
		`,
			placeholder(driver, 1),
			placeholder(driver, 2),
			placeholder(driver, 3),
			placeholder(driver, 4),
			placeholder(driver, 5),
			placeholder(driver, 6),
		), pageID, workspaceID, boardID, page.title, page.slug, page.body)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadBoard(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (Board, error) {
	var board Board
	if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT id, workspace_id, team_id, name, slug FROM boards WHERE id = %s", placeholder(driver, 1)), boardID).Scan(&board.ID, &board.WorkspaceID, &board.TeamID, &board.Name, &board.Slug); err != nil {
		return Board{}, notFoundOrErr(err)
	}
	if err := ensureAdminWorkspaceMembers(ctx, q, driver, board.WorkspaceID); err != nil {
		return Board{}, err
	}

	columns, err := loadColumns(ctx, q, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	activeSprintID, found, err := activeSprintIDForTeam(ctx, q, driver, board.TeamID)
	if err != nil {
		return Board{}, err
	}
	cardsByColumn := make(map[string][]BoardCard)
	if found {
		cardsByColumn, err = loadCardsByColumn(ctx, q, driver, boardID, activeSprintID)
	}
	if err != nil {
		return Board{}, err
	}
	for index := range columns {
		columns[index].Cards = cardsByColumn[columns[index].ID]
	}
	board.Columns = columns
	members, err := listWorkspaceMembers(ctx, q, driver, board.WorkspaceID)
	if err != nil {
		return Board{}, err
	}
	board.Members = members

	labels, err := loadWorkspaceLabels(ctx, q, driver, board.WorkspaceID)
	if err != nil {
		return Board{}, err
	}
	board.Labels = labels

	wikiPages, err := loadWikiPages(ctx, q, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	board.WikiPages = wikiPages

	return board, nil
}

func loadColumns(ctx context.Context, q sqlQueryer, driver Driver, boardID string) ([]BoardColumn, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(
		"SELECT id, name, position FROM columns WHERE board_id = %s ORDER BY position, created_at, id",
		placeholder(driver, 1),
	), boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []BoardColumn
	for rows.Next() {
		var column BoardColumn
		if err := rows.Scan(&column.ID, &column.Title, &column.Position); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func loadCardsByColumn(ctx context.Context, q sqlQueryer, driver Driver, boardID string, sprintID string) (map[string][]BoardCard, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT cards.id, cards.column_id, %s, COALESCE((SELECT boards.name FROM boards WHERE boards.id = cards.board_id), ''), cards.sprint_id, cards.title, cards.description, cards.owner_initials, cards.priority, cards.position, %s,
			COALESCE(%s, ''), COALESCE(users.display_name, ''), COALESCE(users.email, '')
		FROM cards
		LEFT JOIN users ON users.id = cards.assignee_id
		WHERE cards.board_id = %s AND cards.sprint_id = %s
		ORDER BY cards.position, cards.created_at, cards.id
	`, idText(driver, "cards.board_id"), cardDueText(driver), idText(driver, "cards.assignee_id"), placeholder(driver, 1), placeholder(driver, 2)), boardID, sprintID)
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
	cardsByColumn := make(map[string][]BoardCard)
	for _, card := range cards {
		cardsByColumn[card.ColumnID] = append(cardsByColumn[card.ColumnID], card)
	}
	return cardsByColumn, nil
}

func loadWikiPages(ctx context.Context, q sqlQueryer, driver Driver, boardID string) ([]WikiPage, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, title, slug, body_markdown
		FROM wiki_pages
		WHERE board_id = %s
		ORDER BY title, id
	`, placeholder(driver, 1)), boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := make([]WikiPage, 0)
	for rows.Next() {
		var page WikiPage
		if err := rows.Scan(&page.ID, &page.Title, &page.Slug, &page.BodyMarkdown); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pages, nil
}

func loadWikiPage(ctx context.Context, q sqlQueryer, driver Driver, pageID string) (WikiPage, error) {
	var page WikiPage
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, title, slug, body_markdown
		FROM wiki_pages
		WHERE id = %s
	`, placeholder(driver, 1)), pageID).Scan(&page.ID, &page.Title, &page.Slug, &page.BodyMarkdown); err != nil {
		return WikiPage{}, notFoundOrErr(err)
	}
	return page, nil
}

func loadCardDetail(ctx context.Context, q sqlQueryer, driver Driver, cardID string) (CardDetail, error) {
	card, err := loadCard(ctx, q, driver, cardID)
	if err != nil {
		return CardDetail{}, err
	}
	comments, err := loadCardComments(ctx, q, driver, cardID)
	if err != nil {
		return CardDetail{}, err
	}
	activity, err := loadCardActivity(ctx, q, driver, cardID)
	if err != nil {
		return CardDetail{}, err
	}

	return CardDetail{
		Card:     card,
		Comments: comments,
		Activity: activity,
	}, nil
}

func loadCardComments(ctx context.Context, q sqlQueryer, driver Driver, cardID string) ([]CardComment, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, card_id, body, %s
		FROM card_comments
		WHERE card_id = %s
		ORDER BY created_at, id
	`, timeText(driver, "created_at"), placeholder(driver, 1)), cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]CardComment, 0)
	for rows.Next() {
		var comment CardComment
		if err := rows.Scan(&comment.ID, &comment.CardID, &comment.Body, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return comments, nil
}

func loadCardActivity(ctx context.Context, q sqlQueryer, driver Driver, cardID string) ([]ActivityEvent, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, card_id, event_type, %s
		FROM activity_events
		WHERE card_id = %s
		ORDER BY created_at DESC, id DESC
	`, timeText(driver, "created_at"), placeholder(driver, 1)), cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]ActivityEvent, 0)
	for rows.Next() {
		var event ActivityEvent
		if err := rows.Scan(&event.ID, &event.CardID, &event.EventType, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Summary = activitySummary(event.EventType)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func loadCard(ctx context.Context, q sqlQueryer, driver Driver, cardID string) (BoardCard, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT cards.id, cards.column_id, %s, COALESCE((SELECT boards.name FROM boards WHERE boards.id = cards.board_id), ''), cards.sprint_id, cards.title, cards.description, cards.owner_initials, cards.priority, cards.position, %s,
			COALESCE(%s, ''), COALESCE(users.display_name, ''), COALESCE(users.email, '')
		FROM cards
		LEFT JOIN users ON users.id = cards.assignee_id
		WHERE cards.id = %s
	`, idText(driver, "cards.board_id"), cardDueText(driver), idText(driver, "cards.assignee_id"), placeholder(driver, 1)), cardID)

	card, err := scanCard(row)
	if err != nil {
		return BoardCard{}, notFoundOrErr(err)
	}
	labels, err := loadCardLabels(ctx, q, driver, card.ID)
	if err != nil {
		return BoardCard{}, err
	}
	card.Labels = labels
	return card, nil
}

func loadCardScope(ctx context.Context, q sqlQueryer, driver Driver, cardID string) (string, string, error) {
	var workspaceID string
	var boardID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT boards.workspace_id, cards.board_id
		FROM cards
		JOIN boards ON boards.id = cards.board_id
		WHERE cards.id = %s
	`, placeholder(driver, 1)), cardID).Scan(&workspaceID, &boardID); err != nil {
		return "", "", notFoundOrErr(err)
	}
	return workspaceID, boardID, nil
}

func appendActivity(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, boardID string, cardID string, eventType string) error {
	eventID, err := newID()
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO activity_events (id, workspace_id, board_id, card_id, event_type, payload)
		VALUES (%s, %s, %s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		jsonPlaceholder(driver, 6),
	), eventID, workspaceID, boardID, cardID, eventType, "{}")
	return err
}

func loadCardOrder(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (map[string][]string, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(
		"SELECT id, column_id FROM cards WHERE board_id = %s ORDER BY column_id, position, created_at, id",
		placeholder(driver, 1),
	), boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cardOrder := make(map[string][]string)
	for rows.Next() {
		var cardID string
		var columnID string
		if err := rows.Scan(&cardID, &columnID); err != nil {
			return nil, err
		}
		cardOrder[columnID] = append(cardOrder[columnID], cardID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cardOrder, nil
}

type cardScanner interface {
	Scan(...any) error
}

func scanCard(scanner cardScanner) (BoardCard, error) {
	var card BoardCard
	var priority string
	var sprintID sql.NullString
	var assigneeID sql.NullString
	var assigneeName sql.NullString
	var assigneeEmail sql.NullString
	if err := scanner.Scan(
		&card.ID,
		&card.ColumnID,
		&card.BoardID,
		&card.BoardName,
		&sprintID,
		&card.Title,
		&card.Description,
		&card.Owner,
		&priority,
		&card.Position,
		&card.Due,
		&assigneeID,
		&assigneeName,
		&assigneeEmail,
	); err != nil {
		return BoardCard{}, err
	}
	card.SprintID = sprintID.String
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
	return card, nil
}

func loadWorkspaceLabels(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string) ([]CardLabel, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, name, color
		FROM labels
		WHERE workspace_id = %s
		ORDER BY lower(name), id
	`, idText(driver, "id"), idText(driver, "workspace_id"), placeholder(driver, 1)), workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	labels := make([]CardLabel, 0)
	for rows.Next() {
		var label CardLabel
		if err := rows.Scan(&label.ID, &label.WorkspaceID, &label.Name, &label.Color); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return labels, nil
}

func loadCardLabels(ctx context.Context, q sqlQueryer, driver Driver, cardID string) ([]CardLabel, error) {
	cards := []BoardCard{{ID: cardID}}
	if err := attachLabelsToCards(ctx, q, driver, cards); err != nil {
		return nil, err
	}
	return cards[0].Labels, nil
}

func attachLabelsToCards(ctx context.Context, q sqlQueryer, driver Driver, cards []BoardCard) error {
	if len(cards) == 0 {
		return nil
	}
	cardIDs := make([]string, 0, len(cards))
	seen := make(map[string]bool)
	for _, card := range cards {
		if card.ID == "" || seen[card.ID] {
			continue
		}
		seen[card.ID] = true
		cardIDs = append(cardIDs, card.ID)
	}
	if len(cardIDs) == 0 {
		return nil
	}

	args := make([]any, len(cardIDs))
	for index, cardID := range cardIDs {
		args[index] = cardID
	}
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, %s, labels.name, labels.color
		FROM card_labels
		JOIN labels ON labels.id = card_labels.label_id
		WHERE card_labels.card_id IN (%s)
		ORDER BY lower(labels.name), labels.id
	`,
		idText(driver, "card_labels.card_id"),
		idText(driver, "labels.id"),
		idText(driver, "labels.workspace_id"),
		placeholders(driver, len(cardIDs)),
	), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	labelsByCard := make(map[string][]CardLabel)
	for rows.Next() {
		var cardID string
		var label CardLabel
		if err := rows.Scan(&cardID, &label.ID, &label.WorkspaceID, &label.Name, &label.Color); err != nil {
			return err
		}
		labelsByCard[cardID] = append(labelsByCard[cardID], label)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range cards {
		cards[index].Labels = labelsByCard[cards[index].ID]
		if cards[index].Labels == nil {
			cards[index].Labels = []CardLabel{}
		}
	}
	return nil
}

func replaceCardLabels(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, cardID string, names []string) error {
	_, err := q.ExecContext(ctx, fmt.Sprintf("DELETE FROM card_labels WHERE card_id = %s", placeholder(driver, 1)), cardID)
	if err != nil {
		return err
	}

	labels, err := ensureLabels(ctx, q, driver, workspaceID, names)
	if err != nil {
		return err
	}
	for _, label := range labels {
		cardLabelID, err := newID()
		if err != nil {
			return err
		}
		_, err = q.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO card_labels (id, card_id, label_id)
			VALUES (%s, %s, %s)
		`, placeholder(driver, 1), placeholder(driver, 2), placeholder(driver, 3)), cardLabelID, cardID, label.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureLabels(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, names []string) ([]CardLabel, error) {
	normalized := normalizeLabelNames(names)
	labels := make([]CardLabel, 0, len(normalized))
	for _, name := range normalized {
		labelID, found, err := lookupID(ctx, q, driver, fmt.Sprintf(`
			SELECT id FROM labels
			WHERE workspace_id = %s AND lower(name) = %s
		`, placeholder(driver, 1), placeholder(driver, 2)), workspaceID, strings.ToLower(name))
		if err != nil {
			return nil, err
		}
		if !found {
			labelID, err = newID()
			if err != nil {
				return nil, err
			}
			_, err = q.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO labels (id, workspace_id, name, color)
				VALUES (%s, %s, %s, %s)
			`, placeholder(driver, 1), placeholder(driver, 2), placeholder(driver, 3), placeholder(driver, 4)), labelID, workspaceID, name, labelColor(name))
			if err != nil {
				return nil, err
			}
		}

		label, err := loadLabel(ctx, q, driver, labelID)
		if err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, nil
}

func loadLabel(ctx context.Context, q sqlQueryer, driver Driver, labelID string) (CardLabel, error) {
	var label CardLabel
	err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, name, color
		FROM labels
		WHERE id = %s
	`, idText(driver, "id"), idText(driver, "workspace_id"), placeholder(driver, 1)), labelID).Scan(&label.ID, &label.WorkspaceID, &label.Name, &label.Color)
	if err != nil {
		return CardLabel{}, notFoundOrErr(err)
	}
	return label, nil
}

func validateCardAssignee(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, assigneeID string) (string, error) {
	assigneeID = strings.TrimSpace(assigneeID)
	if assigneeID == "" {
		return "", nil
	}

	var userID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s
		FROM workspace_members
		WHERE workspace_id = %s AND user_id = %s
	`, idText(driver, "user_id"), placeholder(driver, 1), placeholder(driver, 2)), workspaceID, assigneeID).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: assignee must be a workspace member", ErrValidation)
		}
		return "", err
	}
	return userID, nil
}

func lookupID(ctx context.Context, q sqlQueryer, _ Driver, query string, args ...any) (string, bool, error) {
	var id string
	if err := q.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

func countRows(ctx context.Context, q sqlQueryer, driver Driver, table string, column string, value string) (int, error) {
	var count int
	if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s", table, column, placeholder(driver, 1)), value).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func removeCardID(cardIDs []string, target string) []string {
	next := cardIDs[:0]
	for _, cardID := range cardIDs {
		if cardID != target {
			next = append(next, cardID)
		}
	}
	return next
}

func normalizeLabelNames(names []string) []string {
	normalized := make([]string, 0, len(names))
	seen := make(map[string]bool)
	for _, raw := range names {
		for _, part := range strings.Split(raw, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if len(name) > 40 {
				name = name[:40]
			}
			key := strings.ToLower(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			normalized = append(normalized, name)
		}
	}
	return normalized
}

func labelColor(name string) string {
	palette := []string{"#0f766e", "#2563eb", "#7c3aed", "#be123c", "#b45309", "#4d7c0f", "#0369a1", "#6d28d9"}
	hash := 0
	for _, char := range strings.ToLower(name) {
		hash = (hash*31 + int(char)) % len(palette)
	}
	return palette[hash]
}

func nullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func normalizePriority(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "normal":
		return "normal", nil
	case "low":
		return "low", nil
	case "high":
		return "high", nil
	case "urgent":
		return "urgent", nil
	default:
		return "", fmt.Errorf("%w: priority must be low, normal, high, or urgent", ErrValidation)
	}
}

func displayPriority(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return "Low"
	case "high":
		return "High"
	case "urgent":
		return "Urgent"
	default:
		return "Normal"
	}
}

func uniqueBoardSlug(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, base string) (string, error) {
	if base == "" {
		base = "board"
	}

	for index := 0; index < 100; index++ {
		slug := base
		if index > 0 {
			slug = fmt.Sprintf("%s-%d", base, index+1)
		}

		var existingID string
		if err := q.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT id FROM boards WHERE workspace_id = %s AND slug = %s",
			placeholder(driver, 1),
			placeholder(driver, 2),
		), workspaceID, slug).Scan(&existingID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return slug, nil
			}
			return "", err
		}
	}

	return "", fmt.Errorf("%w: board slug is unavailable", ErrValidation)
}

func uniqueWikiSlug(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, excludePageID string, base string) (string, error) {
	if base == "" {
		base = "page"
	}

	for index := 0; index < 100; index++ {
		slug := base
		if index > 0 {
			slug = fmt.Sprintf("%s-%d", base, index+1)
		}

		args := []any{workspaceID, slug}
		query := fmt.Sprintf(
			"SELECT id FROM wiki_pages WHERE workspace_id = %s AND slug = %s",
			placeholder(driver, 1),
			placeholder(driver, 2),
		)
		if excludePageID != "" {
			query = fmt.Sprintf("%s AND id <> %s", query, placeholder(driver, 3))
			args = append(args, excludePageID)
		}

		var existingID string
		if err := q.QueryRowContext(ctx, query, args...).Scan(&existingID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return slug, nil
			}
			return "", err
		}
	}

	return "", fmt.Errorf("%w: wiki slug is unavailable", ErrValidation)
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false

	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if builder.Len() > 0 && !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func activitySummary(eventType string) string {
	switch eventType {
	case "card.created":
		return "Card created"
	case "card.updated":
		return "Card updated"
	case "card.moved":
		return "Card moved"
	case "card.commented":
		return "Comment added"
	case "card.sprint_assigned":
		return "Sprint assignment updated"
	default:
		return "Activity recorded"
	}
}

func currentTimestamp(driver Driver) string {
	if driver == DriverPostgres {
		return "now()"
	}
	return "datetime('now')"
}

func cardDueText(driver Driver) string {
	if driver == DriverPostgres {
		return "COALESCE(due_at::date::text, NULLIF(due_label, ''), '')"
	}
	return "COALESCE(substr(due_at, 1, 10), NULLIF(due_label, ''), '')"
}

func normalizeDueDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: due date is required", ErrValidation)
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", fmt.Errorf("%w: due date must use YYYY-MM-DD", ErrValidation)
	}
	return parsed.Format("2006-01-02"), nil
}

func defaultDueDate() string {
	return time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
}

func timeText(driver Driver, column string) string {
	if driver == DriverPostgres {
		return column + "::text"
	}
	return column
}

func jsonPlaceholder(driver Driver, index int) string {
	if driver == DriverPostgres {
		return fmt.Sprintf("$%d::jsonb", index)
	}
	return "?"
}

func notFoundOrErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func placeholder(driver Driver, index int) string {
	if driver == DriverPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func placeholders(driver Driver, count int) string {
	values := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		values = append(values, placeholder(driver, index))
	}
	return strings.Join(values, ", ")
}
