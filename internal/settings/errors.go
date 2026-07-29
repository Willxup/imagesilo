package settings

import "errors"

var (
	ErrInvalidVisibility = errors.New("invalid default visibility")
	ErrInvalidProcessing = errors.New("invalid image processing settings")
)
