package indexstate

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	imagealias "github.com/Willxup/imagesilo/internal/alias"
	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
	"github.com/Willxup/imagesilo/internal/delivery"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

func TestRebuildBarrierCannotOverwriteConcurrentIndexChanges(t *testing.T) {
	ctx := context.Background()
	directory, db := prepareRebuilderTest(t)
	defer db.Close()
	now := time.Unix(1_700_000_000, 0).UTC()
	const imageID = "019c1234-5678-7abc-8def-0123456789ab"
	insertRebuilderImage(t, db, imageID, now)
	if err := os.WriteFile(filepath.Join(directory, "images", imageID), []byte("image"), 0o640); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	passwordHash, err := auth.HashPassword("phase-four-rebuild-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	authRepository := auth.NewRepository(db)
	if err := authRepository.CreateAdmin(ctx, auth.Admin{
		ID: "administrator", Email: "admin@example.com", PasswordHash: passwordHash, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateAdmin() error = %v", err)
	}

	barrier := indexbarrier.New()
	deliveryIndex := delivery.NewIndex()
	sessionIndex := auth.NewSessionIndex()
	tokenIndex := apitoken.NewIndex()
	authService, err := auth.NewServiceWithBarrier(authRepository, sessionIndex, barrier)
	if err != nil {
		t.Fatalf("NewServiceWithBarrier() error = %v", err)
	}
	tokenRepository := apitoken.NewRepository(db)
	tokenService := apitoken.NewServiceWithBarrier(tokenRepository, tokenIndex, barrier)
	filesystem := storage.NewFilesystem(directory)
	rebuilder := NewRebuilder(
		db, filesystem, authRepository, tokenRepository,
		deliveryIndex, sessionIndex, tokenIndex, barrier,
	)
	if _, err := rebuilder.Rebuild(ctx, now); err != nil {
		t.Fatalf("initial Rebuild() error = %v", err)
	}
	imageService := images.NewServiceWithProcessorAndBarrier(
		images.NewRepository(db), filesystem, deliveryIndex,
		processor.NewEngine(), processor.NewGate(1), barrier,
	)
	aliasService := imagealias.NewService(imagealias.NewRepository(db), deliveryIndex, barrier)

	_, sessionToken, _, err := authService.Login(ctx, "admin@example.com", "phase-four-rebuild-password", now)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	token, plaintextToken, err := tokenService.Create(ctx, "rebuild token", []apitoken.Scope{apitoken.ScopeAliasesWrite}, nil, now)
	if err != nil {
		t.Fatalf("token Create() error = %v", err)
	}
	existingAlias, err := aliasService.Create(ctx, "/legacy/remove.jpg", imageID, "test", now)
	if err != nil {
		t.Fatalf("alias Create() error = %v", err)
	}

	originalLoad := rebuilder.loadSnapshot
	snapshotLoaded := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	rebuilder.loadSnapshot = func(ctx context.Context, now time.Time) (snapshot, Result, error) {
		loaded, result, err := originalLoad(ctx, now)
		close(snapshotLoaded)
		<-releaseSnapshot
		return loaded, result, err
	}
	rebuildDone := make(chan error, 1)
	go func() {
		_, err := rebuilder.Rebuild(ctx, now)
		rebuildDone <- err
	}()
	<-snapshotLoaded

	type mutationResult struct {
		name string
		err  error
	}
	started := make(chan struct{}, 5)
	mutations := make(chan mutationResult, 5)
	run := func(name string, operation func() error) {
		go func() {
			started <- struct{}{}
			mutations <- mutationResult{name: name, err: operation()}
		}()
	}
	run("visibility", func() error {
		updated, err := imageService.ChangeVisibility(ctx, imageID, images.VisibilityPrivate)
		if err == nil && !updated {
			return errors.New("image was not updated")
		}
		return err
	})
	run("session logout", func() error { return authService.Logout(ctx, sessionToken) })
	run("token revoke", func() error { return tokenService.Revoke(ctx, token.ID) })
	run("alias create", func() error {
		_, err := aliasService.Create(ctx, "/legacy/add.jpg", imageID, "test", now.Add(time.Second))
		return err
	})
	run("alias delete", func() error { return aliasService.Delete(ctx, existingAlias.ID) })
	for range 5 {
		<-started
	}
	select {
	case result := <-mutations:
		t.Fatalf("mutation %q completed while rebuild held the exclusive barrier: %v", result.name, result.err)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseSnapshot)
	if err := <-rebuildDone; err != nil {
		t.Fatalf("concurrent Rebuild() error = %v", err)
	}
	for range 5 {
		result := <-mutations
		if result.err != nil {
			t.Errorf("mutation %q error = %v", result.name, result.err)
		}
	}

	if target, ok := deliveryIndex.Get(imageID); !ok || target.Visibility != string(images.VisibilityPrivate) {
		t.Fatalf("delivery visibility after rebuild = %+v, %t", target, ok)
	}
	if _, err := authService.Authenticate(sessionToken, now); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatalf("session authentication after logout error = %v", err)
	}
	if _, err := tokenService.Authenticate(plaintextToken, now); !errors.Is(err, apitoken.ErrInvalidToken) {
		t.Fatalf("token authentication after revoke error = %v", err)
	}
	if _, ok := deliveryIndex.ResolveAlias("/legacy/remove.jpg"); ok {
		t.Fatal("deleted alias was restored by rebuild")
	}
	if id, ok := deliveryIndex.ResolveAlias("/legacy/add.jpg"); !ok || id != imageID {
		t.Fatalf("created alias after rebuild = %q, %t", id, ok)
	}

	assertRebuilderDatabaseState(t, db, imageID, token.ID)
}

func prepareRebuilderTest(t *testing.T) (string, *sql.DB) {
	t.Helper()
	directory := t.TempDir()
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(directory, path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%s): %v", path, err)
		}
	}
	db, err := database.Open(filepath.Join(directory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	return directory, db
}

func insertRebuilderImage(t *testing.T, db *sql.DB, id string, now time.Time) {
	t.Helper()
	hash := make([]byte, 32)
	if _, err := db.Exec(`
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, 'test.jpg', ?, '.jpg', 'image/jpeg', 1, 1, 1, 1, ?, ?, '{}', 'public', 'admin', NULL, ?)`,
		id, id, hash, hash, now.Unix(),
	); err != nil {
		t.Fatalf("insert image: %v", err)
	}
}

func assertRebuilderDatabaseState(t *testing.T, db *sql.DB, imageID, tokenID string) {
	t.Helper()
	var visibility, tokenStatus string
	var sessions, addedAliases, removedAliases int
	if err := db.QueryRow("SELECT visibility FROM images WHERE id = ?", imageID).Scan(&visibility); err != nil {
		t.Fatalf("query image visibility: %v", err)
	}
	if err := db.QueryRow("SELECT status FROM api_tokens WHERE id = ?", tokenID).Scan(&tokenStatus); err != nil {
		t.Fatalf("query API token status: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM image_aliases WHERE alias_path = '/legacy/add.jpg'").Scan(&addedAliases); err != nil {
		t.Fatalf("count added aliases: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM image_aliases WHERE alias_path = '/legacy/remove.jpg'").Scan(&removedAliases); err != nil {
		t.Fatalf("count removed aliases: %v", err)
	}
	if visibility != "private" || tokenStatus != "revoked" || sessions != 0 || addedAliases != 1 || removedAliases != 0 {
		t.Fatalf("database state: visibility=%s token=%s sessions=%d added=%d removed=%d", visibility, tokenStatus, sessions, addedAliases, removedAliases)
	}
}
