package auth

import (
	"net/mail"
	"strings"
)

func NormalizeEmail(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized || len(normalized) > 254 {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func NormalizeDisplayName(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)
	length := len([]rune(normalized))
	if length < 1 || length > 80 {
		return "", ErrInvalidDisplayName
	}
	return normalized, nil
}
