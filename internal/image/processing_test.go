package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Willxup/imagesilo/db/migrations"
	"github.com/Willxup/imagesilo/internal/delivery"
	"github.com/Willxup/imagesilo/internal/platform/database"
	"github.com/Willxup/imagesilo/internal/platform/processor"
	"github.com/Willxup/imagesilo/internal/platform/storage"
)

type fakeEngine struct {
	transform      func(inputPath, outputPath string, plan processor.Plan) error
	thumbnail      func(inputPath, outputPath string) error
	transformCalls int
}

func (f *fakeEngine) Transform(inputPath, outputPath string, _ processor.Metadata, _ processor.Options, plan processor.Plan) error {
	f.transformCalls++
	return f.transform(inputPath, outputPath, plan)
}

func (f *fakeEngine) Thumbnail(inputPath, outputPath string) error {
	if f.thumbnail != nil {
		return f.thumbnail(inputPath, outputPath)
	}
	return os.WriteFile(outputPath, onePixelJPEG(testingT{}), 0o600)
}

type testingT struct{}

func (testingT) Helper()                           {}
func (testingT) Fatalf(format string, args ...any) { panic("unexpected fixture failure") }

type readTracker struct {
	read bool
}

func (r *readTracker) Read([]byte) (int, error) {
	r.read = true
	return 0, errors.New("reader must not be consumed")
}

