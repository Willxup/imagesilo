//go:build !vips || !cgo

package processor

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type portableEngine struct{}

func NewEngine() Engine {
	return portableEngine{}
}

func (portableEngine) Transform(_, _ string, _ Metadata, _ Options, plan Plan) error {
	if plan.Action == ActionPreserve {
		return nil
	}
	return ErrUnavailable
}

func (portableEngine) Thumbnail(inputPath, outputPath string) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer file.Close()
	configuration, format, err := image.DecodeConfig(file)
	if err != nil {
		return ErrInvalidImage
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	source, _, err := image.Decode(file)
	if err != nil {
		return ErrInvalidImage
	}
	bounds := source.Bounds()
	backgroundColor := color.Color(color.White)
	if format == "gif" {
		bounds = image.Rect(0, 0, configuration.Width, configuration.Height)
		var backgroundIndex [1]byte
		if _, err := file.ReadAt(backgroundIndex[:], 11); err != nil {
			return ErrInvalidImage
		}
		if palette, ok := configuration.ColorModel.(color.Palette); ok && int(backgroundIndex[0]) < len(palette) {
			backgroundColor = palette[backgroundIndex[0]]
		}
		for _, entry := range source.(*image.Paletted).Palette {
			if _, _, _, alpha := entry.RGBA(); alpha == 0 {
				backgroundColor = color.White
				break
			}
		}
		// Preserve the logical canvas and frame offset without allocating a
		// second full-size image or decoding the remaining animation frames.
		source = gifCanvas{Image: source, rectangle: bounds}
	}
	width, height := fitWithin(bounds.Dx(), bounds.Dy(), 512, 512)
	background := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(background, background.Bounds(), &image.Uniform{C: backgroundColor}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(background, background.Bounds(), source, bounds, draw.Over, nil)
	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	err = jpeg.Encode(output, background, &jpeg.Options{Quality: 80})
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}

type gifCanvas struct {
	image.Image
	rectangle image.Rectangle
}

func (canvas gifCanvas) Bounds() image.Rectangle {
	return canvas.rectangle
}

func (canvas gifCanvas) At(x, y int) color.Color {
	if !image.Pt(x, y).In(canvas.Image.Bounds()) {
		return color.Transparent
	}
	return canvas.Image.At(x, y)
}

func fitWithin(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}
	ratioWidth := float64(maxWidth) / float64(width)
	ratioHeight := float64(maxHeight) / float64(height)
	ratio := ratioWidth
	if ratioHeight < ratio {
		ratio = ratioHeight
	}
	resultWidth := int(float64(width) * ratio)
	resultHeight := int(float64(height) * ratio)
	if resultWidth < 1 {
		resultWidth = 1
	}
	if resultHeight < 1 {
		resultHeight = 1
	}
	return resultWidth, resultHeight
}
