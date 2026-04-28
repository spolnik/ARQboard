package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")
var ErrValidation = errors.New("validation failed")

type Board struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Slug      string        `json:"slug"`
	Columns   []BoardColumn `json:"columns"`
	WikiPages []WikiPage    `json:"wikiPages"`
}

type BoardColumn struct {
	ID       string      `json:"id"`
	Title    string      `json:"title"`
	Position int         `json:"position"`
	Cards    []BoardCard `json:"cards"`
}

type BoardCard struct {
	ID          string `json:"id"`
	ColumnID    string `json:"columnId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Priority    string `json:"priority"`
	Due         string `json:"due"`
	Position    int    `json:"position"`
}

type WikiPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type CreateCardParams struct {
	ColumnID      string
	Title         string
	OwnerInitials string
}

type MoveCardParams struct {
	CardID   string
	ColumnID string
	Position int
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
	title    string
	position int
}

type seedCard struct {
	columnTitle string
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
	{title: "Planned", position: 0},
	{title: "In progress", position: 1},
	{title: "Ready for review", position: 2},
}

var defaultCards = []seedCard{
	{
		columnTitle: "Planned",
		title:       "Wire auth session cookie flow",
		description: "Map the session cookie lifecycle, expiry behavior, and local fallback for the first auth pass.",
		owner:       "MS",
		priority:    "high",
		due:         "Apr 30",
		position:    0,
	},
	{
		columnTitle: "Planned",
		title:       "Draft workspace migration fixtures",
		description: "Keep migration examples tiny and readable so the test database can be recreated quickly.",
		owner:       "JR",
		priority:    "normal",
		due:         "May 2",
		position:    1,
	},
	{
		columnTitle: "In progress",
		title:       "Ready for review API shape",
		description: "Lock the first JSON contracts for boards, columns, cards, and move operations before wiring the UI.",
		owner:       "AK",
		priority:    "urgent",
		due:         "Today",
		position:    0,
	},
	{
		columnTitle: "Ready for review",
		title:       "Deployment checklist",
		description: "Document the minimum local and container checks before a branch is pushed for review.",
		owner:       "JL",
		priority:    "normal",
		due:         "May 3",
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
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT board_id FROM columns WHERE id = %s", placeholder(driver, 1)), columnID).Scan(&boardID); err != nil {
		return BoardCard{}, notFoundOrErr(err)
	}

	var position int
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(MAX(position), -1) + 1 FROM cards WHERE column_id = %s", placeholder(driver, 1)), columnID).Scan(&position); err != nil {
		return BoardCard{}, err
	}

	cardID, err := newID()
	if err != nil {
		return BoardCard{}, err
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO cards (id, board_id, column_id, title, description, priority, position, owner_initials, due_label)
		VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
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
	), cardID, boardID, columnID, title, "New card created locally and persisted in the board database.", "normal", position, ownerInitials(params.OwnerInitials), "Later")
	if err != nil {
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
	var currentColumnID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT board_id, column_id FROM cards WHERE id = %s", placeholder(driver, 1)), params.CardID).Scan(&boardID, &currentColumnID); err != nil {
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

	board, err := loadBoard(ctx, tx, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	if err := tx.Commit(); err != nil {
		return Board{}, err
	}

	return board, nil
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
		if err := seedCards(ctx, q, driver, boardID, columnIDs); err != nil {
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
	id, found, err := lookupID(ctx, q, driver, fmt.Sprintf(
		"SELECT id FROM boards WHERE workspace_id = %s AND slug = %s",
		placeholder(driver, 1),
		placeholder(driver, 2),
	), workspaceID, "platform")
	if err != nil || found {
		return id, err
	}

	id, err = newID()
	if err != nil {
		return "", err
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO boards (id, workspace_id, name, slug, description) VALUES (%s, %s, %s, %s, %s)",
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
	), id, workspaceID, "Platform Board", "platform", "Default ARQboard workspace board.")
	return id, err
}

func ensureColumns(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (map[string]string, error) {
	columnIDs := make(map[string]string)
	rows, err := q.QueryContext(ctx, fmt.Sprintf(
		"SELECT id, name FROM columns WHERE board_id = %s",
		placeholder(driver, 1),
	), boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		columnIDs[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, column := range defaultColumns {
		if _, ok := columnIDs[column.title]; ok {
			continue
		}

		id, err := newID()
		if err != nil {
			return nil, err
		}
		_, err = q.ExecContext(ctx, fmt.Sprintf(
			"INSERT INTO columns (id, board_id, name, position) VALUES (%s, %s, %s, %s)",
			placeholder(driver, 1),
			placeholder(driver, 2),
			placeholder(driver, 3),
			placeholder(driver, 4),
		), id, boardID, column.title, column.position)
		if err != nil {
			return nil, err
		}
		columnIDs[column.title] = id
	}

	return columnIDs, nil
}

func seedCards(ctx context.Context, q sqlQueryer, driver Driver, boardID string, columnIDs map[string]string) error {
	for _, card := range defaultCards {
		columnID := columnIDs[card.columnTitle]
		if columnID == "" {
			return fmt.Errorf("%w: seeded column %q not found", ErrValidation, card.columnTitle)
		}

		cardID, err := newID()
		if err != nil {
			return err
		}
		_, err = q.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO cards (id, board_id, column_id, title, description, priority, position, owner_initials, due_label)
			VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
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
		), cardID, boardID, columnID, card.title, card.description, card.priority, card.position, card.owner, card.due)
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
	if err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT id, name, slug FROM boards WHERE id = %s", placeholder(driver, 1)), boardID).Scan(&board.ID, &board.Name, &board.Slug); err != nil {
		return Board{}, notFoundOrErr(err)
	}

	columns, err := loadColumns(ctx, q, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	cardsByColumn, err := loadCardsByColumn(ctx, q, driver, boardID)
	if err != nil {
		return Board{}, err
	}
	for index := range columns {
		columns[index].Cards = cardsByColumn[columns[index].ID]
	}
	board.Columns = columns

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

func loadCardsByColumn(ctx context.Context, q sqlQueryer, driver Driver, boardID string) (map[string][]BoardCard, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, column_id, title, description, owner_initials, priority, position, due_label
		FROM cards
		WHERE board_id = %s
		ORDER BY position, created_at, id
	`, placeholder(driver, 1)), boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cardsByColumn := make(map[string][]BoardCard)
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cardsByColumn[card.ColumnID] = append(cardsByColumn[card.ColumnID], card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cardsByColumn, nil
}

