package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var ErrDatabaseUnavailable = errors.New("database unavailable")
var ErrUserExists = errors.New("user already exists")

type Driver string

const (
	DriverUnknown  Driver = ""
	DriverPostgres Driver = "postgres"
	DriverSQLite   Driver = "sqlite"
)

type Connection struct {
	Driver Driver
	Pool   *pgxpool.Pool
	SQL    *sql.DB
}

type ReadinessChecker struct {
	Conn *Connection
}

type CreateAdminUserParams struct {
	Email       string
	Password    string
	DisplayName string
}

func Open(ctx context.Context, databaseURL string) (*Connection, error) {
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	driver := DriverForURL(databaseURL)
	switch driver {
	case DriverPostgres:
		pool, err := pgxpool.New(openCtx, databaseURL)
		if err != nil {
			return nil, err
		}
		if err := pool.Ping(openCtx); err != nil {
			pool.Close()
			return nil, err
		}
		sqlDB, err := sql.Open(sqlDriverName(driver), databaseURL)
		if err != nil {
			pool.Close()
			return nil, err
		}
		if err := sqlDB.PingContext(openCtx); err != nil {
			pool.Close()
			sqlDB.Close()
			return nil, err
		}
		return &Connection{Driver: driver, Pool: pool, SQL: sqlDB}, nil
	case DriverSQLite:
		if err := ensureSQLiteDir(databaseURL); err != nil {
			return nil, err
		}
		sqlDB, err := sql.Open(sqlDriverName(driver), sqliteDSN(databaseURL))
		if err != nil {
			return nil, err
		}
		if err := sqlDB.PingContext(openCtx); err != nil {
			sqlDB.Close()
			return nil, err
		}
		return &Connection{Driver: driver, SQL: sqlDB}, nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func (c *Connection) Close() {
	if c == nil {
		return
	}
	if c.Pool != nil {
		c.Pool.Close()
	}
	if c.SQL != nil {
		_ = c.SQL.Close()
	}
}

func (c ReadinessChecker) Ready(ctx context.Context) error {
	if c.Conn == nil {
		return ErrDatabaseUnavailable
	}
	switch c.Conn.Driver {
	case DriverPostgres:
		if c.Conn.Pool == nil {
			return ErrDatabaseUnavailable
		}
		return c.Conn.Pool.Ping(ctx)
	case DriverSQLite:
		if c.Conn.SQL == nil {
			return ErrDatabaseUnavailable
		}
		return c.Conn.SQL.PingContext(ctx)
	default:
		return ErrDatabaseUnavailable
	}
}

func MigrateUp(ctx context.Context, databaseURL string, migrationFS fs.FS) error {
	driver := DriverForURL(databaseURL)
	if driver == DriverUnknown {
		return fmt.Errorf("unsupported database URL %q", databaseURL)
	}
	if err := ensureSQLiteDir(databaseURL); err != nil {
		return err
	}

	sqlDB, err := sql.Open(sqlDriverName(driver), sqlDSN(databaseURL))
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return err
	}
	if driver == DriverSQLite {
		if err := repairSQLiteWorkspaceMembersID(ctx, sqlDB); err != nil {
			return err
		}
		if err := repairSQLiteColumnSystemKey(ctx, sqlDB); err != nil {
			return err
		}
		if err := repairSQLiteSprintPlanningSchema(ctx, sqlDB); err != nil {
			return err
		}
	}

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return err
	}

	return goose.Up(sqlDB, ".")
}

