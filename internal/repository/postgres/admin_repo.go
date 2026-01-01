package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAdminNotFound      = errors.New("admin user not found")
	ErrAdminInactive      = errors.New("admin user is inactive")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// AdminUser represents an admin user entity
type AdminUser struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Email        string
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AdminRepository handles admin user database operations
type AdminRepository struct {
	db *pgxpool.Pool
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

// FindByEmail finds an admin user by email
func (r *AdminRepository) FindByEmail(ctx context.Context, email string) (*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, email, is_active, last_login_at, created_at, updated_at
		FROM admin_users
		WHERE email = $1
	`

	var admin AdminUser
	err := r.db.QueryRow(ctx, query, email).Scan(
		&admin.ID,
		&admin.Username,
		&admin.PasswordHash,
		&admin.Email,
		&admin.IsActive,
		&admin.LastLoginAt,
		&admin.CreatedAt,
		&admin.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}

	return &admin, nil
}

// FindByUsername finds an admin user by username
func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, email, is_active, last_login_at, created_at, updated_at
		FROM admin_users
		WHERE username = $1
	`

	var admin AdminUser
	err := r.db.QueryRow(ctx, query, username).Scan(
		&admin.ID,
		&admin.Username,
		&admin.PasswordHash,
		&admin.Email,
		&admin.IsActive,
		&admin.LastLoginAt,
		&admin.CreatedAt,
		&admin.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}

	return &admin, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *AdminRepository) UpdateLastLogin(ctx context.Context, adminID uuid.UUID) error {
	query := `
		UPDATE admin_users
		SET last_login_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, adminID)
	return err
}

// VerifyPassword verifies if the provided password matches using pgcrypto
func (r *AdminRepository) VerifyPassword(ctx context.Context, hashedPassword, plainPassword string) (bool, error) {
	query := `SELECT $1 = crypt($2, $1)`

	var matches bool
	err := r.db.QueryRow(ctx, query, hashedPassword, plainPassword).Scan(&matches)
	if err != nil {
		return false, err
	}

	return matches, nil
}
