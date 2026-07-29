package auth

import "errors"

var (
	ErrAdminExists        = errors.New("administrator already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
)