func repairSQLiteWorkspaceMembersID(ctx context.Context, sqlDB *sql.DB) error {
	hasMembers, err := sqliteTableExists(ctx, sqlDB, "workspace_members")
	if err != nil || !hasMembers {
		return err
	}
	hasID, err := sqliteColumnExists(ctx, sqlDB, "workspace_members", "id")
	if err != nil || hasID {
		return err
	}

	createdAtExpr := "created_at"
	hasCreatedAt, err := sqliteColumnExists(ctx, sqlDB, "workspace_members", "created_at")
	if err != nil {
		return err
	}
	if !hasCreatedAt {
		createdAtExpr = "datetime('now')"
	}

	if _, err := sqlDB.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer sqlDB.ExecContext(ctx, "PRAGMA foreign_keys = ON")

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		"DROP TABLE IF EXISTS workspace_members_repair",
		`CREATE TABLE workspace_members_repair (
			id uuid PRIMARY KEY NOT NULL,
			workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role text NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
			created_at text NOT NULL DEFAULT (datetime('now')),
			UNIQUE (workspace_id, user_id)
		)`,
		fmt.Sprintf(`INSERT INTO workspace_members_repair (id, workspace_id, user_id, role, created_at)
			SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))),
				workspace_id,
				user_id,
				role,
				%s
			FROM workspace_members`, createdAtExpr),
		"DROP TABLE workspace_members",
		"ALTER TABLE workspace_members_repair RENAME TO workspace_members",
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func repairSQLiteColumnSystemKey(ctx context.Context, sqlDB *sql.DB) error {
	hasColumns, err := sqliteTableExists(ctx, sqlDB, "columns")
	if err != nil || !hasColumns {
		return err
	}
	hasSystemKey, err := sqliteColumnExists(ctx, sqlDB, "columns", "system_key")
	if err != nil || hasSystemKey {
		return err
	}
	if _, err := sqlDB.ExecContext(ctx, "ALTER TABLE columns ADD COLUMN system_key text"); err != nil {
		return err
	}
	_, err = sqlDB.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS columns_board_system_key_unique ON columns(board_id, system_key) WHERE system_key IS NOT NULL")
	return err
}

func repairSQLiteSprintPlanningSchema(ctx context.Context, sqlDB *sql.DB) error {
	hasGoose, err := sqliteTableExists(ctx, sqlDB, "goose_db_version")
	if err != nil || !hasGoose {
		return err
	}
	applied, err := sqliteMigrationApplied(ctx, sqlDB, 2)
	if err != nil || !applied {
		return err
	}

	hasSprints, err := sqliteTableExists(ctx, sqlDB, "sprints")
	if err != nil {
		return err
	}
	if !hasSprints {
		if _, err := sqlDB.ExecContext(ctx, `
			CREATE TABLE sprints (
				id uuid PRIMARY KEY NOT NULL,
				workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
				name text NOT NULL,
				goal text NOT NULL DEFAULT '',
				status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed')),
				starts_on text,
				ends_on text,
				started_at text,
				completed_at text,
				created_at text NOT NULL DEFAULT (datetime('now')),
				updated_at text NOT NULL DEFAULT (datetime('now')),
				UNIQUE (workspace_id, name)
			);
		`); err != nil {
			return err
		}
		if _, err := sqlDB.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS sprints_workspace_status_idx ON sprints(workspace_id, status)"); err != nil {
			return err
		}
		if _, err := sqlDB.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS sprints_one_active_per_workspace_idx ON sprints(workspace_id) WHERE status = 'active'"); err != nil {
			return err
		}
	}

	hasSprintID, err := sqliteColumnExists(ctx, sqlDB, "cards", "sprint_id")
	if err != nil {
		return err
	}
	if !hasSprintID {
		if _, err := sqlDB.ExecContext(ctx, "ALTER TABLE cards ADD COLUMN sprint_id uuid REFERENCES sprints(id) ON DELETE SET NULL"); err != nil {
			return err
		}
	}
	_, err = sqlDB.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS cards_sprint_id_idx ON cards(sprint_id)")
	return err
}

