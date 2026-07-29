package image

import "errors"

var (
	ErrFileTooLarge  = errors.New("image exceeds the maximum upload size")
	ErrInvalidJPEG   = errors.New("file is not a valid JPEG image")
	ErrTooManyPixels = errors.New("image exceeds the maximum pixel count")
)
