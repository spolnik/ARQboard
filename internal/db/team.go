package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type WorkspaceMember struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	IsAdmin     bool   `json:"isAdmin"`
}

type Team struct {
	ID          string       `json:"id"`
	WorkspaceID string       `json:"workspaceId"`
	Name        string       `json:"name"`
	Slug        string       `json:"slug"`
	Members     []TeamMember `json:"members"`
}

type TeamMember struct {
	ID          string `json:"id"`
	TeamID      string `json:"teamId"`
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	IsAdmin     bool   `json:"isAdmin"`
}

type CreateWorkspaceMemberParams struct {
	Email       string
	DisplayName string
	Password    string
	Role        string
}

type CreateTeamParams struct {
	Name string
}

type AddTeamMemberParams struct {
	TeamID string
	UserID string
	Role   string
}

type UpdateWorkspaceMemberParams struct {
	MemberID string
	Role     string
}

type TeamStore struct {
	Conn *Connection
}

func (store TeamStore) ListTeams(ctx context.Context) ([]Team, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
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
	teams, err := listTeams(ctx, tx, driver, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return teams, nil
}

func (store TeamStore) CreateTeam(ctx context.Context, params CreateTeamParams) (Team, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return Team{}, fmt.Errorf("%w: team name is required", ErrValidation)
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Team{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Team{}, err
	}
	defer tx.Rollback()

	workspaceID, err := ensureWorkspace(ctx, tx, driver)
	if err != nil {
		return Team{}, err
	}
	if err := ensureAdminWorkspaceMembers(ctx, tx, driver, workspaceID); err != nil {
		return Team{}, err
	}
	teamID, err := newID()
	if err != nil {
		return Team{}, err
	}
	slug, err := uniqueTeamSlug(ctx, tx, driver, workspaceID, slugify(name))
	if err != nil {
		return Team{}, err
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO teams (id, workspace_id, name, slug)
		VALUES (%s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
	), teamID, workspaceID, name, slug)
	if err != nil {
		return Team{}, err
	}
	if err := seedTeamMembersFromWorkspace(ctx, tx, driver, workspaceID, teamID); err != nil {
		return Team{}, err
	}
	team, err := loadTeam(ctx, tx, driver, teamID)
	if err != nil {
		return Team{}, err
	}
	if err := tx.Commit(); err != nil {
		return Team{}, err
	}
	return team, nil
}

func (store TeamStore) AddTeamMember(ctx context.Context, params AddTeamMemberParams) (Team, error) {
	teamID := strings.TrimSpace(params.TeamID)
	userID := strings.TrimSpace(params.UserID)
	if teamID == "" || userID == "" {
		return Team{}, fmt.Errorf("%w: teamId and userId are required", ErrValidation)
	}
	role, err := normalizeWorkspaceRole(params.Role)
	if err != nil {
		return Team{}, err
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return Team{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Team{}, err
	}
	defer tx.Rollback()

	workspaceID, err := loadTeamWorkspace(ctx, tx, driver, teamID)
	if err != nil {
		return Team{}, err
	}
	if _, found, err := lookupID(ctx, tx, driver, fmt.Sprintf(`
		SELECT id
		FROM workspace_members
		WHERE workspace_id = %s AND user_id = %s
	`, placeholder(driver, 1), placeholder(driver, 2)), workspaceID, userID); err != nil {
		return Team{}, err
	} else if !found {
		return Team{}, fmt.Errorf("%w: user must be a workspace member before joining a team", ErrValidation)
	}

	memberID, found, err := lookupID(ctx, tx, driver, fmt.Sprintf(`
		SELECT id
		FROM team_members
		WHERE team_id = %s AND user_id = %s
	`, placeholder(driver, 1), placeholder(driver, 2)), teamID, userID)
	if err != nil {
		return Team{}, err
	}
	if found {
		_, err = tx.ExecContext(ctx, fmt.Sprintf("UPDATE team_members SET role = %s WHERE id = %s", placeholder(driver, 1), placeholder(driver, 2)), role, memberID)
	} else {
		memberID, err = newID()
		if err != nil {
			return Team{}, err
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO team_members (id, team_id, user_id, role)
			VALUES (%s, %s, %s, %s)
		`,
			placeholder(driver, 1),
			placeholder(driver, 2),
			placeholder(driver, 3),
			placeholder(driver, 4),
		), memberID, teamID, userID, role)
	}
	if err != nil {
		return Team{}, err
	}

	team, err := loadTeam(ctx, tx, driver, teamID)
	if err != nil {
		return Team{}, err
	}
	if err := tx.Commit(); err != nil {
		return Team{}, err
	}
	return team, nil
}

func (store TeamStore) ListWorkspaceMembers(ctx context.Context) ([]WorkspaceMember, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return nil, err
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

	members, err := listWorkspaceMembers(ctx, tx, driver, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return members, nil
}

func (store TeamStore) CreateWorkspaceMember(ctx context.Context, params CreateWorkspaceMemberParams) (WorkspaceMember, error) {
	email := normalizeEmail(params.Email)
	if email == "" || !strings.Contains(email, "@") {
		return WorkspaceMember{}, fmt.Errorf("%w: email is required", ErrValidation)
	}
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return WorkspaceMember{}, fmt.Errorf("%w: displayName is required", ErrValidation)
	}
	role, err := normalizeWorkspaceRole(params.Role)
	if err != nil {
		return WorkspaceMember{}, err
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return WorkspaceMember{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return WorkspaceMember{}, err
	}
	defer tx.Rollback()

	workspaceID, err := ensureWorkspace(ctx, tx, driver)
	if err != nil {
		return WorkspaceMember{}, err
	}
	if err := ensureAdminWorkspaceMembers(ctx, tx, driver, workspaceID); err != nil {
		return WorkspaceMember{}, err
	}

	userID, found, err := lookupID(ctx, tx, driver, "SELECT id FROM users WHERE lower(email) = "+placeholder(driver, 1), email)
	if err != nil {
		return WorkspaceMember{}, err
	}
	if !found {
		if len(params.Password) < 12 {
			return WorkspaceMember{}, fmt.Errorf("%w: password must be at least 12 characters", ErrValidation)
		}
		userID, err = newID()
		if err != nil {
			return WorkspaceMember{}, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
		if err != nil {
			return WorkspaceMember{}, err
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO users (id, email, password_hash, display_name, is_admin)
			VALUES (%s, %s, %s, %s, %s)
		`,
			placeholder(driver, 1),
			placeholder(driver, 2),
			placeholder(driver, 3),
			placeholder(driver, 4),
			placeholder(driver, 5),
		), userID, email, string(hash), displayName, false)
		if err != nil {
			return WorkspaceMember{}, err
		}
	} else {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE users
			SET display_name = %s,
				updated_at = %s
			WHERE id = %s
		`, placeholder(driver, 1), currentTimestamp(driver), placeholder(driver, 2)), displayName, userID)
		if err != nil {
			return WorkspaceMember{}, err
		}
	}

	memberID, found, err := lookupID(ctx, tx, driver, fmt.Sprintf(`
		SELECT id FROM workspace_members
		WHERE workspace_id = %s AND user_id = %s
	`, placeholder(driver, 1), placeholder(driver, 2)), workspaceID, userID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	if found {
		_, err = tx.ExecContext(ctx, fmt.Sprintf("UPDATE workspace_members SET role = %s WHERE id = %s", placeholder(driver, 1), placeholder(driver, 2)), role, memberID)
		if err != nil {
			return WorkspaceMember{}, err
		}
	} else {
		memberID, err = newID()
		if err != nil {
			return WorkspaceMember{}, err
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO workspace_members (id, workspace_id, user_id, role)
			VALUES (%s, %s, %s, %s)
		`,
			placeholder(driver, 1),
			placeholder(driver, 2),
			placeholder(driver, 3),
			placeholder(driver, 4),
		), memberID, workspaceID, userID, role)
		if err != nil {
			return WorkspaceMember{}, err
		}
	}
	defaultTeamID, err := ensureDefaultTeam(ctx, tx, driver, workspaceID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	teamMemberID, err := newID()
	if err != nil {
		return WorkspaceMember{}, err
	}
	if driver == DriverPostgres {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO team_members (id, team_id, user_id, role)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role
		`, teamMemberID, defaultTeamID, userID, role)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO team_members (id, team_id, user_id, role)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(team_id, user_id) DO UPDATE SET role = excluded.role
		`, teamMemberID, defaultTeamID, userID, role)
	}
	if err != nil {
		return WorkspaceMember{}, err
	}

	member, err := loadWorkspaceMember(ctx, tx, driver, memberID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkspaceMember{}, err
	}
	return member, nil
}

