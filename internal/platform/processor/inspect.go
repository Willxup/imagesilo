package processor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	_ "golang.org/x/image/webp"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func InspectFile(ctx context.Context, path string, limits Limits) (Metadata, error) {
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("stat image for inspection: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return Metadata{}, ErrInvalidImage
	}
	if limits.MaxBytes > 0 && info.Size() > limits.MaxBytes {
		return Metadata{}, ErrFileTooLarge
	}

	file, err := os.Open(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("open image for inspection: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	header, err := reader.Peek(16)
	if err != nil && err != io.EOF {
		return Metadata{}, ErrInvalidImage
	}
	format, err := detectFormat(header)
	if err != nil {
		return Metadata{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Metadata{}, fmt.Errorf("rewind image for configuration: %w", err)
	}
	configuration, decodedFormat, err := stdimage.DecodeConfig(file)
	if err != nil || configuration.Width <= 0 || configuration.Height <= 0 || decodedFormat != string(format) {
		return Metadata{}, ErrInvalidImage
	}
	pixels := int64(configuration.Width) * int64(configuration.Height)
	if pixels <= 0 || (limits.MaxTotalPixels > 0 && pixels > limits.MaxTotalPixels) {
		return Metadata{}, ErrTooManyPixels
	}
	preflightFrames := 0
	if format == FormatGIF {
		maxFrames := int64(0)
		if limits.MaxTotalPixels > 0 {
			maxFrames = limits.MaxTotalPixels / pixels
		}
		preflightFrames, err = preflightGIFFrames(path, maxFrames)
		if err != nil {
			return Metadata{}, err
		}
	}
	frames, err := validateDecode(path, format, configuration.Width, configuration.Height)
	if err != nil {
		return Metadata{}, err
	}
	if preflightFrames > 0 && frames != preflightFrames {
		return Metadata{}, ErrInvalidImage
	}
	if limits.MaxTotalPixels > 0 && pixels*int64(frames) > limits.MaxTotalPixels {
		return Metadata{}, ErrTooManyPixels
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	mimeType, extension, ok := FormatMetadata(format)
	if !ok {
		return Metadata{}, ErrUnsupportedFormat
	}
	return Metadata{
		Format: format, MIMEType: mimeType, Extension: extension,
		Width: configuration.Width, Height: configuration.Height, Frames: frames,
	}, nil
}

func detectFormat(header []byte) (Format, error) {
	switch {
	case len(header) >= 3 && header[0] == 0xff && header[1] == 0xd8 && header[2] == 0xff:
		return FormatJPEG, nil
	case len(header) >= len(pngSignature) && bytes.Equal(header[:len(pngSignature)], pngSignature):
		return FormatPNG, nil
	case len(header) >= 6 && (bytes.Equal(header[:6], []byte("GIF87a")) || bytes.Equal(header[:6], []byte("GIF89a"))):
		return FormatGIF, nil
	case len(header) >= 12 && bytes.Equal(header[:4], []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WEBP")):
		return FormatWebP, nil
	default:
		return "", ErrUnsupportedFormat
	}
}
