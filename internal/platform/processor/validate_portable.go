//go:build !vips || !cgo

package processor

import (
	stdimage "image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/webp"
)

func validateDecode(path string, format Format, width, height int) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if format == FormatGIF {
		decoded, err := gif.DecodeAll(file)
		if err != nil || len(decoded.Image) == 0 {
			return 0, ErrInvalidImage
		}
		for _, frame := range decoded.Image {
			if frame.Bounds().Dx() != width || frame.Bounds().Dy() != height {
				return 0, ErrInvalidImage
			}
		}
		return len(decoded.Image), nil
	}
	decoded, decodedFormat, err := stdimage.Decode(file)
	if err != nil || decodedFormat != string(format) || decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
		return 0, ErrInvalidImage
	}
	return 1, nil
}
