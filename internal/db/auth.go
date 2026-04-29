package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrUnauthenticated = errors.New("unauthenticated")

const defaultSessionTTL = 7 * 24 * time.Hour
const sqliteTimeFormat = "2006-01-02 15:04:05"

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	IsAdmin     bool   `json:"isAdmin"`
}

type LoginParams struct {
	Email    string
	Password string
}

type LoginSession struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

type AuthStore struct {
	Conn       *Connection
	SessionTTL time.Duration
}

func (store AuthStore) Login(ctx context.Context, params LoginParams) (LoginSession, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return LoginSession{}, err
	}

	email := normalizeEmail(params.Email)
	password := params.Password
	if email == "" || strings.TrimSpace(password) == "" {
		return LoginSession{}, ErrInvalidCredentials
	}

	user, hash, err := loadUserForLogin(ctx, sqlDB, driver, email)
	if err != nil {
		return LoginSession{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return LoginSession{}, ErrInvalidCredentials
	}

	token, err := newSessionToken()
	if err != nil {
		return LoginSession{}, err
	}
	sessionID, err := newID()
	if err != nil {
		return LoginSession{}, err
	}
	expiresAt := time.Now().UTC().Add(store.sessionTTL())
	_, err = sqlDB.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO sessions (id, user_id, token_hash, expires_at)
		VALUES (%s, %s, %s, %s)
	`,
		uuidPlaceholder(driver, 1),
		uuidPlaceholder(driver, 2),
		placeholder(driver, 3),
		placeholder(driver, 4),
	), sessionID, user.ID, sessionTokenHash(token), sessionExpiryValue(driver, expiresAt))
	if err != nil {
		return LoginSession{}, err
	}

	return LoginSession{
		User:      user,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (store AuthStore) CurrentUser(ctx context.Context, token string) (User, error) {
	sqlDB, driver, err := store.database()
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(token) == "" {
		return User{}, ErrUnauthenticated
	}

	var user User
	err = sqlDB.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s, users.email, users.display_name, users.is_admin
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = %s
			AND sessions.revoked_at IS NULL
			AND sessions.expires_at > %s
	`,
		idText(driver, "users.id"),
		placeholder(driver, 1),
		currentTimestamp(driver),
	), sessionTokenHash(token)).Scan(&user.ID, &user.Email, &user.DisplayName, &user.IsAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUnauthenticated
		}
		return User{}, err
	}

	return user, nil
}

func (store AuthStore) Logout(ctx context.Context, token string) error {
	sqlDB, driver, err := store.database()
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return nil
	}

	_, err = sqlDB.ExecContext(ctx, fmt.Sprintf(`
		UPDATE sessions
		SET revoked_at = %s
		WHERE token_hash = %s
	`, currentTimestamp(driver), placeholder(driver, 1)), sessionTokenHash(token))
	return err
}

func (store AuthStore) database() (*sql.DB, Driver, error) {
	if store.Conn == nil || store.Conn.SQL == nil {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	if store.Conn.Driver != DriverPostgres && store.Conn.Driver != DriverSQLite {
		return nil, DriverUnknown, ErrDatabaseUnavailable
	}
	return store.Conn.SQL, store.Conn.Driver, nil
}

func (store AuthStore) sessionTTL() time.Duration {
	if store.SessionTTL == 0 {
		return defaultSessionTTL
	}
	return store.SessionTTL
}

func loadUserForLogin(ctx context.Context, sqlDB *sql.DB, driver Driver, email string) (User, string, error) {
	var user User
	var hash string
	err := sqlDB.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s, email, password_hash, display_name, is_admin
		FROM users
		WHERE lower(email) = %s
	`, idText(driver, "id"), placeholder(driver, 1)), email).Scan(&user.ID, &user.Email, &hash, &user.DisplayName, &user.IsAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", ErrInvalidCredentials
		}
		return User{}, "", err
	}
	return user, hash, nil
}

func newSessionToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func sessionTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func sessionExpiryValue(driver Driver, expiresAt time.Time) any {
	if driver == DriverPostgres {
		return expiresAt
	}
	return expiresAt.Format(sqliteTimeFormat)
}

func idText(driver Driver, column string) string {
	if driver == DriverPostgres {
		return column + "::text"
	}
	return column
}

func uuidPlaceholder(driver Driver, index int) string {
	if driver == DriverPostgres {
		return fmt.Sprintf("$%d::uuid", index)
	}
	return "?"
}
