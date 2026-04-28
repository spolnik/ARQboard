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

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect(gooseDialect(driver)); err != nil {
		return err
	}

	return goose.Up(sqlDB, ".")
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
	if displayName == "" {
		displayName = email
	}

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
