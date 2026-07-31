package imagealias

import "errors"

var (
	ErrAliasConflict = errors.New("alias path already exists")
	ErrAliasNotFound = errors.New("alias was not found")
	ErrImageNotFound = errors.New("target image was not found")
	ErrInvalidSource = errors.New("alias source is invalid")
	ErrInvalidImage  = errors.New("target image id is invalid")
	ErrInvalidCursor = errors.New("alias cursor is invalid")
)
