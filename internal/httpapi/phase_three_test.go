package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Willxup/imagesilo/internal/platform/processor"
)

type phaseThreeEngine struct {
	transformCalls int
}

func (e *phaseThreeEngine) Transform(inputPath, outputPath string, _ processor.Metadata, _ processor.Options, plan processor.Plan) error {
	e.transformCalls++
	if plan.Action == processor.ActionConvert && plan.OutputFormat == processor.FormatWebP {
		return os.WriteFile(outputPath, phaseThreeWebP(testingHelper{}), 0o600)
	}
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func (e *phaseThreeEngine) Thumbnail(_, outputPath string) error {
	return os.WriteFile(outputPath, phaseThreeJPEG(testingHelper{}, 1, 1), 0o600)
}

type testingHelper struct{}

func (testingHelper) Helper()               {}
func (testingHelper) Fatalf(string, ...any) { panic("unexpected fixture failure") }

func TestPhaseThreeFormatMatrixSystemLimitsAndThumbnails(t *testing.T) {
	engine := &phaseThreeEngine{}
	fixture := newHTTPFixture(t, engine, processor.NewGate(2), 2)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	systemResult := fixture.request(http.MethodGet, "/api/v1/system", nil, cookies, "", "")
	if systemResult.Code != http.StatusOK {
		t.Fatalf("system status = %d, body = %s", systemResult.Code, systemResult.Body.String())
	}
	var system systemResponse
	if err := json.Unmarshal(systemResult.Body.Bytes(), &system); err != nil {
		t.Fatalf("decode system response: %v", err)
	}
	if system.ProcessingConcurrency != 2 || len(system.SupportedFormats) != 4 ||
		(system.VIPSVersion != "disabled" && system.VIPSVersion != "8.18.4") {
		t.Fatalf("system response = %+v", system)
	}

	fixtures := []struct {
		name     string
		filename string
		mimeType string
		data     []byte
	}{
		{name: "jpeg", filename: "fixture.jpg", mimeType: "image/jpeg", data: phaseThreeJPEG(t, 4, 3)},
		{name: "png", filename: "fixture.png", mimeType: "image/png", data: phaseThreePNG(t, 4, 3)},
		{name: "webp", filename: "fixture.webp", mimeType: "image/webp", data: phaseThreeWebP(t)},
		{name: "gif", filename: "fixture.gif", mimeType: "image/gif", data: phaseThreeGIF(t, 4, 3)},
	}
	for _, test := range fixtures {
		t.Run(test.name, func(t *testing.T) {
			uploaded := fixture.uploadBytes(cookies, csrfToken, "public", "", test.filename, test.data)
			if uploaded.MIMEType != test.mimeType || uploaded.SourceSHA256 != uploaded.StoredSHA256 {
				t.Fatalf("uploaded image = %+v", uploaded)
			}
			thumbnail := fixture.request(http.MethodGet, uploaded.ThumbnailURL, nil, cookies, "", "")
			if thumbnail.Code != http.StatusOK || thumbnail.Header().Get("Content-Type") != "image/jpeg" {
				t.Fatalf("thumbnail response = %d, Content-Type = %q", thumbnail.Code, thumbnail.Header().Get("Content-Type"))
			}
			if test.name == "gif" {
				delivered := fixture.request(http.MethodGet, uploaded.StandardURL, nil, nil, "", "")
				if delivered.Code != http.StatusOK || !bytes.Equal(delivered.Body.Bytes(), test.data) {
					t.Fatal("animated GIF bytes changed during default upload")
				}
			}
		})
	}
	if engine.transformCalls != 0 {
		t.Fatalf("default format uploads invoked byte transform %d times", engine.transformCalls)
	}
}

func TestPhaseThreeProcessingSettingsDriveExplicitWebPConversion(t *testing.T) {
	engine := &phaseThreeEngine{}
	fixture := newHTTPFixture(t, engine, processor.NewGate(1), 1)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	settingsResult := fixture.request(http.MethodPatch, "/api/v1/settings/processing", map[string]any{
		"compressionEnabled":     false,
		"jpegQuality":            85,
		"webpQuality":            82,
		"pngCompressionLevel":    6,
		"conversionEnabled":      true,
		"conversionWebpQuality":  82,
		"conversionWebpLossless": false,
	}, cookies, csrfToken, "")
	if settingsResult.Code != http.StatusOK {
		t.Fatalf("update processing settings status = %d, body = %s", settingsResult.Code, settingsResult.Body.String())
	}
	uploaded := fixture.uploadBytes(cookies, csrfToken, "public", "", "convert.jpg", phaseThreeJPEG(t, 1, 1))
	if uploaded.MIMEType != "image/webp" || uploaded.Extension != ".webp" || uploaded.SourceSHA256 == uploaded.StoredSHA256 {
		t.Fatalf("converted upload = %+v", uploaded)
	}
	var summary map[string]any
	if err := json.Unmarshal(uploaded.ProcessingSummary, &summary); err != nil {
		t.Fatalf("decode processing summary: %v", err)
	}
	if summary["action"] != "convert" || summary["sourceFormat"] != "jpeg" || summary["storedFormat"] != "webp" {
		t.Fatalf("processing summary = %+v", summary)
	}
	delivered := fixture.request(http.MethodGet, uploaded.StandardURL, nil, nil, "", "")
	if delivered.Code != http.StatusOK || delivered.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("converted delivery = %d, Content-Type = %q", delivered.Code, delivered.Header().Get("Content-Type"))
	}
}

func TestPhaseThreeFullGateReturnsRetryable503WithoutTemporaryLeak(t *testing.T) {
	gate := processor.NewGate(1)
	release, ok := gate.TryAcquire()
	if !ok {
		t.Fatal("failed to occupy processing gate")
	}
	defer release()
	fixture := newHTTPFixture(t, &phaseThreeEngine{}, gate, 1)
	cookies, csrfToken, _ := fixture.login(nil, phaseTwoPassword)
	response := rawPhaseThreeUpload(t, fixture.router, cookies, csrfToken, phaseThreeJPEG(t, 4, 3))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("busy upload = %d, Retry-After = %q, body = %s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(fixture.dataDirectory, "tmp"))
	if err != nil {
		t.Fatalf("ReadDir(tmp) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files remain after busy response: %+v", entries)
	}
}

func rawPhaseThreeUpload(t *testing.T, router http.Handler, cookies []*http.Cookie, csrfToken string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "busy.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write upload data: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart.Close() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	addAuthentication(request, cookies, csrfToken, "")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func phaseThreeJPEG(t interface {
	Helper()
	Fatalf(string, ...any)
}, width, height int) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	value.Set(0, 0, color.RGBA{R: 200, G: 100, A: 255})
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, value, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func phaseThreePNG(t interface {
	Helper()
	Fatalf(string, ...any)
}, width, height int) []byte {
	t.Helper()
	value := image.NewNRGBA(image.Rect(0, 0, width, height))
	value.Set(0, 0, color.NRGBA{G: 180, A: 128})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buffer.Bytes()
}

func phaseThreeGIF(t interface {
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
	if err := gif.EncodeAll(&buffer, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{5, 5}}); err != nil {
		t.Fatalf("gif.EncodeAll() error = %v", err)
	}
	return buffer.Bytes()
}

func phaseThreeWebP(t interface {
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