func TestCompressionRejectsNonSmallerOutputAndKeepsUniqueUploads(t *testing.T) {
	service, filesystem, _, closeDB := newProcessingTestService(t, &fakeEngine{transform: copyTransform})
	defer closeDB()
	data := testJPEG(t)
	options := processingUploadOptions(processor.Options{CompressionEnabled: true, JPEGQuality: 85})
	first, err := service.Upload(context.Background(), bytes.NewReader(data), "first.jpg", options, time.Now())
	if err != nil {
		t.Fatalf("Upload(first) error = %v", err)
	}
	second, err := service.Upload(context.Background(), bytes.NewReader(data), "second.jpg", options, time.Now())
	if err != nil {
		t.Fatalf("Upload(second) error = %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("two uploads of identical bytes reused one UUID")
	}
	if first.SourceSHA256 != first.StoredSHA256 || first.SourceSize != first.StoredSize {
		t.Fatal("non-smaller compression output replaced the original bytes")
	}
	var summary processingSummary
	if err := json.Unmarshal([]byte(first.ProcessingSummary), &summary); err != nil {
		t.Fatalf("decode processing summary: %v", err)
	}
	if !summary.Preserved || !summary.CompressionRejected || summary.Action != processor.ActionPreserve {
		t.Fatalf("processing summary = %+v", summary)
	}
	thumbnail, err := filesystem.OpenThumbnail(first.ID)
	if err != nil {
		t.Fatalf("OpenThumbnail() error = %v", err)
	}
	if err := thumbnail.Close(); err != nil {
		t.Fatalf("Close(thumbnail) error = %v", err)
	}
}

func TestConversionKeepsExplicitWebPSemanticsEvenWhenBytesChange(t *testing.T) {
	engine := &fakeEngine{transform: func(_, outputPath string, plan processor.Plan) error {
		if plan.Action != processor.ActionConvert || plan.OutputFormat != processor.FormatWebP {
			return errors.New("unexpected conversion plan")
		}
		return os.WriteFile(outputPath, onePixelWebP(t), 0o600)
	}}
	service, _, _, closeDB := newProcessingTestService(t, engine)
	defer closeDB()
	data := onePixelJPEG(t)
	value, err := service.Upload(context.Background(), bytes.NewReader(data), "convert.jpg", processingUploadOptions(processor.Options{
		ConversionEnabled: true, ConversionWebPQuality: 82,
	}), time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if value.MIMEType != "image/webp" || value.Extension != ".webp" || value.SourceSHA256 == value.StoredSHA256 {
		t.Fatalf("converted image = %+v", value)
	}
	var summary processingSummary
	if err := json.Unmarshal([]byte(value.ProcessingSummary), &summary); err != nil {
		t.Fatalf("decode processing summary: %v", err)
	}
	if summary.Action != processor.ActionConvert || summary.SourceFormat != processor.FormatJPEG || summary.StoredFormat != processor.FormatWebP {
		t.Fatalf("processing summary = %+v", summary)
	}
}

func TestAnimatedGIFAlwaysPreservesFrames(t *testing.T) {
	engine := &fakeEngine{transform: func(_, _ string, _ processor.Plan) error {
		return errors.New("GIF transform must not be called")
	}}
	service, filesystem, _, closeDB := newProcessingTestService(t, engine)
	defer closeDB()
	data := animatedGIF(t)
	value, err := service.Upload(context.Background(), bytes.NewReader(data), "animated.gif", processingUploadOptions(processor.Options{
		CompressionEnabled: true, ConversionEnabled: true,
	}), time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if engine.transformCalls != 0 || value.MIMEType != "image/gif" || value.SourceSHA256 != value.StoredSHA256 {
		t.Fatalf("GIF processing changed animation semantics: calls=%d value=%+v", engine.transformCalls, value)
	}
	stored, err := filesystem.Open(value.StorageKey)
	if err != nil {
		t.Fatalf("Open(stored GIF) error = %v", err)
	}
	decoded, err := gif.DecodeAll(stored)
	stored.Close()
	if err != nil || len(decoded.Image) != 2 {
		t.Fatalf("stored GIF decode = (%d frames, %v)", len(decoded.Image), err)
	}
}

func TestConvertToWebPReplacesStoredContentAndKeepsImageIdentity(t *testing.T) {
	engine := &fakeEngine{transform: func(_, outputPath string, plan processor.Plan) error {
		if plan.Action != processor.ActionConvert || plan.OutputFormat != processor.FormatWebP {
			return errors.New("unexpected conversion plan")
		}
		return os.WriteFile(outputPath, onePixelWebP(t), 0o600)
	}}
	service, filesystem, _, closeDB := newProcessingTestService(t, engine)
	defer closeDB()
	value, err := service.Upload(context.Background(), bytes.NewReader(onePixelJPEG(t)), "existing.jpg", processingUploadOptions(processor.Options{}), time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	oldStorageKey := value.StorageKey
	result, err := service.ConvertToWebP(context.Background(), value.ID, processor.Options{ConversionWebPQuality: 82})
	if err != nil {
		t.Fatalf("ConvertToWebP() error = %v", err)
	}
	if result.Image.ID != value.ID || result.Image.MIMEType != "image/webp" || result.Image.Extension != ".webp" || result.Image.OriginalName != "existing.webp" {
		t.Fatalf("converted image = %+v", result.Image)
	}
	if result.Image.StorageKey == oldStorageKey || result.CleanupPending {
		t.Fatalf("conversion result = %+v", result)
	}
	if old, err := filesystem.Open(oldStorageKey); err == nil {
		old.Close()
		t.Fatal("old stored file still exists")
	}
	stored, err := filesystem.Open(result.Image.StorageKey)
	if err != nil {
		t.Fatalf("Open(new stored image) error = %v", err)
	}
	stored.Close()
	if _, err := service.ConvertToWebP(context.Background(), value.ID, processor.Options{ConversionWebPQuality: 82}); !errors.Is(err, ErrConversionNotSupported) {
		t.Fatalf("second ConvertToWebP() error = %v", err)
	}
}

func TestConvertToWebPRemovesStaleThumbnailWhenReplacementIsUnavailable(t *testing.T) {
	thumbnailUnavailable := false
	engine := &fakeEngine{
		transform: func(_, outputPath string, _ processor.Plan) error {
			return os.WriteFile(outputPath, onePixelWebP(t), 0o600)
		},
		thumbnail: func(_, outputPath string) error {
			if thumbnailUnavailable {
				return processor.ErrUnavailable
			}
			return os.WriteFile(outputPath, onePixelJPEG(t), 0o600)
		},
	}
	service, filesystem, _, closeDB := newProcessingTestService(t, engine)
	defer closeDB()
	value, err := service.Upload(context.Background(), bytes.NewReader(onePixelJPEG(t)), "stale.jpg", processingUploadOptions(processor.Options{}), time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	thumbnailUnavailable = true
	result, err := service.ConvertToWebP(context.Background(), value.ID, processor.Options{ConversionWebPQuality: 82})
	if err != nil || !result.ThumbnailUpdated || result.CleanupPending {
		t.Fatalf("ConvertToWebP() = %+v, %v", result, err)
	}
	if thumbnail, err := filesystem.OpenThumbnail(value.ID); err == nil {
		thumbnail.Close()
		t.Fatal("conversion left the old thumbnail available")
	}
}

func TestConvertToWebPSerializesWithVisibilityChange(t *testing.T) {
	transformStarted := make(chan struct{})
	releaseTransform := make(chan struct{})
	engine := &fakeEngine{transform: func(_, outputPath string, _ processor.Plan) error {
		if err := os.WriteFile(outputPath, onePixelWebP(t), 0o600); err != nil {
			return err
		}
		close(transformStarted)
		<-releaseTransform
		return nil
	}}
	service, _, _, closeDB := newProcessingTestService(t, engine)
	defer closeDB()
	value, err := service.Upload(context.Background(), bytes.NewReader(onePixelJPEG(t)), "visibility.jpg", processingUploadOptions(processor.Options{}), time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	visibilityAttempted := make(chan struct{})
	service.beforeMutationLock = func(operation, _ string) {
		if operation == "visibility" {
			close(visibilityAttempted)
		}
	}
	conversionDone := make(chan error, 1)
	go func() {
		_, conversionErr := service.ConvertToWebP(context.Background(), value.ID, processor.Options{ConversionWebPQuality: 82})
		conversionDone <- conversionErr
	}()
	<-transformStarted
	visibilityDone := make(chan error, 1)
	go func() {
		_, visibilityErr := service.ChangeVisibility(context.Background(), value.ID, VisibilityPrivate)
		visibilityDone <- visibilityErr
	}()
	<-visibilityAttempted
	close(releaseTransform)
	if err := <-conversionDone; err != nil {
		t.Fatalf("ConvertToWebP() error = %v", err)
	}
	if err := <-visibilityDone; err != nil {
		t.Fatalf("ChangeVisibility() error = %v", err)
	}
	stored, err := service.Get(context.Background(), value.ID)
	if err != nil || stored.Visibility != VisibilityPrivate {
		t.Fatalf("stored image = %+v, %v", stored, err)
	}
	target, ok := service.index.Get(value.ID)
	if !ok || target.Visibility != string(VisibilityPrivate) {
		t.Fatalf("delivery target = %+v, found=%v", target, ok)
	}
}

func TestConvertToWebPSerializesWithDeleteWithoutGhostIndex(t *testing.T) {
	engine := &fakeEngine{transform: func(_, outputPath string, _ processor.Plan) error {
		return os.WriteFile(outputPath, onePixelWebP(t), 0o600)
	}}
	service, _, _, closeDB := newProcessingTestService(t, engine)
	defer closeDB()
	value, err := service.Upload(context.Background(), bytes.NewReader(onePixelJPEG(t)), "delete.jpg", processingUploadOptions(processor.Options{}), time.Now())
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	contentUpdated := make(chan struct{})
	releaseConversion := make(chan struct{})
	deleteAttempted := make(chan struct{})
	service.afterContentUpdate = func(string) {
		close(contentUpdated)
		<-releaseConversion
	}
	service.beforeMutationLock = func(operation, _ string) {
		if operation == "delete" {
			close(deleteAttempted)
		}
	}
	conversionDone := make(chan error, 1)
	go func() {
		_, conversionErr := service.ConvertToWebP(context.Background(), value.ID, processor.Options{ConversionWebPQuality: 82})
		conversionDone <- conversionErr
	}()
	<-contentUpdated
	deleteDone := make(chan error, 1)
	go func() {
		_, deleteErr := service.Delete(context.Background(), value.ID)
		deleteDone <- deleteErr
	}()
	<-deleteAttempted
	close(releaseConversion)
	if err := <-conversionDone; err != nil {
		t.Fatalf("ConvertToWebP() error = %v", err)
	}
	if err := <-deleteDone; err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Get(context.Background(), value.ID); !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("Get() after delete error = %v", err)
	}
	if target, ok := service.index.Get(value.ID); ok {
		t.Fatalf("deleted image left ghost delivery target: %+v", target)
	}
}

func TestFullProcessingGateAndInvalidImageLeaveNoTemporaryFiles(t *testing.T) {
	dataDirectory := prepareUploadTestData(t)
	db, err := database.Open(filepath.Join(dataDirectory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db); err != nil {
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	filesystem := storage.NewFilesystem(dataDirectory)
	gate := processor.NewGate(1)
	release, ok := gate.TryAcquire()
	if !ok {
		t.Fatal("failed to occupy processing gate")
	}
	service := NewServiceWithProcessor(NewRepository(db), filesystem, delivery.NewIndex(), &fakeEngine{transform: copyTransform}, gate)
	reader := &readTracker{}
	if _, err := service.Upload(context.Background(), reader, "busy.jpg", processingUploadOptions(processor.Options{}), time.Now()); !errors.Is(err, ErrProcessingBusy) {
		t.Fatalf("busy Upload() error = %v, want ErrProcessingBusy", err)
	}
	if reader.read {
		t.Fatal("busy Upload() consumed request bytes before admission")
	}
	release()
	if _, err := service.Upload(context.Background(), bytes.NewReader([]byte("not an image")), "bad.jpg", processingUploadOptions(processor.Options{}), time.Now()); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("invalid Upload() error = %v, want ErrUnsupportedFormat", err)
	}
	entries, err := os.ReadDir(filepath.Join(dataDirectory, "tmp"))
	if err != nil {
		t.Fatalf("ReadDir(tmp) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files remain after rejected uploads: %+v", entries)
	}
}

func TestThumbnailFailureRejectsUploadWithoutLeavingFilesOrRows(t *testing.T) {
	service, _, dataDirectory, closeDB := newProcessingTestService(t, &fakeEngine{
		transform: copyTransform,
		thumbnail: func(_, _ string) error {
			return errors.New("thumbnail failed")
		},
	})
	defer closeDB()

	if _, err := service.Upload(
		context.Background(),
		bytes.NewReader(testJPEG(t)),
		"thumbnail-error.jpg",
		processingUploadOptions(processor.Options{}),
		time.Now(),
	); err == nil || err.Error() != "thumbnail failed" {
		t.Fatalf("Upload() error = %v, want thumbnail failure", err)
	}
	if values, err := service.List(context.Background(), 10); err != nil || len(values) != 0 {
		t.Fatalf("List() after thumbnail failure = (%+v, %v)", values, err)
	}
	for _, directory := range []string{filepath.Join(dataDirectory, "images"), filepath.Join(dataDirectory, "tmp")} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("ReadDir(%s) error = %v", directory, err)
		}
		if len(entries) != 0 {
			t.Fatalf("files remain in %s after thumbnail failure: %+v", directory, entries)
		}
	}
}

func newProcessingTestService(t *testing.T, engine processor.Engine) (*Service, *storage.Filesystem, string, func()) {
	t.Helper()
	dataDirectory := prepareUploadTestData(t)
	db, err := database.Open(filepath.Join(dataDirectory, "db", "imagesilo.db"))
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	if err := migrations.Apply(context.Background(), db); err != nil {
		db.Close()
		t.Fatalf("migrations.Apply() error = %v", err)
	}
	filesystem := storage.NewFilesystem(dataDirectory)
	service := NewServiceWithProcessor(NewRepository(db), filesystem, delivery.NewIndex(), engine, processor.NewGate(1))
	return service, filesystem, dataDirectory, func() { _ = db.Close() }
}

func processingUploadOptions(options processor.Options) UploadOptions {
	return UploadOptions{
		Visibility: VisibilityPublic, UploadedVia: "admin",
		Limits: processor.Limits{MaxBytes: 1 << 20, MaxTotalPixels: 100}, Processing: options,
	}
}

func copyTransform(inputPath, outputPath string, _ processor.Plan) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func onePixelJPEG(t interface {
	Helper()
	Fatalf(string, ...any)
}) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 1, 1))
	value.Set(0, 0, color.RGBA{R: 80, G: 120, B: 160, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func onePixelWebP(t interface {
	Helper()
	Fatalf(string, ...any)
}) []byte {
	t.Helper()
	const encoded = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString(WebP) error = %v", err)
	}
	return data
}

func animatedGIF(t interface {
	Helper()
	Fatalf(string, ...any)
}) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	second := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	for index := range second.Pix {
		second.Pix[index] = 1
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{5, 5}}); err != nil {
		t.Fatalf("gif.EncodeAll() error = %v", err)
	}
	return buffer.Bytes()
}
