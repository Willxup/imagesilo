//go:build cgo && vips

package processor

/*
#cgo pkg-config: vips
#include <stdlib.h>
#include <vips/vips.h>

enum {
	IMAGESILO_SAVE_JPEG = 1,
	IMAGESILO_SAVE_PNG = 2,
	IMAGESILO_SAVE_WEBP = 3
};

static int imagesilo_transform(
	const char *input,
	const char *output,
	int output_format,
	int quality,
	int compression,
	int lossless
) {
	VipsImage *image = vips_image_new_from_file(input,
		"access", VIPS_ACCESS_SEQUENTIAL,
		NULL);
	if (image == NULL) {
		return -1;
	}
	int result = -1;
	switch (output_format) {
	case IMAGESILO_SAVE_JPEG:
		result = vips_jpegsave(image, output,
			"Q", quality,
			"interlace", TRUE,
			NULL);
		break;
	case IMAGESILO_SAVE_PNG:
		result = vips_pngsave(image, output,
			"compression", compression,
			NULL);
		break;
	case IMAGESILO_SAVE_WEBP:
		result = vips_webpsave(image, output,
			"Q", quality,
			"lossless", lossless,
			"effort", 4,
			NULL);
		break;
	}
	g_object_unref(image);
	return result;
}

static int imagesilo_thumbnail(const char *input, const char *output) {
	VipsImage *thumbnail = NULL;
	if (vips_thumbnail(input, &thumbnail, 512,
		"height", 512,
		"size", VIPS_SIZE_DOWN,
		"no-rotate", TRUE,
		NULL)) {
		return -1;
	}
	int result = vips_jpegsave(thumbnail, output,
		"Q", 80,
		"interlace", TRUE,
		NULL);
	g_object_unref(thumbnail);
	return result;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type vipsEngine struct{}

func NewEngine() Engine {
	return vipsEngine{}
}

func (vipsEngine) Transform(inputPath, outputPath string, _ Metadata, options Options, plan Plan) error {
	if err := Startup(); err != nil {
		return err
	}
	input := C.CString(inputPath)
	output := C.CString(outputPath)
	defer C.free(unsafe.Pointer(input))
	defer C.free(unsafe.Pointer(output))

	outputFormat := 0
	quality := 0
	compression := options.PNGCompressionLevel
	lossless := 0
	switch plan.OutputFormat {
	case FormatJPEG:
		outputFormat = C.IMAGESILO_SAVE_JPEG
		quality = options.JPEGQuality
	case FormatPNG:
		outputFormat = C.IMAGESILO_SAVE_PNG
	case FormatWebP:
		outputFormat = C.IMAGESILO_SAVE_WEBP
		quality = options.WebPQuality
		if plan.Action == ActionConvert {
			quality = options.ConversionWebPQuality
			if options.ConversionWebPLossless {
				lossless = 1
			}
		}
	default:
		return ErrUnsupportedFormat
	}
	if C.imagesilo_transform(input, output, C.int(outputFormat), C.int(quality), C.int(compression), C.int(lossless)) != 0 {
		return vipsError("transform image")
	}
	return nil
}

func (vipsEngine) Thumbnail(inputPath, outputPath string) error {
	if err := Startup(); err != nil {
		return err
	}
	input := C.CString(inputPath)
	output := C.CString(outputPath)
	defer C.free(unsafe.Pointer(input))
	defer C.free(unsafe.Pointer(output))
	if C.imagesilo_thumbnail(input, output) != 0 {
		return vipsError("generate thumbnail")
	}
	return nil
}

func vipsError(operation string) error {
	message := C.GoString(C.vips_error_buffer())
	C.vips_error_clear()
	if message == "" {
		message = "unknown libvips error"
	}
	return fmt.Errorf("%s: %s", operation, message)
}
