package setup

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/database"
)

func TestInitializeIsAtomicAndOneTime(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	service := NewService(db)
	initialized, err := service.Initialized(context.Background())
	if err != nil || initialized {
		t.Fatalf("Initialized() = %v, %v", initialized, err)
	}
	request := Request{
		DisplayName: "Will", Email: "ADMIN@example.com", Password: "a secure setup password",
		DefaultVisibility: images.VisibilityPrivate, JPEGQuality: 85, WebPQuality: 82,
		PNGCompressionLevel: 6, ConversionWebPQuality: 80,
	}
	admin, err := service.Initialize(context.Background(), request, time.Unix(1_900_000_000, 0))
	if err != nil || admin.DisplayName != "Will" || admin.Email != "admin@example.com" {
		t.Fatalf("Initialize() = %+v, %v", admin, err)
	}
	if _, err := service.Initialize(context.Background(), request, time.Now()); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Initialize() error = %v", err)
	}
	var visibility string
	if err := db.QueryRow("SELECT default_visibility FROM app_settings WHERE singleton = 1").Scan(&visibility); err != nil || visibility != "private" {
		t.Fatalf("default visibility = %q, %v", visibility, err)
	}
}
