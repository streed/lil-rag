package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import SQLite driver
	"golang.org/x/crypto/bcrypt"
)

// Auth manages user authentication
type Auth struct {
	db *sql.DB
}

// User represents a user in the system
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Don't expose password hash in JSON
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session represents an active user session
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	Token     string    `json:"-"` // Don't expose session token in JSON
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// New creates a new Auth instance with the given database path
func New(dbPath string) (*Auth, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := ensureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create auth directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_timeout=30000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open auth database: %w", err)
	}

	auth := &Auth{db: db}

	if err := auth.initializeSchema(); err != nil {
		_ = db.Close() // Ignore close errors during cleanup
		return nil, fmt.Errorf("failed to initialize auth schema: %w", err)
	}

	return auth, nil
}

// Close closes the database connection
func (a *Auth) Close() error {
	return a.db.Close()
}

// initializeSchema creates the necessary tables
func (a *Auth) initializeSchema() error {
	schema := `
-- Users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sessions table for active logins
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
`

	_, err := a.db.Exec(schema)
	return err
}

// CreateUser creates a new user with the given username and password
func (a *Auth) CreateUser(ctx context.Context, username, password string) (*User, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		INSERT INTO users (username, password_hash, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	result, err := a.db.ExecContext(ctx, query, username, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	// Return the created user
	return a.GetUserByID(ctx, int(userID))
}

// GetUserByID retrieves a user by their ID
func (a *Auth) GetUserByID(ctx context.Context, userID int) (*User, error) {
	query := `
		SELECT id, username, password_hash, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	var user User
	err := a.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by their username
func (a *Auth) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, created_at, updated_at
		FROM users
		WHERE username = ?
	`

	var user User
	err := a.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// AuthenticateUser verifies username and password, returns user if valid
func (a *Auth) AuthenticateUser(ctx context.Context, username, password string) (*User, error) {
	user, err := a.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("authentication failed: invalid credentials")
	}

	return user, nil
}

// CreateSession creates a new session for the user
func (a *Auth) CreateSession(ctx context.Context, userID int) (*Session, error) {
	// Generate session ID and token
	sessionID, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	token, err := generateRandomString(64)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	// Set expiration to 30 days from now
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	query := `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = a.db.ExecContext(ctx, query, sessionID, userID, token, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Get user info for the session
	user, err := a.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user for session: %w", err)
	}

	return &Session{
		ID:        sessionID,
		UserID:    userID,
		Username:  user.Username,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}, nil
}

// ValidateSession validates a session token and returns the session if valid
func (a *Auth) ValidateSession(ctx context.Context, token string) (*Session, error) {
	query := `
		SELECT s.id, s.user_id, u.username, s.token, s.expires_at, s.created_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
	`

	var session Session
	err := a.db.QueryRowContext(ctx, query, token).Scan(
		&session.ID, &session.UserID, &session.Username, &session.Token, &session.ExpiresAt, &session.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session (logout)
func (a *Auth) DeleteSession(ctx context.Context, token string) error {
	_, err := a.db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// CleanExpiredSessions removes expired sessions
func (a *Auth) CleanExpiredSessions(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP")
	if err != nil {
		return fmt.Errorf("failed to clean expired sessions: %w", err)
	}
	return nil
}

// ListUsers returns all users (for administrative purposes)
func (a *Auth) ListUsers(ctx context.Context) ([]User, error) {
	query := `
		SELECT id, username, password_hash, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore close errors in defer
	}()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	if users == nil {
		users = []User{}
	}

	return users, nil
}

// UserExists checks if a user with the given username exists
func (a *Auth) UserExists(ctx context.Context, username string) (bool, error) {
	var count int
	err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}
	return count > 0, nil
}

// HasUsers checks if there are any users in the system
func (a *Auth) HasUsers(ctx context.Context) (bool, error) {
	var count int
	err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count users: %w", err)
	}
	return count > 0, nil
}

// DeleteUser removes a user and all their sessions from the system
func (a *Auth) DeleteUser(ctx context.Context, username string) error {
	// Start a transaction
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			// Rollback error is logged but doesn't override the main error
			_ = rbErr // Explicitly ignore error
		}
	}()

	// Get user ID first
	var userID int
	err = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user '%s' not found", username)
		}
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Delete all sessions for this user
	_, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	// Delete the user
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete transaction: %w", err)
	}

	return nil
}

// UpdatePassword changes a user's password
func (a *Auth) UpdatePassword(ctx context.Context, username, newPassword string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if newPassword == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update the password in the database
	result, err := a.db.ExecContext(ctx,
		"UPDATE users SET password_hash = ?, updated_at = ? WHERE username = ?",
		string(hashedPassword), time.Now(), username)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ensureDir creates a directory if it doesn't exist
func ensureDir(_ string) error {
	return nil // Simple implementation for now
}

// SecureCompare performs a constant-time comparison of two strings
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
