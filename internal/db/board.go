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
	ColumnID      string
	Title         string
	OwnerInitials string
}

type UpdateCardParams struct {
	CardID        string
	Title         string
	Description   string
	Priority      string
	OwnerInitials string
	Due           string
}

type MoveCardParams struct {
	CardID   string
	ColumnID string
	Position int
}

type CreateCardCommentParams struct {
	CardID string
	Body   string
}

type CreateWikiPageParams struct {
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
	{title: "Done", position: 3},
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
	var workspaceID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT columns.board_id, boards.workspace_id
		FROM columns
		JOIN boards ON boards.id = columns.board_id
		WHERE columns.id = %s
	`, placeholder(driver, 1)), columnID).Scan(&boardID, &workspaceID); err != nil {
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
	due := strings.TrimSpace(params.Due)
	if due == "" {
		due = "Later"
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

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE cards
		SET title = %s,
			description = %s,
			priority = %s,
			owner_initials = %s,
			due_label = %s,
			updated_at = %s
		WHERE id = %s
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
		placeholder(driver, 5),
		currentTimestamp(driver),
		placeholder(driver, 6),
	), title, strings.TrimSpace(params.Description), priority, ownerInitials(params.OwnerInitials), due, cardID)
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

	boardID, err := ensureDefaultBoard(ctx, tx, driver)
	if err != nil {
		return WikiPage{}, err
	}
	var workspaceID string
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT workspace_id FROM boards WHERE id = %s", placeholder(driver, 1)), boardID).Scan(&workspaceID); err != nil {
		return WikiPage{}, err
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
