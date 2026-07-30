package delivery

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeAliasPath(t *testing.T) {
	valid := map[string]string{
		"/legacy/photo.jpg":                "/legacy/photo.jpg",
		"/legacy/a%20b.jpg":                "/legacy/a%20b.jpg",
		"/legacy/旧图.png":                   "/legacy/%E6%97%A7%E5%9B%BE.png",
		"/legacy/%E6%97%A7%E5%9B%BE.png":   "/legacy/%E6%97%A7%E5%9B%BE.png",
		"/image/not-a-canonical-uuid":      "/image/not-a-canonical-uuid",
		"/healthz/legacy-image.jpg":        "/healthz/legacy-image.jpg",
		"/legacy/case-sensitive/Photo.JPG": "/legacy/case-sensitive/Photo.JPG",
	}
	for input, want := range valid {
		t.Run("valid_"+strings.ReplaceAll(input, "/", "_"), func(t *testing.T) {
			got, err := NormalizeAliasPath(input)
			if err != nil || got != want {
				t.Fatalf("NormalizeAliasPath(%q) = %q, %v; want %q", input, got, err, want)
			}
		})
	}

	invalid := []string{
		"", "relative.jpg", "//example.com/a.jpg", "/legacy//a.jpg", "/legacy/./a.jpg",
		"/legacy/../a.jpg", "/legacy/%2E%2E/a.jpg", "/legacy/%252e%252e/a.jpg",
		"/legacy\\a.jpg", "/legacy/%5C/a.jpg", "/legacy/%00.jpg", "/legacy/%61.jpg",
		"/legacy/%e6%97%a7.png", "/legacy/a.jpg?download=1", "/legacy/a.jpg#part",
		"/legacy/trailing/", "/legacy/a%2Fb.jpg", "/legacy/%ZZ.jpg",
	}
	for _, input := range invalid {
		t.Run("invalid_"+strings.ReplaceAll(input, "/", "_"), func(t *testing.T) {
			if _, err := NormalizeAliasPath(input); !errors.Is(err, ErrInvalidAliasPath) {
				t.Fatalf("NormalizeAliasPath(%q) error = %v, want ErrInvalidAliasPath", input, err)
			}
		})
	}
}

func TestNormalizeAliasPathRejectsReservedRoutes(t *testing.T) {
	for _, input := range []string{
		"/api/v1", "/api/v1/images", "/admin", "/admin/settings", "/assets", "/assets/app.js", "/brand", "/brand/imagesilo-logo.png",
		"/healthz", "/readyz", "/image/019c1234-5678-7abc-8def-0123456789ab",
		"/image/019C1234-5678-7ABC-8DEF-0123456789AB", "/image/019c123456787abc8def0123456789ab",
	} {
		if _, err := NormalizeAliasPath(input); !errors.Is(err, ErrReservedAliasPath) {
			t.Errorf("NormalizeAliasPath(%q) error = %v, want ErrReservedAliasPath", input, err)
		}
	}
}
