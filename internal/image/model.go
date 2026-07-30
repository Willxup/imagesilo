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
	UpdatedAt            time.Time
}

type UploadOptions struct {
	Visibility           Visibility
	UploadedVia          string
	UploadedByAPITokenID *string
	Limits               processor.Limits
	Processing           processor.Options
}

type ListFilter struct {
	Limit          int
	Cursor         string
	Query          string
	Visibility     Visibility
	MIMEType       string
	UploadedVia    string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	MinStoredBytes int64
	MaxStoredBytes int64
	MinWidth       int
	MaxWidth       int
	MinHeight      int
	MaxHeight      int
}

type Page struct {
	Items      []Image
	NextCursor string
}

type DeleteResult struct {
	ImageID           string
	ImageFileDeleted  bool
	ThumbnailDeleted  bool
	CleanupPending    bool
	ImageCleanupError error
	ThumbCleanupError error
}

type ConversionResult struct {
	Image               Image
	OriginalFileDeleted bool
	ThumbnailUpdated    bool
	CleanupPending      bool
	OriginalFileError   error
	ThumbnailError      error
}
