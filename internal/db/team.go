package db

import (
	"context"
	"database/sql"
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

type CreateWorkspaceMemberParams struct {
	Email       string
	DisplayName string
	Password    string
	Role        string
}

type UpdateWorkspaceMemberParams struct {
	MemberID string
	Role     string
}

type TeamStore struct {
	Conn *Connection
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
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		displayName = email
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
	} else if strings.TrimSpace(params.DisplayName) != "" {
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
