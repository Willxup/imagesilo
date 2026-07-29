package apitoken

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

func TestTokenLifecycleUsesHashOnlyIndex(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}

	now := time.Unix(1_800_000_000, 0).UTC()
	expiresAt := now.Add(time.Hour)
	service := NewService(NewRepository(db), NewIndex())
	token, plaintext, err := service.Create(context.Background(), "automation", []Scope{
		ScopeImagesUpload, ScopeImagesReadPrivate, ScopeImagesDelete, ScopeAliasesWrite, ScopeImagesUpload,
	}, &expiresAt, now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(plaintext) != 47 || plaintext[:4] != "ist_" {
		t.Fatalf("plaintext token format = %q", plaintext)
	}
	if token.TokenPrefix != plaintext[:12] {
		t.Fatalf("TokenPrefix = %q, want %q", token.TokenPrefix, plaintext[:12])
	}
	if len(token.Scopes) != 4 ||
		token.Scopes[0] != ScopeAliasesWrite ||
		token.Scopes[1] != ScopeImagesDelete ||
		token.Scopes[2] != ScopeImagesReadPrivate ||
		token.Scopes[3] != ScopeImagesUpload {
		t.Fatalf("normalized scopes = %v", token.Scopes)
	}
	identity, err := service.Authenticate(plaintext, now.Add(time.Minute))
	if err != nil || identity.TokenID != token.ID {
		t.Fatalf("Authenticate() = (%+v, %v)", identity, err)
	}
	for _, scope := range []Scope{ScopeImagesUpload, ScopeImagesReadPrivate, ScopeImagesDelete, ScopeAliasesWrite} {
		if !identity.HasScope(scope) {
			t.Fatalf("Authenticate() identity is missing scope %s", scope)
		}
	}

	var storedHash []byte
	var storedPrefix string
	if err := db.QueryRow("SELECT token_hash, token_prefix FROM api_tokens WHERE id = ?", token.ID).Scan(&storedHash, &storedPrefix); err != nil {
		t.Fatalf("query stored token = %v", err)
	}
	wantHash := sha256.Sum256([]byte(plaintext))
	if !bytes.Equal(storedHash, wantHash[:]) {
		t.Fatal("database token_hash does not match SHA-256 of plaintext")
	}
	if bytes.Equal(storedHash, []byte(plaintext)) || storedPrefix == plaintext {
		t.Fatal("database contains a recoverable plaintext API token")
	}

	reloaded := NewService(NewRepository(db), NewIndex())
	if err := reloaded.Load(context.Background(), now); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, err := reloaded.Authenticate(plaintext, now.Add(time.Minute)); err != nil {
		t.Fatalf("Authenticate() after reload error = %v", err)
	}
	if err := reloaded.Revoke(context.Background(), token.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if _, err := reloaded.Authenticate(plaintext, now.Add(time.Minute)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Authenticate() after revoke error = %v, want ErrInvalidToken", err)
	}
	listed, err := reloaded.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Status != "revoked" || listed[0].TokenPrefix != storedPrefix {
		t.Fatalf("List() = %+v", listed)
	}
}

func TestTokenExpiryAndValidation(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}

	now := time.Unix(1_800_000_000, 0).UTC()
	service := NewService(NewRepository(db), NewIndex())
	if _, _, err := service.Create(context.Background(), "invalid", nil, nil, now); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("Create() empty scopes error = %v, want ErrInvalidScope", err)
	}
	past := now.Add(-time.Second)
	if _, _, err := service.Create(context.Background(), "expired", []Scope{ScopeImagesUpload}, &past, now); err == nil {
		t.Fatal("Create() accepted an expiration in the past")
	}
	expiresAt := now.Add(time.Minute)
	_, plaintext, err := service.Create(context.Background(), "short lived", []Scope{ScopeAliasesWrite}, &expiresAt, now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := service.Authenticate(plaintext, expiresAt); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Authenticate() at expiration error = %v, want ErrInvalidToken", err)
	}
	if removed := service.CleanupExpired(expiresAt); removed != 1 || service.TokenCount() != 0 {
		t.Fatalf("CleanupExpired() = %d, count = %d", removed, service.TokenCount())
	}
	reloaded := NewService(NewRepository(db), NewIndex())
	if err := reloaded.Load(context.Background(), expiresAt); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if reloaded.TokenCount() != 0 {
		t.Fatalf("reloaded expired token count = %d", reloaded.TokenCount())
	}
}
