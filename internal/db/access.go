package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type AccessLevel int

const (
	AccessRead AccessLevel = iota + 1
	AccessWrite
	AccessManage
)

type AccessStore struct {
	Conn *Connection
}

func (store AccessStore) ListTeamsForUser(ctx context.Context, user User) ([]Team, error) {
	if user.IsAdmin {
		return (TeamStore{Conn: store.Conn}).ListTeams(ctx)
	}
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.ID) == "" {
		return nil, ErrUnauthenticated
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	workspaceID, err := ensureWorkspace(ctx, tx, driver)
	if err != nil {
		return nil, err
	}
	if err := ensureAdminWorkspaceMembers(ctx, tx, driver, workspaceID); err != nil {
		return nil, err
	}
	if _, err := ensureDefaultTeam(ctx, tx, driver, workspaceID); err != nil {
		return nil, err
	}

	teams, err := listTeamsForUser(ctx, tx, driver, workspaceID, user.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return teams, nil
}

func (store AccessStore) ListBoardsForUser(ctx context.Context, user User) ([]BoardSummary, error) {
	if user.IsAdmin {
		return (BoardStore{Conn: store.Conn}).ListBoards(ctx)
	}
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.ID) == "" {
		return nil, ErrUnauthenticated
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := ensureDefaultBoard(ctx, tx, driver); err != nil {
		return nil, err
	}
	boards, err := listBoardSummariesForUser(ctx, tx, driver, user.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return boards, nil
}

func (store AccessStore) ListWikiPagesForUser(ctx context.Context, user User) ([]WikiPage, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
	}
	if !user.IsAdmin && strings.TrimSpace(user.ID) == "" {
		return nil, ErrUnauthenticated
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := ensureDefaultBoard(ctx, tx, driver); err != nil {
		return nil, err
	}
	pages, err := listWikiPagesForUser(ctx, tx, driver, user)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return pages, nil
}

func (store AccessStore) AuthorizeTeam(ctx context.Context, user User, teamID string, level AccessLevel) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return fmt.Errorf("%w: teamId is required", ErrValidation)
	}
	sqlDB, driver, err := store.database()
	if err != nil {
		return err
	}
	return authorizeTeam(ctx, sqlDB, driver, user, teamID, level)
}

func (store AccessStore) AuthorizeBoard(ctx context.Context, user User, boardID string, level AccessLevel) error {
	teamID, err := store.resourceTeamID(ctx, "SELECT team_id FROM boards WHERE id = %s", boardID)
	if err != nil {
		return err
	}
	return store.AuthorizeTeam(ctx, user, teamID, level)
}

func (store AccessStore) AuthorizeColumn(ctx context.Context, user User, columnID string, level AccessLevel) error {
	teamID, err := store.resourceTeamID(ctx, `
		SELECT boards.team_id
		FROM columns
		JOIN boards ON boards.id = columns.board_id
		WHERE columns.id = %s
	`, columnID)
	if err != nil {
		return err
	}
	return store.AuthorizeTeam(ctx, user, teamID, level)
}

func (store AccessStore) AuthorizeCard(ctx context.Context, user User, cardID string, level AccessLevel) error {
	teamID, err := store.resourceTeamID(ctx, `
		SELECT boards.team_id
		FROM cards
		JOIN boards ON boards.id = cards.board_id
		WHERE cards.id = %s
	`, cardID)
	if err != nil {
		return err
	}
	return store.AuthorizeTeam(ctx, user, teamID, level)
}

func (store AccessStore) AuthorizeSprint(ctx context.Context, user User, sprintID string, level AccessLevel) error {
	teamID, err := store.resourceTeamID(ctx, "SELECT team_id FROM sprints WHERE id = %s", sprintID)
	if err != nil {
		return err
	}
	return store.AuthorizeTeam(ctx, user, teamID, level)
}

func (store AccessStore) AuthorizeWikiPage(ctx context.Context, user User, pageID string, level AccessLevel) error {
	teamID, err := store.resourceTeamID(ctx, `
		SELECT boards.team_id
		FROM wiki_pages
		JOIN boards ON boards.id = wiki_pages.board_id
		WHERE wiki_pages.id = %s
	`, pageID)
	if err != nil {
		return err
	}
	return store.AuthorizeTeam(ctx, user, teamID, level)
}

