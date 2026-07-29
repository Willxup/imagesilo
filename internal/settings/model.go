package settings

import "github.com/Willxup/imagesilo/internal/image"

type Settings struct {
	MaxUploadBytes         int64
	MaxBatchCount          int
	MaxTotalPixels         int64
	CompressionEnabled     bool
	JPEGQuality            int
	WebPQuality            int
	PNGCompressionLevel    int
	ConversionEnabled      bool
	ConversionWebPQuality  int
	ConversionWebPLossless bool
	DefaultVisibility      image.Visibility
	MaintenanceHour        int
}
