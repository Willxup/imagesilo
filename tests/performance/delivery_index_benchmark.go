package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

type indexBenchmarkResult struct {
	Paths                  int     `json:"paths"`
	BuildMilliseconds      float64 `json:"buildMilliseconds"`
	HeapDeltaBytes         uint64  `json:"heapDeltaBytes"`
	BytesPerPath           float64 `json:"bytesPerPath"`
	Lookups                int     `json:"lookups"`
	StandardHitNanoseconds float64 `json:"standardHitNanoseconds"`
	AliasHitNanoseconds    float64 `json:"aliasHitNanoseconds"`
	AliasMissNanoseconds   float64 `json:"aliasMissNanoseconds"`
	LookupFailures         int     `json:"lookupFailures"`
}

var benchmarkTarget delivery.Target

func main() {
	rawSizes := flag.String("sizes", "10000,100000", "comma-separated alias path counts")
	lookups := flag.Int("lookups", 1_000_000, "lookups per hit or miss measurement")
	flag.Parse()
	if *lookups < 1 {
		fatal("lookups must be positive")
	}
	sizes, err := parseSizes(*rawSizes)
	if err != nil {
		fatal(err.Error())
	}
	encoder := json.NewEncoder(os.Stdout)
	for _, size := range sizes {
		result, err := measure(size, *lookups)
		if err != nil {
			fatal(err.Error())
		}
		if err := encoder.Encode(result); err != nil {
			fatal(fmt.Sprintf("encode result: %v", err))
		}
	}
}

func measure(size, lookups int) (indexBenchmarkResult, error) {
	directory, err := os.MkdirTemp("", "imagesilo-delivery-index-")
	if err != nil {
		return indexBenchmarkResult{}, fmt.Errorf("create benchmark directory: %w", err)
	}
	defer os.RemoveAll(directory)
	for _, path := range []string{"db", "images", filepath.Join("cache", "thumbnails"), "tmp"} {
		if err := os.MkdirAll(filepath.Join(directory, path), 0o750); err != nil {
			return indexBenchmarkResult{}, fmt.Errorf("create benchmark data directories: %w", err)
		}
	}
	db, err := database.Open(filepath.Join(directory, "db", "imagesilo.db"))
	if err != nil {
		return indexBenchmarkResult{}, err
	}
	defer db.Close()
	ctx := context.Background()
	if err := migrations.Apply(ctx, db); err != nil {
		return indexBenchmarkResult{}, err
	}
	const imageID = "019c1234-5678-7abc-8def-0123456789ab"
	if err := seedBenchmarkDatabase(ctx, db, imageID, size); err != nil {
		return indexBenchmarkResult{}, err
	}
	if err := os.WriteFile(filepath.Join(directory, "images", imageID), []byte("image"), 0o640); err != nil {
		return indexBenchmarkResult{}, fmt.Errorf("write benchmark image: %w", err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	snapshot, loaded, err := delivery.Build(ctx, db, storage.NewFilesystem(directory))
	if err != nil {
		return indexBenchmarkResult{}, err
	}
	if loaded.LoadedAliasCount != size || len(loaded.LoadedIDs) != 1 || len(loaded.MissingIDs) != 0 {
		return indexBenchmarkResult{}, fmt.Errorf("unexpected loader result: %+v", loaded)
	}
	index := delivery.NewIndex()
	index.ReplaceAll(snapshot.Targets, snapshot.Aliases)
	buildDuration := time.Since(started)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	heapDelta := uint64(0)
	if after.HeapAlloc > before.HeapAlloc {
		heapDelta = after.HeapAlloc - before.HeapAlloc
	}

	sampleCount := size
	if sampleCount > 4096 {
		sampleCount = 4096
	}
	hits := make([]string, sampleCount)
	misses := make([]string, sampleCount)
	for value := 0; value < sampleCount; value++ {
		hits[value] = benchmarkPath(value)
		misses[value] = fmt.Sprintf("/missing/%08d.jpg", value)
	}
	failures := 0
	standardStarted := time.Now()
	for value := 0; value < lookups; value++ {
		target, ok := index.Get(imageID)
		if !ok {
			failures++
		}
		benchmarkTarget = target
	}
	standardDuration := time.Since(standardStarted)
	aliasHitStarted := time.Now()
	for value := 0; value < lookups; value++ {
		target, ok := index.GetAlias(hits[value%sampleCount])
		if !ok {
			failures++
		}
		benchmarkTarget = target
	}
	aliasHitDuration := time.Since(aliasHitStarted)
	aliasMissStarted := time.Now()
	for value := 0; value < lookups; value++ {
		target, ok := index.GetAlias(misses[value%sampleCount])
		if ok {
			failures++
		}
		benchmarkTarget = target
	}
	aliasMissDuration := time.Since(aliasMissStarted)
	runtime.KeepAlive(index)

	return indexBenchmarkResult{
		Paths: size, BuildMilliseconds: milliseconds(buildDuration), HeapDeltaBytes: heapDelta,
		BytesPerPath: float64(heapDelta) / float64(size), Lookups: lookups,
		StandardHitNanoseconds: nanosecondsPerOperation(standardDuration, lookups),
		AliasHitNanoseconds:    aliasHitDuration.Seconds() * 1e9 / float64(lookups),
		AliasMissNanoseconds:   aliasMissDuration.Seconds() * 1e9 / float64(lookups),
		LookupFailures:         failures,
	}, nil
}

func seedBenchmarkDatabase(ctx context.Context, db databaseExecutor, imageID string, aliases int) error {
	hash := make([]byte, 32)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO images(
			id, original_name, storage_key, extension, mime_type, width, height,
			source_size, stored_size, source_sha256, stored_sha256, processing_summary,
			visibility, uploaded_via, uploaded_by_api_token_id, created_at
		) VALUES (?, 'benchmark.jpg', ?, '.jpg', 'image/jpeg', 1, 1, 5, 5, ?, ?, '{}', 'public', 'admin', NULL, ?)`,
		imageID, imageID, hash, hash, 1_700_000_000,
	); err != nil {
		return fmt.Errorf("insert benchmark image: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin benchmark alias seed: %w", err)
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx, `
		INSERT INTO image_aliases(id, alias_path, image_id, source, created_at)
		VALUES (?, ?, ?, 'benchmark', ?)`)
	if err != nil {
		return fmt.Errorf("prepare benchmark alias seed: %w", err)
	}
	defer statement.Close()
	for value := 0; value < aliases; value++ {
		if _, err := statement.ExecContext(ctx, fmt.Sprintf("alias-%08d", value), benchmarkPath(value), imageID, 1_700_000_000); err != nil {
			return fmt.Errorf("insert benchmark alias %d: %w", value, err)
		}
	}
	if err := statement.Close(); err != nil {
		return fmt.Errorf("close benchmark alias statement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit benchmark alias seed: %w", err)
	}
	return nil
}

type databaseExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func parseSizes(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 1 {
			return nil, fmt.Errorf("invalid path count %q", part)
		}
		result = append(result, value)
	}
	return result, nil
}

func benchmarkPath(value int) string {
	return fmt.Sprintf("/legacy/2026/07/%08d-image.jpg", value)
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func nanosecondsPerOperation(duration time.Duration, operations int) float64 {
	return duration.Seconds() * 1e9 / float64(operations)
}

func fatal(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