func loadWikiPages(ctx context.Context, q sqlQueryer, driver Driver, boardID string) ([]WikiPage, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, title, slug
		FROM wiki_pages
		WHERE board_id = %s
		ORDER BY title, id
	`, placeholder(driver, 1)), boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []WikiPage
	for rows.Next() {
		var page WikiPage
		if err := rows.Scan(&page.ID, &page.Title, &page.Slug); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pages, nil
}

func loadCard(ctx context.Context, q sqlQueryer, driver Driver, cardID string) (BoardCard, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id, column_id, title, description, owner_initials, priority, position, due_label
		FROM cards
		WHERE id = %s
	`, placeholder(driver, 1)), cardID)

	card, err := scanCard(row)
	if err != nil {
		return BoardCard{}, notFoundOrErr(err)
	}
	return card, nil
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
	if err := scanner.Scan(&card.ID, &card.ColumnID, &card.Title, &card.Description, &card.Owner, &priority, &card.Position, &card.Due); err != nil {
		return BoardCard{}, err
	}
	card.Owner = ownerInitials(card.Owner)
	card.Priority = displayPriority(priority)
	return card, nil
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

func ownerInitials(value string) string {
	owner := strings.ToUpper(strings.TrimSpace(value))
	if owner == "" {
		return "ME"
	}
	if len(owner) > 3 {
		return owner[:3]
	}
	return owner
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
