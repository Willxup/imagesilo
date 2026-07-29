package processor

import "errors"

type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatWebP Format = "webp"
	FormatGIF  Format = "gif"
)

var (
	ErrInvalidImage      = errors.New("uploaded file is not a valid supported image")
	ErrUnsupportedFormat = errors.New("image format is not supported")
	ErrFileTooLarge      = errors.New("image exceeds the maximum upload size")
	ErrTooManyPixels     = errors.New("image exceeds the maximum decoded pixel count")
	ErrUnavailable       = errors.New("image byte processing requires the production libvips build")
)

type Limits struct {
	MaxBytes       int64
	MaxTotalPixels int64
}

type Metadata struct {
	Format    Format
	MIMEType  string
	Extension string
	Width     int
	Height    int
	Frames    int
}

type Options struct {
	CompressionEnabled     bool
	JPEGQuality            int
	WebPQuality            int
	PNGCompressionLevel    int
	ConversionEnabled      bool
	ConversionWebPQuality  int
	ConversionWebPLossless bool
}

type Action string

const (
	ActionPreserve Action = "preserve"
	ActionCompress Action = "compress"
	ActionConvert  Action = "convert"
)

type Plan struct {
	Action       Action
	OutputFormat Format
}

type Engine interface {
	Transform(inputPath, outputPath string, metadata Metadata, options Options, plan Plan) error
	Thumbnail(inputPath, outputPath string) error
}

func SelectPlan(metadata Metadata, options Options) Plan {
	if metadata.Format == FormatGIF {
		return Plan{Action: ActionPreserve, OutputFormat: FormatGIF}
	}
	if options.ConversionEnabled && (metadata.Format == FormatJPEG || metadata.Format == FormatPNG) {
		return Plan{Action: ActionConvert, OutputFormat: FormatWebP}
	}
	if options.CompressionEnabled {
		switch metadata.Format {
		case FormatJPEG, FormatPNG, FormatWebP:
			return Plan{Action: ActionCompress, OutputFormat: metadata.Format}
		}
	}
	return Plan{Action: ActionPreserve, OutputFormat: metadata.Format}
}

func FormatMetadata(format Format) (mimeType, extension string, ok bool) {
	switch format {
	case FormatJPEG:
		return "image/jpeg", ".jpg", true
	case FormatPNG:
		return "image/png", ".png", true
	case FormatWebP:
		return "image/webp", ".webp", true
	case FormatGIF:
		return "image/gif", ".gif", true
	default:
		return "", "", false
	}
}