func sqliteMigrationApplied(ctx context.Context, sqlDB *sql.DB, version int64) (bool, error) {
	var applied bool
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT is_applied
		FROM goose_db_version
		WHERE version_id = ?
		ORDER BY id DESC
		LIMIT 1
	`, version).Scan(&applied); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return applied, nil
}

func sqliteTableExists(ctx context.Context, sqlDB *sql.DB, table string) (bool, error) {
	var name string
	if err := sqlDB.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func sqliteColumnExists(ctx context.Context, sqlDB *sql.DB, table string, column string) (bool, error) {
	rows, err := sqlDB.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func CreateAdminUser(ctx context.Context, conn *Connection, params CreateAdminUserParams) (string, error) {
	if conn == nil {
		return "", ErrDatabaseUnavailable
	}
	if err := validateAdminUser(params); err != nil {
		return "", err
	}

	email := normalizeEmail(params.Email)
	displayName := strings.TrimSpace(params.DisplayName)

	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	switch conn.Driver {
	case DriverPostgres:
		return createPostgresAdminUser(ctx, conn, email, displayName, string(hash))
	case DriverSQLite:
		return createSQLiteAdminUser(ctx, conn, email, displayName, string(hash))
	default:
		return "", ErrDatabaseUnavailable
	}
}

func createPostgresAdminUser(ctx context.Context, conn *Connection, email string, displayName string, hash string) (string, error) {
	if conn.Pool == nil {
		return "", ErrDatabaseUnavailable
	}

	var userID string
	err := conn.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, is_admin)
		VALUES ($1, $2, $3, true)
		RETURNING id::text
	`, email, hash, displayName).Scan(&userID)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrUserExists
		}
		return "", err
	}

	return userID, nil
}

func createSQLiteAdminUser(ctx context.Context, conn *Connection, email string, displayName string, hash string) (string, error) {
	if conn.SQL == nil {
		return "", ErrDatabaseUnavailable
	}

	userID, err := newID()
	if err != nil {
		return "", err
	}

	_, err = conn.SQL.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, is_admin)
		VALUES (?, ?, ?, ?, 1)
	`, userID, email, hash, displayName)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrUserExists
		}
		return "", err
	}

	return userID, nil
}

func validateAdminUser(params CreateAdminUserParams) error {
	if normalizeEmail(params.Email) == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(params.Email, "@") {
		return fmt.Errorf("email %q is not valid", params.Email)
	}
	if strings.TrimSpace(params.DisplayName) == "" {
		return errors.New("display name is required")
	}
	if len(params.Password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func DriverForURL(databaseURL string) Driver {
	lower := strings.ToLower(strings.TrimSpace(databaseURL))
	switch {
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return DriverPostgres
	case strings.HasPrefix(lower, "sqlite://"), strings.HasPrefix(lower, "sqlite:"):
		return DriverSQLite
	case strings.Contains(lower, "://"):
		return DriverUnknown
	default:
		return DriverSQLite
	}
}

func sqlDriverName(driver Driver) string {
	switch driver {
	case DriverPostgres:
		return "pgx"
	case DriverSQLite:
		return "sqlite"
	default:
		return ""
	}
}

func gooseDialect(driver Driver) string {
	switch driver {
	case DriverPostgres:
		return "postgres"
	case DriverSQLite:
		return "sqlite3"
	default:
		return ""
	}
}

func sqlDSN(databaseURL string) string {
	if DriverForURL(databaseURL) == DriverSQLite {
		return sqliteDSN(databaseURL)
	}
	return databaseURL
}

func sqliteDSN(databaseURL string) string {
	path := sqlitePath(databaseURL)
	if path == ":memory:" {
		return path
	}

	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

func sqlitePath(databaseURL string) string {
	value := strings.TrimSpace(databaseURL)
	switch {
	case strings.HasPrefix(strings.ToLower(value), "sqlite://"):
		value = value[len("sqlite://"):]
	case strings.HasPrefix(strings.ToLower(value), "sqlite:"):
		value = value[len("sqlite:"):]
	}

	if strings.HasPrefix(value, "/") && len(value) > 2 && value[2] == ':' {
		value = value[1:]
	}
	if value == "" {
		return "data/arqboard.db"
	}
	if strings.HasPrefix(value, "file:") || value == ":memory:" {
		return value
	}
	return filepath.FromSlash(value)
}

func ensureSQLiteDir(databaseURL string) error {
	if DriverForURL(databaseURL) != DriverSQLite {
		return nil
	}

	path := sqlitePath(databaseURL)
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "constraint failed") || strings.Contains(message, "unique constraint")
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	encoded := hex.EncodeToString(bytes[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
