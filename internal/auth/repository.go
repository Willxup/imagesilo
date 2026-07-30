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
	if admin.DisplayName == "" {
		admin.DisplayName = "ImageSilo"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin(id, display_name, email, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		admin.ID, admin.DisplayName, admin.Email, admin.PasswordHash, admin.CreatedAt.Unix(), admin.UpdatedAt.Unix(),
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
	return r.findAdmin(ctx, `
		SELECT id, display_name, email, password_hash, created_at, updated_at
		FROM admin WHERE email = ?`, email)
}

func (r *Repository) FindAdminByID(ctx context.Context, id string) (Admin, error) {
	return r.findAdmin(ctx, `
		SELECT id, display_name, email, password_hash, created_at, updated_at
		FROM admin WHERE id = ?`, id)
}

func (r *Repository) findAdmin(ctx context.Context, query, value string) (Admin, error) {
	var admin Admin
	var createdAt, updatedAt int64
	err := r.db.QueryRowContext(ctx, query, value).Scan(&admin.ID, &admin.DisplayName, &admin.Email, &admin.PasswordHash, &createdAt, &updatedAt)
	if err != nil {
		return Admin{}, err
	}
	admin.CreatedAt = time.Unix(createdAt, 0).UTC()
	admin.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return admin, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, adminID, displayName, email string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin SET display_name = ?, email = ?, updated_at = ? WHERE id = ?`,
		displayName, email, now.Unix(), adminID,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrAdminExists
		}
		return fmt.Errorf("update administrator profile: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return fmt.Errorf("administrator profile update affected %d rows", updated)
	}
	return nil
}

func (r *Repository) CreateSession(ctx context.Context, session Session) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions(id, admin_id, token_hash, csrf_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		session.ID, session.AdminID, session.TokenHash[:], session.CSRFHash[:], session.ExpiresAt.Unix(), session.CreatedAt.Unix(),
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

func (r *Repository) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", now.Unix())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired sessions: %w", err)
	}
	return count, nil
}

func (r *Repository) ChangePasswordAndRevokeOtherSessions(
	ctx context.Context,
	adminID string,
	passwordHash string,
	keepSessionHash [32]byte,
	now time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE admin SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, now.Unix(), adminID,
	)
	if err != nil {
		return fmt.Errorf("update administrator password: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil || updated != 1 {
		return fmt.Errorf("administrator password update affected %d rows", updated)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions WHERE admin_id = ? AND token_hash <> ?`,
		adminID, keepSessionHash[:],
	); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}

func (r *Repository) ListActiveSessions(ctx context.Context, now time.Time) (map[[32]byte]SessionIdentity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sessions.id, sessions.admin_id, admin.display_name, admin.email, sessions.token_hash, sessions.csrf_hash, sessions.expires_at
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
		var rawHash, rawCSRFHash []byte
		var expiresAt int64
		if err := rows.Scan(&identity.SessionID, &identity.AdminID, &identity.DisplayName, &identity.Email, &rawHash, &rawCSRFHash, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan active session: %w", err)
		}
		if len(rawHash) != 32 || len(rawCSRFHash) != 32 {
			return nil, fmt.Errorf("session %s has invalid authentication hash length", identity.SessionID)
		}
		var hash [32]byte
		copy(hash[:], rawHash)
		copy(identity.CSRFHash[:], rawCSRFHash)
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