func (store TeamStore) UpdateWorkspaceMember(ctx context.Context, params UpdateWorkspaceMemberParams) (WorkspaceMember, error) {
	memberID := strings.TrimSpace(params.MemberID)
	if memberID == "" {
		return WorkspaceMember{}, fmt.Errorf("%w: memberId is required", ErrValidation)
	}
	role, err := normalizeWorkspaceRole(params.Role)
	if err != nil {
		return WorkspaceMember{}, err
	}

	sqlDB, driver, err := store.database()
	if err != nil {
		return WorkspaceMember{}, err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return WorkspaceMember{}, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE workspace_members SET role = %s WHERE id = %s", placeholder(driver, 1), placeholder(driver, 2)), role, memberID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return WorkspaceMember{}, err
	}
	if affected == 0 {
		return WorkspaceMember{}, ErrNotFound
	}

	member, err := loadWorkspaceMember(ctx, tx, driver, memberID)
	if err != nil {
		return WorkspaceMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkspaceMember{}, err
	}
	return member, nil
}

func (store TeamStore) database() (*sql.DB, Driver, error) {
	if store.Conn == nil || store.Conn.SQL == nil {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	if store.Conn.Driver != DriverPostgres && store.Conn.Driver != DriverSQLite {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	return store.Conn.SQL, store.Conn.Driver, nil
}

func ensureAdminWorkspaceMembers(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string) error {
	rows, err := q.QueryContext(ctx, "SELECT id FROM users WHERE is_admin = "+truthLiteral(driver))
	if err != nil {
		return err
	}
	defer rows.Close()

	var adminIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		adminIDs = append(adminIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, userID := range adminIDs {
		memberID, err := newID()
		if err != nil {
			return err
		}
		if driver == DriverPostgres {
			_, err = q.ExecContext(ctx, `
				INSERT INTO workspace_members (id, workspace_id, user_id, role)
				VALUES ($1, $2, $3, 'owner')
				ON CONFLICT (workspace_id, user_id) DO NOTHING
			`, memberID, workspaceID, userID)
		} else {
			_, err = q.ExecContext(ctx, `
				INSERT OR IGNORE INTO workspace_members (id, workspace_id, user_id, role)
				VALUES (?, ?, ?, 'owner')
			`, memberID, workspaceID, userID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func listWorkspaceMembers(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string) ([]WorkspaceMember, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s,
			%s,
			%s,
			users.email,
			users.display_name,
			workspace_members.role,
			users.is_admin
		FROM workspace_members
		JOIN users ON users.id = workspace_members.user_id
		WHERE workspace_members.workspace_id = %s
		ORDER BY
			CASE workspace_members.role
				WHEN 'owner' THEN 0
				WHEN 'admin' THEN 1
				WHEN 'member' THEN 2
				ELSE 3
			END,
			lower(users.email)
	`,
		idText(driver, "workspace_members.id"),
		idText(driver, "workspace_members.workspace_id"),
		idText(driver, "workspace_members.user_id"),
		placeholder(driver, 1),
	), workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]WorkspaceMember, 0)
	for rows.Next() {
		member, err := scanWorkspaceMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func loadWorkspaceMember(ctx context.Context, q sqlQueryer, driver Driver, memberID string) (WorkspaceMember, error) {
	row := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s,
			%s,
			%s,
			users.email,
			users.display_name,
			workspace_members.role,
			users.is_admin
		FROM workspace_members
		JOIN users ON users.id = workspace_members.user_id
		WHERE workspace_members.id = %s
	`,
		idText(driver, "workspace_members.id"),
		idText(driver, "workspace_members.workspace_id"),
		idText(driver, "workspace_members.user_id"),
		placeholder(driver, 1),
	), memberID)
	member, err := scanWorkspaceMember(row)
	if err != nil {
		return WorkspaceMember{}, notFoundOrErr(err)
	}
	return member, nil
}

func scanWorkspaceMember(scanner cardScanner) (WorkspaceMember, error) {
	var member WorkspaceMember
	if err := scanner.Scan(&member.ID, &member.WorkspaceID, &member.UserID, &member.Email, &member.DisplayName, &member.Role, &member.IsAdmin); err != nil {
		return WorkspaceMember{}, err
	}
	return member, nil
}

func ensureDefaultTeam(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string) (string, error) {
	teamID, found, err := lookupID(ctx, q, driver, fmt.Sprintf(`
		SELECT id
		FROM teams
		WHERE workspace_id = %s
		ORDER BY created_at, id
		LIMIT 1
	`, placeholder(driver, 1)), workspaceID)
	if err != nil || found {
		return teamID, err
	}

	var name string
	var slug string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT name, slug
		FROM workspaces
		WHERE id = %s
	`, placeholder(driver, 1)), workspaceID).Scan(&name, &slug); err != nil {
		return "", err
	}
	teamID, err = newID()
	if err != nil {
		return "", err
	}
	_, err = q.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO teams (id, workspace_id, name, slug)
		VALUES (%s, %s, %s, %s)
	`,
		placeholder(driver, 1),
		placeholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
	), teamID, workspaceID, name, slug)
	if err != nil {
		return "", err
	}
	if err := seedTeamMembersFromWorkspace(ctx, q, driver, workspaceID, teamID); err != nil {
		return "", err
	}
	return teamID, nil
}

func seedTeamMembersFromWorkspace(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, teamID string) error {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT user_id, role
		FROM workspace_members
		WHERE workspace_id = %s
	`, placeholder(driver, 1)), workspaceID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		var role string
		if err := rows.Scan(&userID, &role); err != nil {
			return err
		}
		memberID, err := newID()
		if err != nil {
			return err
		}
		if driver == DriverPostgres {
			_, err = q.ExecContext(ctx, `
				INSERT INTO team_members (id, team_id, user_id, role)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (team_id, user_id) DO NOTHING
			`, memberID, teamID, userID, role)
		} else {
			_, err = q.ExecContext(ctx, `
				INSERT OR IGNORE INTO team_members (id, team_id, user_id, role)
				VALUES (?, ?, ?, ?)
			`, memberID, teamID, userID, role)
		}
		if err != nil {
			return err
		}
	}
	return rows.Err()
}

func listTeams(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string) ([]Team, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, name, slug
		FROM teams
		WHERE workspace_id = %s
		ORDER BY lower(name), id
	`, idText(driver, "id"), idText(driver, "workspace_id"), placeholder(driver, 1)), workspaceID)
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

func loadTeam(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (Team, error) {
	var team Team
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s, %s, name, slug
		FROM teams
		WHERE id = %s
	`, idText(driver, "id"), idText(driver, "workspace_id"), placeholder(driver, 1)), teamID).Scan(&team.ID, &team.WorkspaceID, &team.Name, &team.Slug); err != nil {
		return Team{}, notFoundOrErr(err)
	}
	members, err := listTeamMembers(ctx, q, driver, team.ID)
	if err != nil {
		return Team{}, err
	}
	team.Members = members
	return team, nil
}

func listTeamMembers(ctx context.Context, q sqlQueryer, driver Driver, teamID string) ([]TeamMember, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s,
			%s,
			%s,
			users.email,
			users.display_name,
			team_members.role,
			users.is_admin
		FROM team_members
		JOIN users ON users.id = team_members.user_id
		WHERE team_members.team_id = %s
		ORDER BY
			CASE team_members.role
				WHEN 'owner' THEN 0
				WHEN 'admin' THEN 1
				WHEN 'member' THEN 2
				ELSE 3
			END,
			lower(users.display_name),
			lower(users.email)
	`,
		idText(driver, "team_members.id"),
		idText(driver, "team_members.team_id"),
		idText(driver, "team_members.user_id"),
		placeholder(driver, 1),
	), teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]TeamMember, 0)
	for rows.Next() {
		var member TeamMember
		if err := rows.Scan(&member.ID, &member.TeamID, &member.UserID, &member.Email, &member.DisplayName, &member.Role, &member.IsAdmin); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func loadTeamWorkspace(ctx context.Context, q sqlQueryer, driver Driver, teamID string) (string, error) {
	var workspaceID string
	if err := q.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT workspace_id
		FROM teams
		WHERE id = %s
	`, placeholder(driver, 1)), teamID).Scan(&workspaceID); err != nil {
		return "", notFoundOrErr(err)
	}
	return workspaceID, nil
}

func uniqueTeamSlug(ctx context.Context, q sqlQueryer, driver Driver, workspaceID string, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "team"
	}

	for index := 0; index < 100; index++ {
		slug := base
		if index > 0 {
			slug = fmt.Sprintf("%s-%d", base, index+1)
		}
		var existingID string
		if err := q.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT id
			FROM teams
			WHERE workspace_id = %s AND slug = %s
		`, placeholder(driver, 1), placeholder(driver, 2)), workspaceID, slug).Scan(&existingID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return slug, nil
			}
			return "", err
		}
	}
	return "", fmt.Errorf("%w: team slug is unavailable", ErrValidation)
}

func normalizeWorkspaceRole(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "owner":
		return "owner", nil
	case "admin":
		return "admin", nil
	case "", "member":
		return "member", nil
	case "viewer":
		return "viewer", nil
	default:
		return "", fmt.Errorf("%w: role must be owner, admin, member, or viewer", ErrValidation)
	}
}

func truthLiteral(driver Driver) string {
	if driver == DriverPostgres {
		return "true"
	}
	return "1"
}
