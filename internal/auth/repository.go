package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateAdmin(ctx context.Context, admin Admin) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin(id, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		admin.ID, admin.Email, admin.PasswordHash, admin.CreatedAt.Unix(), admin.UpdatedAt.Unix(),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrAdminExists
		}
		return fmt.Errorf("create administrator: %w", err)
	}
	return nil
}

func (r *Repository) FindAdminByEmail(ctx context.Context, email string) (Admin, error) {
	var admin Admin
	var createdAt, updatedAt int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, created_at, updated_at
		FROM admin WHERE email = ?`, email,
	).Scan(&admin.ID, &admin.Email, &admin.PasswordHash, &createdAt, &updatedAt)
	if err != nil {
		return Admin{}, err
	}
	admin.CreatedAt = time.Unix(createdAt, 0).UTC()
	admin.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return admin, nil
}

func (r *Repository) CreateSession(ctx context.Context, session Session) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions(id, admin_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.AdminID, session.TokenHash[:], session.ExpiresAt.Unix(), session.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSessionByHash(ctx context.Context, hash [32]byte) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", hash[:]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (r *Repository) ListActiveSessions(ctx context.Context, now time.Time) (map[[32]byte]SessionIdentity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sessions.id, sessions.admin_id, admin.email, sessions.token_hash, sessions.expires_at
		FROM sessions
		JOIN admin ON admin.id = sessions.admin_id
		WHERE sessions.expires_at > ?`, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	result := make(map[[32]byte]SessionIdentity)
	for rows.Next() {
		var identity SessionIdentity
		var rawHash []byte
		var expiresAt int64
		if err := rows.Scan(&identity.SessionID, &identity.AdminID, &identity.Email, &rawHash, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan active session: %w", err)
		}
		if len(rawHash) != 32 {
			return nil, fmt.Errorf("session %s has invalid token hash length", identity.SessionID)
		}
		var hash [32]byte
		copy(hash[:], rawHash)
		identity.ExpiresAt = time.Unix(expiresAt, 0).UTC()
		result[hash] = identity
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active sessions: %w", err)
	}
	return result, nil
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
