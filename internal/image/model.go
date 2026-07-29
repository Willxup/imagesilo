package image

import (
	"time"

	"github.com/Willxup/imagesilo/internal/platform/processor"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Image struct {
	ID                   string
	OriginalName         string
	StorageKey           string
	Extension            string
	MIMEType             string
	Width                int
	Height               int
	SourceSize           int64
	StoredSize           int64
	SourceSHA256         [32]byte
	StoredSHA256         [32]byte
	ProcessingSummary    string
	Visibility           Visibility
	UploadedVia          string
	UploadedByAPITokenID *string
	CreatedAt            time.Time
}

type UploadOptions struct {
	Visibility           Visibility
	UploadedVia          string
	UploadedByAPITokenID *string
	Limits               processor.Limits
	Processing           processor.Options
}
