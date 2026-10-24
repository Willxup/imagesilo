package apitoken

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
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

func TestRevokeDuringCreateCannotRepublishToken(t *testing.T) {
	service, db := newTokenMutationTestService(t)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0).UTC()
	committed := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	releaseCreate := func() { resumeOnce.Do(func() { close(resume) }) }
	t.Cleanup(releaseCreate)
	service.afterCreateCommit = func() {
		close(committed)
		<-resume
	}
	type createResult struct {
		plaintext string
		err       error
	}
	created := make(chan createResult, 1)
	go func() {
		_, plaintext, err := service.Create(ctx, "concurrent revoke", []Scope{ScopeImagesReadPrivate}, nil, now)
		created <- createResult{plaintext: plaintext, err: err}
	}()
	select {
	case <-committed:
	case <-time.After(time.Second):
		t.Fatal("Create() did not commit the token")
	}

	listed, err := service.List(ctx)
	if err != nil || len(listed) != 1 {
		t.Fatalf("List() before publication = %+v, %v", listed, err)
	}
	revokeAttempted := make(chan struct{})
	service.beforeRevokeLock = func() { close(revokeAttempted) }
	revoked := make(chan error, 1)
	go func() { revoked <- service.Revoke(ctx, listed[0].ID) }()
	<-revokeAttempted
	revokeFinished := false
	select {
	case err := <-revoked:
		revokeFinished = true
		t.Errorf("Revoke() completed before the committed token was published: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	releaseCreate()
	result := <-created
	if result.err != nil {
		t.Fatalf("Create() error = %v", result.err)
	}
	if !revokeFinished {
		if err := <-revoked; err != nil {
			t.Fatalf("Revoke() error = %v", err)
		}
	}
	var status string
	if err := db.QueryRow("SELECT status FROM api_tokens WHERE id = ?", listed[0].ID).Scan(&status); err != nil || status != "revoked" {
		t.Fatalf("database token status = %q, %v", status, err)
	}
	if _, err := service.Authenticate(result.plaintext, now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Authenticate() accepted a durably revoked token: %v", err)
	}
	if err := service.Load(ctx, now); err != nil {
		t.Fatalf("Load() after concurrent revoke error = %v", err)
	}
	if service.TokenCount() != 0 {
		t.Fatalf("revoked token was restored by Load(): count = %d", service.TokenCount())
	}
}

func TestWaitingTokenMutationAllowsRebuildAndAuthentication(t *testing.T) {
	service, _ := newTokenMutationTestService(t)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0).UTC()
	token, plaintext, err := service.Create(ctx, "rebuild ordering", []Scope{ScopeImagesUpload}, nil, now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	service.mutationsMu.Lock()
	var unlockOnce sync.Once
	unlock := func() { unlockOnce.Do(service.mutationsMu.Unlock) }
	t.Cleanup(unlock)
	attempted := make(chan struct{})
	service.beforeRevokeLock = func() { close(attempted) }
	revoked := make(chan error, 1)
	go func() { revoked <- service.Revoke(ctx, token.ID) }()
	<-attempted

	rebuilt := make(chan error, 1)
	go func() { rebuilt <- service.Load(ctx, now) }()
	select {
	case err := <-rebuilt:
		if err != nil {
			t.Fatalf("Load() while token mutation waits error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("a waiting token mutation blocked the rebuild barrier")
	}
	authenticated := make(chan error, 1)
	go func() { _, err := service.Authenticate(plaintext, now); authenticated <- err }()
	select {
	case err := <-authenticated:
		if err != nil {
			t.Fatalf("Authenticate() while token mutation waits error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("authentication waited for the token mutation lock")
	}
	unlock()
	if err := <-revoked; err != nil {
		t.Fatalf("Revoke() after rebuild error = %v", err)
	}
	if _, err := service.Authenticate(plaintext, now); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Authenticate() after revoke error = %v", err)
	}
}

func TestTokenMutationErrorsReleaseWriteLock(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Unix(1_800_000_000, 0).UTC()
	for _, test := range []struct {
		name string
		run  func(*Service) error
		want error
	}{
		{name: "create canceled", run: func(service *Service) error {
			_, _, err := service.Create(canceled, "canceled", []Scope{ScopeImagesUpload}, nil, now)
			return err
		}, want: context.Canceled},
		{name: "revoke canceled", run: func(service *Service) error { return service.Revoke(canceled, "missing") }, want: context.Canceled},
		{name: "revoke missing", run: func(service *Service) error { return service.Revoke(context.Background(), "missing") }, want: ErrTokenNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, _ := newTokenMutationTestService(t)
			if err := test.run(service); !errors.Is(err, test.want) {
				t.Fatalf("mutation error = %v, want %v", err, test.want)
			}
			finished := make(chan error, 1)
			go func() {
				token, _, err := service.Create(context.Background(), "after error", []Scope{ScopeImagesUpload}, nil, now)
				if err == nil {
					err = service.Revoke(context.Background(), token.ID)
				}
				if err == nil {
					err = service.Load(context.Background(), now)
				}
				finished <- err
			}()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatalf("token mutations after error failed: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("failed mutation retained a write lock or rebuild barrier")
			}
		})
	}
}

func newTokenMutationTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	return NewService(NewRepository(db), NewIndex()), db
}
