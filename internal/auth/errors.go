package auth

import "errors"

var (
	ErrAdminExists        = errors.New("administrator already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidCSRF        = errors.New("invalid CSRF token")
	ErrPasswordTooShort   = errors.New("password must contain at least 12 bytes")
)
