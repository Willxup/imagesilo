package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/config"
	"github.com/Willxup/imagesilo/internal/platform/database"
)

type synchronizedLogBuffer struct {
	mu      sync.Mutex
	content strings.Builder
}

func (b *synchronizedLogBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.content.Write(value)
}

func (b *synchronizedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.content.String()
}

func TestBuildLogsBootstrapTokenOnlyBeforeInitialization(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		initialized bool
	}{
		{name: "uninitialized", initialized: false},
		{name: "initialized", initialized: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.Config{
				ListenAddress: "127.0.0.1:0", DataDirectory: filepath.Join(t.TempDir(), "data"),
				ProcessingConcurrency: 1, DeliveryConcurrency: 64,
				ShutdownTimeout: 5 * time.Second, CookieSecure: false,
			}
			if err := cfg.PrepareDataDirectories(); err != nil {
				t.Fatal(err)
			}
			db, err := database.Open(cfg.DatabasePath())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := migrations.Apply(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			if testCase.initialized {
				if _, err := db.ExecContext(context.Background(), `
					INSERT INTO admin(id, display_name, email, password_hash, created_at, updated_at)
					VALUES ('administrator', 'Admin', 'admin@example.com', 'unused', 1, 1)
				`); err != nil {
					t.Fatal(err)
				}
			}

			var logs synchronizedLogBuffer
			application, err := Build(context.Background(), cfg, db, slog.New(slog.NewJSONHandler(&logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			application.Close()

			output := logs.String()
			hasBootstrapToken := strings.Contains(output, `"bootstrap_token":"isb_`)
			if hasBootstrapToken == testCase.initialized {
				t.Fatalf("initialized = %v, bootstrap token log presence = %v, logs = %s", testCase.initialized, hasBootstrapToken, output)
			}
		})
	}
}
