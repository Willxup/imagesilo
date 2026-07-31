package auth

import "errors"

var (
	ErrAdminExists        = errors.New("administrator already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidCSRF        = errors.New("invalid CSRF token")
	ErrPasswordTooShort   = errors.New("password must contain at least 12 bytes")
	ErrPasswordTooLong    = errors.New("password must contain at most 1024 bytes")
	ErrInvalidEmail       = errors.New("invalid administrator email address")
	ErrInvalidDisplayName = errors.New("display name must contain 1 to 80 characters")
)
