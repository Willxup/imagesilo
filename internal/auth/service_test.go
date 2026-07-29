package auth

import (
	"context"
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
	identity, token, err := service.Login(context.Background(), "ADMIN@example.com", "a secure test password", now)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
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