func (store AccessStore) database() (*sql.DB, Driver, error) {
	if store.Conn == nil || store.Conn.SQL == nil {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	if store.Conn.Driver != DriverPostgres && store.Conn.Driver != DriverSQLite {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	return store.Conn.SQL, store.Conn.Driver, nil
}

func (store AccessStore) resourceTeamID(ctx context.Context, query string, resourceID string) (string, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return "", fmt.Errorf("%w: resource id is required", ErrValidation)
	}
	sqlDB, driver, err := store.database()
	if err != nil {
		return "", err
	}
	var teamID string
	if err := sqlDB.QueryRowContext(ctx, fmt.Sprintf(query, placeholder(driver, 1)), resourceID).Scan(&teamID); err != nil {
		return "", notFoundOrErr(err)
	}
	return teamID, nil
}

func authorizeTeam(ctx context.Context, q sqlQueryer, driver Driver, user User, teamID string, level AccessLevel) error {
	if _, err := loadTeamWorkspace(ctx, q, driver, teamID); err != nil {
		return err
	}
	if user.IsAdmin {
		return nil
	}
	userID := strings.TrimSpace(user.ID)
	if userID == "" {
		return ErrUnauthenticated
	}

	var role string
	err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT role
		FROM team_members
		WHERE team_id = %s AND user_id = %s
	`, placeholder(driver, 1), placeholder(driver, 2)), teamID, userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrForbidden
		}
		return err
	}
	if !roleAllows(role, level) {
		return ErrForbidden
	}
	return nil
}

func roleAllows(role string, level AccessLevel) bool {
	rank := roleRank(role)
	switch level {
	case AccessRead:
		return rank >= 1
	case AccessWrite:
		return rank >= 2
	case AccessManage:
		return rank >= 3
	default:
		return false
	}
}

func roleRank(role string) int {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return 3
	case "member":
		return 2
	case "viewer":
		return 1
	default:
		return 0
	}
}

func listTeamsForUser(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, userID string) ([]Team, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, teams.name, teams.slug
		FROM teams
		JOIN team_members ON team_members.team_id = teams.id
		WHERE teams.workspace_id = %s AND team_members.user_id = %s
		ORDER BY lower(teams.name), teams.id
	`, idText(driver, "teams.id"), idText(driver, "teams.workspace_id"), placeholder(driver, 1), placeholder(driver, 2)), workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := make([]Team, 0)
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.ID, &team.WorkspaceID, &team.Name, &team.Slug); err != nil {
			return nil, err
		}
		members, err := listTeamMembers(ctx, q, driver, team.ID)
		if err != nil {
			return nil, err
		}
		team.Members = members
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return teams, nil
}

func listBoardSummariesForUser(ctx context.Context, q sqlQueryer, driver Driver, userID string) ([]BoardSummary, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s,
			%s,
			%s,
			boards.name,
			boards.slug,
			COUNT(DISTINCT columns.id) AS column_count,
			COUNT(DISTINCT cards.id) AS card_count
		FROM boards
		JOIN team_members ON team_members.team_id = boards.team_id
		LEFT JOIN columns ON columns.board_id = boards.id
		LEFT JOIN cards ON cards.board_id = boards.id
		WHERE team_members.user_id = %s
		GROUP BY boards.id, boards.workspace_id, boards.team_id, boards.name, boards.slug, boards.created_at
		ORDER BY boards.name, boards.created_at, boards.id
	`,
		idText(driver, "boards.id"),
		idText(driver, "boards.workspace_id"),
		idText(driver, "boards.team_id"),
		placeholder(driver, 1),
	), userID)
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

func listWikiPagesForUser(ctx context.Context, q sqlQueryer, driver Driver, user User) ([]WikiPage, error) {
	join := ""
	where := ""
	args := []any{}
	if !user.IsAdmin {
		join = "JOIN team_members ON team_members.team_id = boards.team_id"
		where = "WHERE team_members.user_id = " + placeholder(driver, 1)
		args = append(args, user.ID)
	}

	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT wiki_pages.id, wiki_pages.title, wiki_pages.slug, wiki_pages.body_markdown
		FROM wiki_pages
		JOIN boards ON boards.id = wiki_pages.board_id
		%s
		%s
		ORDER BY wiki_pages.title, wiki_pages.id
	`, join, where), args...)
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
