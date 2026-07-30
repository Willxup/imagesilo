package delivery

import (
	"errors"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxAliasPathBytes = 2048

var (
	ErrInvalidAliasPath  = errors.New("alias path is invalid")
	ErrReservedAliasPath = errors.New("alias path conflicts with a reserved route")
)

func NormalizeAliasPath(raw string) (string, error) {
	if raw == "" || len(raw) > maxAliasPathBytes || raw[0] != '/' || strings.HasPrefix(raw, "//") || !utf8.ValidString(raw) {
		return "", ErrInvalidAliasPath
	}
	if raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "\\\x00?#\r\n\t") {
		return "", ErrInvalidAliasPath
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || !utf8.ValidString(decoded) {
		return "", ErrInvalidAliasPath
	}
	if strings.ContainsRune(decoded, '%') || strings.ContainsAny(decoded, "\\\x00?#\r\n\t") || strings.HasPrefix(decoded, "//") {
		return "", ErrInvalidAliasPath
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return "", ErrInvalidAliasPath
		}
	}
	if path.Clean(decoded) != decoded {
		return "", ErrInvalidAliasPath
	}
	canonical := (&url.URL{Path: decoded}).EscapedPath()
	if canonical == "" || len(canonical) > maxAliasPathBytes {
		return "", ErrInvalidAliasPath
	}
	if strings.ContainsRune(raw, '%') && raw != canonical {
		return "", ErrInvalidAliasPath
	}
	if IsReservedAliasPath(canonical) {
		return "", ErrReservedAliasPath
	}
	return canonical, nil
}

func IsReservedAliasPath(canonical string) bool {
	decoded, err := url.PathUnescape(canonical)
	if err != nil {
		return true
	}
	for _, prefix := range []string{"/api/v1", "/admin", "/assets", "/brand"} {
		if decoded == prefix || strings.HasPrefix(decoded, prefix+"/") {
			return true
		}
	}
	if decoded == "/healthz" || decoded == "/readyz" {
		return true
	}
	const imagePrefix = "/image/"
	if strings.HasPrefix(decoded, imagePrefix) {
		rawID := strings.TrimPrefix(decoded, imagePrefix)
		_, err := uuid.Parse(rawID)
		return err == nil
	}
	return false
}
