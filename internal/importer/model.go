package importer

import (
	"time"

	imagealias "github.com/Willxup/imagesilo/internal/alias"
	images "github.com/Willxup/imagesilo/internal/image"
	"github.com/Willxup/imagesilo/internal/platform/processor"
)

type Options struct {
	Visibility           images.Visibility
	UploadedByAPITokenID *string
	Limits               processor.Limits
}

type Result struct {
	Image images.Image
	Alias imagealias.Alias
}

type importSummary struct {
	Action             processor.Action `json:"action"`
	SourceFormat       processor.Format `json:"sourceFormat"`
	StoredFormat       processor.Format `json:"storedFormat"`
	Preserved          bool             `json:"preserved"`
	CompressionEnabled bool             `json:"compressionEnabled"`
	ConversionEnabled  bool             `json:"conversionEnabled"`
}

func importedAlias(id, path, imageID string, now time.Time) imagealias.Alias {
	return imagealias.Alias{ID: id, Path: path, ImageID: imageID, Source: "import", CreatedAt: now.UTC()}
}
