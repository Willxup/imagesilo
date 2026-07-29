package apitoken

import "errors"

var (
	ErrInvalidToken      = errors.New("invalid API token")
	ErrInvalidName       = errors.New("API token name must contain 1 to 100 characters")
	ErrInvalidScope      = errors.New("invalid API token scope")
	ErrInvalidExpiration = errors.New("API token expiration must be in the future")
	ErrTokenNotFound     = errors.New("API token not found")
)
