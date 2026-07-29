package processor

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectSupportedFormatMatrix(t *testing.T) {
	fixtures := map[Format][]byte{
		FormatJPEG: testJPEG(t, 4, 3),
		FormatPNG:  testPNG(t, 4, 3),
		FormatWebP: testWebP(t),
		FormatGIF:  testAnimatedGIF(t, 4, 3),
	}
	for format, data := range fixtures {
		t.Run(string(format), func(t *testing.T) {
			path := writeFixture(t, data)
			metadata, err := InspectFile(context.Background(), path, Limits{MaxBytes: 1 << 20, MaxTotalPixels: 100})
			if err != nil {
				t.Fatalf("InspectFile() error = %v", err)
			}
			if metadata.Format != format || metadata.Width < 1 || metadata.Height < 1 {
				t.Fatalf("InspectFile() metadata = %+v", metadata)
			}
			if format == FormatGIF && metadata.Frames != 2 {
				t.Fatalf("animated GIF frames = %d, want 2", metadata.Frames)
			}
		})
	}
}

func TestInspectRejectsCorruptDisguisedAndExcessiveImages(t *testing.T) {
	pngData := testPNG(t, 4, 3)
	disguised := append([]byte{0xff, 0xd8, 0xff}, pngData...)
	for name, data := range map[string][]byte{
		"corrupt":        []byte("not an image"),
		"disguised":      disguised,
		"truncated-jpeg": truncateFixture(testJPEG(t, 32, 24), 16),
		"truncated-png":  truncateFixture(testPNG(t, 32, 24), 16),
		"truncated-webp": truncateFixture(testWebP(t), 4),
		"truncated-gif":  truncateFixture(testAnimatedGIF(t, 32, 24), 8),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := InspectFile(context.Background(), writeFixture(t, data), Limits{MaxBytes: 1 << 20, MaxTotalPixels: 100}); err == nil {
				t.Fatal("InspectFile() accepted invalid image bytes")
			}
		})
	}
	if _, err := InspectFile(context.Background(), writeFixture(t, pngData), Limits{MaxBytes: 1 << 20, MaxTotalPixels: 4}); err != ErrTooManyPixels {
		t.Fatalf("pixel limit error = %v, want ErrTooManyPixels", err)
	}
	if _, err := InspectFile(context.Background(), writeFixture(t, testAnimatedGIF(t, 4, 3)), Limits{MaxBytes: 1 << 20, MaxTotalPixels: 20}); err != ErrTooManyPixels {
		t.Fatalf("animated pixel limit error = %v, want ErrTooManyPixels", err)
	}
}

func truncateFixture(data []byte, count int) []byte {
	if count >= len(data) {
		return nil
	}
	return append([]byte(nil), data[:len(data)-count]...)
}

func TestInspectAcceptsHighCompressionAndExtremeAspectRatiosWithinPixelLimit(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		width  int
		height int
	}{
		{name: "high-compression-png", data: testSolidPNG(t, 2000, 2000), width: 2000, height: 2000},
		{name: "ultra-wide-png", data: testSolidPNG(t, 100_000, 1), width: 100_000, height: 1},
		{name: "ultra-tall-png", data: testSolidPNG(t, 1, 100_000), width: 1, height: 100_000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := InspectFile(context.Background(), writeFixture(t, test.data), Limits{
				MaxBytes: 1 << 20, MaxTotalPixels: 4_000_000,
			})
			if err != nil {
				t.Fatalf("InspectFile() error = %v", err)
			}
			if metadata.Format != FormatPNG || metadata.Width != test.width || metadata.Height != test.height {
				t.Fatalf("InspectFile() metadata = %+v", metadata)
			}
		})
	}
}

func TestSelectPlanKeepsIndependentCompressionAndConversionSemantics(t *testing.T) {
	tests := []struct {
		name     string
		format   Format
		options  Options
		expected Plan
	}{
		{name: "preserve default", format: FormatJPEG, expected: Plan{Action: ActionPreserve, OutputFormat: FormatJPEG}},
		{name: "compress jpeg", format: FormatJPEG, options: Options{CompressionEnabled: true}, expected: Plan{Action: ActionCompress, OutputFormat: FormatJPEG}},
		{name: "convert jpeg", format: FormatJPEG, options: Options{ConversionEnabled: true}, expected: Plan{Action: ActionConvert, OutputFormat: FormatWebP}},
		{name: "conversion wins", format: FormatPNG, options: Options{CompressionEnabled: true, ConversionEnabled: true}, expected: Plan{Action: ActionConvert, OutputFormat: FormatWebP}},
		{name: "conversion does not roundtrip webp", format: FormatWebP, options: Options{ConversionEnabled: true}, expected: Plan{Action: ActionPreserve, OutputFormat: FormatWebP}},
		{name: "gif always preserved", format: FormatGIF, options: Options{CompressionEnabled: true, ConversionEnabled: true}, expected: Plan{Action: ActionPreserve, OutputFormat: FormatGIF}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := Metadata{Format: test.format}
			if actual := SelectPlan(metadata, test.options); actual != test.expected {
				t.Fatalf("SelectPlan() = %+v, want %+v", actual, test.expected)
			}
		})
	}
}

func TestGateRejectsWithoutQueueing(t *testing.T) {
	gate := NewGate(1)
	release, ok := gate.TryAcquire()
	if !ok {
		t.Fatal("first gate acquisition failed")
	}
	if secondRelease, ok := gate.TryAcquire(); ok || secondRelease != nil {
		t.Fatal("full gate queued or accepted a second operation")
	}
	release()
	if release, ok := gate.TryAcquire(); !ok {
		t.Fatal("gate slot was not released")
	} else {
		release()
	}
}

func BenchmarkInspectFormats(b *testing.B) {
	fixtures := map[Format][]byte{
		FormatJPEG: testJPEG(b, 1600, 1200),
		FormatPNG:  testPNG(b, 800, 600),
		FormatWebP: testWebP(b),
		FormatGIF:  testAnimatedGIF(b, 320, 240),
	}
	for format, data := range fixtures {
		b.Run(string(format), func(b *testing.B) {
			path := writeFixture(b, data)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for index := 0; index < b.N; index++ {
				if _, err := InspectFile(context.Background(), path, Limits{MaxBytes: 20 << 20, MaxTotalPixels: 40_000_000}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type testHelper interface {
	Helper()
	Fatalf(string, ...any)
	TempDir() string
}

func writeFixture(t testHelper, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func testJPEG(t interface {
	Helper()
	Fatalf(string, ...any)
}, width, height int) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func testPNG(t interface {
	Helper()
	Fatalf(string, ...any)
}, width, height int) []byte {
	t.Helper()
	value := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value.Set(x, y, color.NRGBA{R: 20, G: uint8(x % 255), B: uint8(y % 255), A: uint8(128 + (x+y)%127)})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func testSolidPNG(t interface {
	Helper()
	Fatalf(string, ...any)
}, width, height int) []byte {
	t.Helper()
	value := image.NewNRGBA(image.Rect(0, 0, width, height))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func testAnimatedGIF(t interface {
	Helper()
	Fatalf(string, ...any)
}, width, height int) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White}
	first := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	second := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	for index := range second.Pix {
		second.Pix[index] = 1
	}
	var buffer bytes.Buffer
	if err := gif.EncodeAll(&buffer, &gif.GIF{
		Image: []*image.Paletted{first, second}, Delay: []int{5, 5}, LoopCount: 0,
	}); err != nil {
		t.Fatalf("gif.EncodeAll() error = %v", err)
	}
	return buffer.Bytes()
}

func testWebP(t interface {
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
