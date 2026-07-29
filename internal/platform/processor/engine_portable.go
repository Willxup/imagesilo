//go:build !vips || !cgo

package processor

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
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
	source, _, err := image.Decode(file)
	file.Close()
	if err != nil {
		return ErrInvalidImage
	}
	bounds := source.Bounds()
	width, height := fitWithin(bounds.Dx(), bounds.Dy(), 512, 512)
	background := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(background, background.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
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
