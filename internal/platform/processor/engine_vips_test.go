//go:build cgo && vips

package processor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVIPSEngineExactVersionCodecMatrixAndConversion(t *testing.T) {
	if err := Startup(); err != nil {
		t.Fatalf("Startup() error = %v", err)
	}
	if VIPSVersion() != "8.18.4" {
		t.Fatalf("VIPSVersion() = %q, want 8.18.4", VIPSVersion())
	}
	engine := NewEngine()
	fixtures := map[Format][]byte{
		FormatJPEG: testJPEG(t, 32, 24),
		FormatPNG:  testPNG(t, 32, 24),
		FormatWebP: testWebP(t),
		FormatGIF:  testAnimatedGIF(t, 32, 24),
	}
	for format, data := range fixtures {
		t.Run("thumbnail-"+string(format), func(t *testing.T) {
			input := writeFixture(t, data)
			output := filepath.Join(t.TempDir(), "thumbnail.jpg")
			if err := engine.Thumbnail(input, output); err != nil {
				t.Fatalf("Thumbnail() error = %v", err)
			}
			metadata, err := InspectFile(context.Background(), output, Limits{MaxBytes: 1 << 20, MaxTotalPixels: 40_000_000})
			if err != nil || metadata.Format != FormatJPEG {
				t.Fatalf("thumbnail metadata = (%+v, %v)", metadata, err)
			}
		})
	}

	input := writeFixture(t, fixtures[FormatJPEG])
	output := filepath.Join(t.TempDir(), "converted.webp")
	metadata, err := InspectFile(context.Background(), input, Limits{MaxBytes: 1 << 20, MaxTotalPixels: 40_000_000})
	if err != nil {
		t.Fatalf("InspectFile(input) error = %v", err)
	}
	if err := engine.Transform(input, output, metadata, Options{
		ConversionWebPQuality: 82,
	}, Plan{Action: ActionConvert, OutputFormat: FormatWebP}); err != nil {
		t.Fatalf("Transform(JPEG to WebP) error = %v", err)
	}
	converted, err := InspectFile(context.Background(), output, Limits{MaxBytes: 1 << 20, MaxTotalPixels: 40_000_000})
	if err != nil || converted.Format != FormatWebP || converted.Width != metadata.Width || converted.Height != metadata.Height {
		t.Fatalf("converted metadata = (%+v, %v)", converted, err)
	}
	if info, err := os.Stat(output); err != nil || info.Size() == 0 {
		t.Fatalf("converted output stat = (%v, %v)", info, err)
	}
}
