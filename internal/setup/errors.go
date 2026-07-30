package setup

import "errors"

var (
	ErrAlreadyInitialized = errors.New("ImageSilo is already initialized")
	ErrInvalidSettings    = errors.New("invalid initial settings")
)
