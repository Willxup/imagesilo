package image

import "errors"

var (
	ErrFileTooLarge          = errors.New("image exceeds the maximum upload size")
	ErrInvalidImage          = errors.New("file is not a valid supported image")
	ErrInvalidJPEG           = ErrInvalidImage
	ErrUnsupportedFormat     = errors.New("image format is not supported")
	ErrTooManyPixels         = errors.New("image exceeds the maximum decoded pixel count")
	ErrProcessingBusy        = errors.New("image processor is at capacity")
	ErrProcessingUnavailable = errors.New("image byte processing is unavailable")
	ErrImageNotFound         = errors.New("image was not found")
	ErrInvalidListFilter     = errors.New("image list filter is invalid")
)
