package imagealias

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/indexbarrier"
	"github.com/Willxup/imagesilo/internal/platform/database"
)

const aliasTestImageID = "019c1234-5678-7abc-8def-0123456789ab"

func TestAliasLifecycleCommitsDatabaseBeforeIndex(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	insertAliasTestImage(t, db, aliasTestImageID)

	index := delivery.NewIndex()
	index.Add(aliasTestImageID, delivery.Target{StorageKey: aliasTestImageID})
	service := NewService(NewRepository(db), index, indexbarrier.New())
	now := time.Unix(1_700_000_000, 0).UTC()
	created, err := service.Create(context.Background(), "/legacy/旧图.jpg", aliasTestImageID, " legacy-import ", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Path != "/legacy/%E6%97%A7%E5%9B%BE.jpg" || created.Source != "legacy-import" {
		t.Fatalf("created alias = %+v", created)
	}
	if imageID, ok := index.ResolveAlias(created.Path); !ok || imageID != aliasTestImageID {
		t.Fatalf("index alias = %q, %t", imageID, ok)
	}

	duplicate, err := service.Create(context.Background(), created.Path, aliasTestImageID, "duplicate", now.Add(time.Second))
	if !errors.Is(err, ErrAliasConflict) || duplicate.ID != "" {
		t.Fatalf("duplicate Create() = %+v, %v", duplicate, err)
	}
	resolved, err := service.Resolve(context.Background(), "/legacy/旧图.jpg")
	if err != nil || resolved.ID != created.ID {
		t.Fatalf("Resolve() = %+v, %v", resolved, err)
	}
	values, err := service.List(context.Background(), 10)
	if err != nil || len(values) != 1 || values[0].ID != created.ID {
		t.Fatalf("List() = %+v, %v", values, err)
	}

	if err := service.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := index.ResolveAlias(created.Path); ok {
		t.Fatal("deleted alias remained in delivery index")
	}
	if _, err := service.Resolve(context.Background(), created.Path); !errors.Is(err, ErrAliasNotFound) {
		t.Fatalf("Resolve() after delete error = %v", err)
	}
}

func TestAliasValidationAndMissingTarget(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	service := NewService(NewRepository(db), delivery.NewIndex(), indexbarrier.New())
	now := time.Now()

	tests := []struct {
		path, imageID, source string
		wantErr               error
	}{
		{"/", aliasTestImageID, "test", delivery.ErrReservedAliasPath},
		{"/api/v1/images", aliasTestImageID, "test", delivery.ErrReservedAliasPath},
		{"/legacy/../image.jpg", aliasTestImageID, "test", delivery.ErrInvalidAliasPath},
		{"/legacy/image.jpg", "not-a-uuid", "test", ErrInvalidImage},
		{"/legacy/image.jpg", aliasTestImageID, "\n", ErrInvalidSource},
		{"/legacy/image.jpg", aliasTestImageID, "test", ErrImageNotFound},
	}
	for _, test := range tests {
		if _, err := service.Create(context.Background(), test.path, test.imageID, test.source, now); !errors.Is(err, test.wantErr) {
			t.Errorf("Create(%q, %q, %q) error = %v, want %v", test.path, test.imageID, test.source, err, test.wantErr)
		}
	}
}

func TestAliasListUsesStableCursorPagination(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	insertAliasTestImage(t, db, aliasTestImageID)
	service := NewService(NewRepository(db), delivery.NewIndex(), indexbarrier.New())
	base := time.Unix(1_700_000_000, 0).UTC()
	created := make([]Alias, 0, 3)
	for index, path := range []string{"/legacy/old.jpg", "/legacy/middle.jpg", "/legacy/new.jpg"} {
		value, err := service.Create(context.Background(), path, aliasTestImageID, "test", base.Add(time.Duration(index)*time.Second))
		if err != nil {
			t.Fatalf("Create(%s) error = %v", path, err)
		}
		created = append(created, value)
	}
	first, err := service.ListPage(context.Background(), 2, "")
	if err != nil || len(first.Items) != 2 || first.Items[0].ID != created[2].ID || first.Items[1].ID != created[1].ID || first.NextCursor == "" {
		t.Fatalf("first ListPage() = %+v, %v", first, err)
	}
	second, err := service.ListPage(context.Background(), 2, first.NextCursor)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != created[0].ID || second.NextCursor != "" {
		t.Fatalf("second ListPage() = %+v, %v", second, err)
	}
	if _, err := service.ListPage(context.Background(), 2, "invalid"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("invalid ListPage() error = %v", err)
	}
}

func TestAliasDeleteSerializesWithSamePathReplacement(t *testing.T) {
	for _, targetID := range []string{aliasTestImageID, "019c1234-5678-7abc-8def-0123456789ac"} {
		t.Run(targetID, func(t *testing.T) {
			db := openAliasTestDatabase(t)
			defer db.Close()
			insertAliasTestImage(t, db, aliasTestImageID)
			if targetID != aliasTestImageID {
				insertAliasTestImage(t, db, targetID)
			}
			const unrelatedImageID = "019c1234-5678-7abc-8def-0123456789ad"
			insertAliasTestImage(t, db, unrelatedImageID)
			index := delivery.NewIndex()
			index.Add(aliasTestImageID, delivery.Target{StorageKey: aliasTestImageID})
			index.Add(targetID, delivery.Target{StorageKey: targetID})
			index.Add(unrelatedImageID, delivery.Target{StorageKey: unrelatedImageID})
			service := NewService(NewRepository(db), index, indexbarrier.New())
			old, err := service.Create(context.Background(), "/legacy/replaced.jpg", aliasTestImageID, "old", time.Now())
			if err != nil {
				t.Fatal(err)
			}
			deletedSQL := make(chan struct{})
			resumeDelete := make(chan struct{})
			releaseDelete := sync.OnceFunc(func() { close(resumeDelete) })
			defer releaseDelete()
			service.afterRepositoryChange = func(operation, path string) {
				if operation == "delete" {
					close(deletedSQL)
					<-resumeDelete
				}
			}
			deleted := make(chan error, 1)
			go func() { deleted <- service.Delete(context.Background(), old.ID) }()
			<-deletedSQL
			replacement := make(chan error, 1)
			go func() {
				_, err := service.Create(context.Background(), old.Path, targetID, "new", time.Now())
				replacement <- err
			}()
			if _, err := service.Create(context.Background(), "/legacy/other.jpg", unrelatedImageID, "other", time.Now()); err != nil {
				t.Fatalf("unrelated alias create failed: %v", err)
			}
			if _, ok := index.GetAlias(old.Path); !ok {
				t.Fatal("delivery reads were blocked or changed before index removal")
			}
			select {
			case err := <-replacement:
				t.Fatalf("replacement passed the unfinished delete: %v", err)
			case <-time.After(25 * time.Millisecond):
			}
			releaseDelete()
			if err := <-deleted; err != nil {
				t.Fatal(err)
			}
			if err := <-replacement; err != nil {
				t.Fatal(err)
			}
			persisted, err := service.Resolve(context.Background(), old.Path)
			if err != nil || persisted.ID == old.ID || persisted.ImageID != targetID {
				t.Fatalf("replacement in SQLite = %+v, %v", persisted, err)
			}
			if imageID, ok := index.ResolveAlias(old.Path); !ok || imageID != targetID {
				t.Fatalf("replacement in index = %q, %t", imageID, ok)
			}
			if err := service.Delete(context.Background(), old.ID); !errors.Is(err, ErrAliasNotFound) {
				t.Fatalf("stale Delete() error = %v, want ErrAliasNotFound", err)
			}
		})
	}
}

func TestAliasDelayedCreateCannotResurrectAfterDelete(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	insertAliasTestImage(t, db, aliasTestImageID)
	index := delivery.NewIndex()
	index.Add(aliasTestImageID, delivery.Target{StorageKey: aliasTestImageID})
	service := NewService(NewRepository(db), index, indexbarrier.New())
	createdSQL := make(chan struct{})
	resumeCreate := make(chan struct{})
	releaseCreate := sync.OnceFunc(func() { close(resumeCreate) })
	defer releaseCreate()
	service.afterRepositoryChange = func(operation, path string) {
		if operation == "create" {
			close(createdSQL)
			<-resumeCreate
		}
	}
	created := make(chan error, 1)
	go func() {
		_, err := service.Create(context.Background(), "/legacy/delayed.jpg", aliasTestImageID, "test", time.Now())
		created <- err
	}()
	<-createdSQL
	persisted, err := service.Resolve(context.Background(), "/legacy/delayed.jpg")
	if err != nil {
		t.Fatal(err)
	}
	deleted := make(chan error, 1)
	go func() { deleted <- service.Delete(context.Background(), persisted.ID) }()
	select {
	case err := <-deleted:
		t.Fatalf("delete passed the unfinished create: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	releaseCreate()
	if err := <-created; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Resolve(context.Background(), persisted.Path); !errors.Is(err, ErrAliasNotFound) {
		t.Fatalf("Resolve() after deletion error = %v", err)
	}
	if _, ok := index.ResolveAlias(persisted.Path); ok {
		t.Fatal("delayed create resurrected the deleted alias")
	}
}

func TestAliasFailedMutationsReleasePathLock(t *testing.T) {
	db := openAliasTestDatabase(t)
	defer db.Close()
	insertAliasTestImage(t, db, aliasTestImageID)
	index := delivery.NewIndex()
	index.Add(aliasTestImageID, delivery.Target{StorageKey: aliasTestImageID})
	service := NewService(NewRepository(db), index, indexbarrier.New())
	initial, err := service.Create(context.Background(), "/legacy/retry.jpg", aliasTestImageID, "test", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	run := func(operation func() error, want error) {
		t.Helper()
		completed := make(chan error, 1)
		go func() { completed <- operation() }()
		select {
		case err := <-completed:
			if !errors.Is(err, want) {
				t.Fatalf("mutation error = %v, want %v", err, want)
			}
		case <-time.After(time.Second):
			t.Fatal("failed mutation did not release the alias path lock")
		}
	}
	run(func() error {
		_, err := service.Create(context.Background(), initial.Path, aliasTestImageID, "duplicate", time.Now())
		return err
	}, ErrAliasConflict)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	run(func() error {
		_, err := service.Create(canceled, initial.Path, aliasTestImageID, "canceled", time.Now())
		return err
	}, context.Canceled)
	run(func() error { return service.Delete(context.Background(), initial.ID) }, nil)
	run(func() error {
		_, err := service.Create(context.Background(), initial.Path, aliasTestImageID, "retry", time.Now())
		return err
	}, nil)
}

func openAliasTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	return db
}

func insertAliasTestImage(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	hash := make([]byte, 32)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, 'test.jpg', ?, '.jpg', 'image/jpeg', 1, 1, 1, 1, ?, ?, '{}', 'public', 'admin', NULL, ?)`,
		id, id, hash, hash, time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert test image: %v", err)
	}
}
