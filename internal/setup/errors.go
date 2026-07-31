package setup

import "errors"

var (
	ErrAlreadyInitialized    = errors.New("ImageSilo is already initialized")
	ErrInvalidBootstrapToken = errors.New("invalid bootstrap token")
	ErrInvalidSettings       = errors.New("invalid initial settings")
)
