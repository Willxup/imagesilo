package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/platform/database"
)

func TestSessionAuthenticationUsesIndexAndReloads(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}

	passwordHash, err := hashPassword("a secure test password", PasswordParameters{
		Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	repository := NewRepository(db)
	if err := repository.CreateAdmin(context.Background(), Admin{
		ID: "admin-id", Email: "admin@example.com", PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateAdmin() error = %v", err)
	}

	index := NewSessionIndex()
	service, err := NewService(repository, index)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	identity, token, csrfToken, err := service.Login(context.Background(), "ADMIN@example.com", "a secure test password", now)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if err := service.ValidateCSRF(identity, csrfToken); err != nil {
		t.Fatalf("ValidateCSRF() error = %v", err)
	}
	var storedTokenHash, storedCSRFHash []byte
	if err := db.QueryRow("SELECT token_hash, csrf_hash FROM sessions WHERE id = ?", identity.SessionID).Scan(&storedTokenHash, &storedCSRFHash); err != nil {
		t.Fatalf("query stored session hashes = %v", err)
	}
	wantTokenHash := sha256.Sum256([]byte(token))
	wantCSRFHash := sha256.Sum256([]byte(csrfToken))
	if !bytes.Equal(storedTokenHash, wantTokenHash[:]) || !bytes.Equal(storedCSRFHash, wantCSRFHash[:]) {
		t.Fatal("database session hashes do not match SHA-256 of issued secrets")
	}
	if bytes.Equal(storedTokenHash, []byte(token)) || bytes.Equal(storedCSRFHash, []byte(csrfToken)) {
		t.Fatal("database contains a recoverable plaintext session or CSRF token")
	}
	if authenticated, err := service.Authenticate(token, now.Add(time.Minute)); err != nil || authenticated.AdminID != identity.AdminID {
		t.Fatalf("Authenticate() = (%+v, %v), want admin %s", authenticated, err, identity.AdminID)
	}

	reloadedIndex := NewSessionIndex()
	reloadedService, err := NewService(repository, reloadedIndex)
	if err != nil {
		t.Fatalf("NewService(reload) error = %v", err)
	}
	if err := reloadedService.LoadSessions(context.Background(), now); err != nil {
		t.Fatalf("LoadSessions() error = %v", err)
	}
	if _, err := reloadedService.Authenticate(token, now.Add(time.Minute)); err != nil {
		t.Fatalf("reloaded Authenticate() error = %v", err)
	}
	if err := reloadedService.Logout(context.Background(), token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := reloadedService.Authenticate(token, now.Add(time.Minute)); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Authenticate() after logout error = %v, want ErrInvalidSession", err)
	}
}

func TestSessionCSRFPasswordChangeAndExpiryStayConsistent(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}

	const oldPassword = "a secure original password"
	const newPassword = "a secure replacement password"
	passwordHash, err := hashPassword(oldPassword, PasswordParameters{
		Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	repository := NewRepository(db)
	if err := repository.CreateAdmin(context.Background(), Admin{
		ID: "admin-id", Email: "admin@example.com", PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateAdmin() error = %v", err)
	}
	service, err := NewService(repository, NewSessionIndex())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	firstIdentity, firstToken, firstCSRF, err := service.Login(context.Background(), "admin@example.com", oldPassword, now)
	if err != nil {
		t.Fatalf("first Login() error = %v", err)
	}
	secondIdentity, secondToken, secondCSRF, err := service.Login(context.Background(), "admin@example.com", oldPassword, now.Add(time.Second))
	if err != nil {
		t.Fatalf("second Login() error = %v", err)
	}
	if firstToken == secondToken || firstCSRF == secondCSRF || service.SessionCount() != 2 {
		t.Fatalf("session rotation inputs are not unique or count = %d", service.SessionCount())
	}
	if err := service.ValidateCSRF(firstIdentity, firstCSRF); err != nil {
		t.Fatalf("ValidateCSRF(first) error = %v", err)
	}
	if err := service.ValidateCSRF(firstIdentity, secondCSRF); !errors.Is(err, ErrInvalidCSRF) {
		t.Fatalf("ValidateCSRF(cross-session) error = %v, want ErrInvalidCSRF", err)
	}

	if err := service.ChangePassword(
		context.Background(), secondIdentity, secondToken, oldPassword, newPassword, now.Add(time.Minute),
	); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if _, err := service.Authenticate(firstToken, now.Add(2*time.Minute)); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("first session after password change error = %v, want ErrInvalidSession", err)
	}
	if _, err := service.Authenticate(secondToken, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("current session after password change error = %v", err)
	}
	if service.SessionCount() != 1 {
		t.Fatalf("session count after password change = %d, want 1", service.SessionCount())
	}
	if _, _, _, err := service.Login(context.Background(), "admin@example.com", oldPassword, now.Add(2*time.Minute)); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() with old password error = %v, want ErrInvalidCredentials", err)
	}
	if _, _, _, err := service.Login(context.Background(), "admin@example.com", newPassword, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("Login() with new password error = %v", err)
	}

	cleanupAt := now.Add(sessionLifetime + 3*time.Minute)
	deleted, err := service.CleanupExpired(context.Background(), cleanupAt)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if deleted != 2 || service.SessionCount() != 0 {
		t.Fatalf("CleanupExpired() deleted = %d, count = %d, want 2 and 0", deleted, service.SessionCount())
	}
}
