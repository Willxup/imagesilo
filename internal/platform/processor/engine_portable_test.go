//go:build !vips || !cgo

package processor

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"os"
	"testing"
)

func TestPortableGIFThumbnailPreservesCanvasOffsetBackgroundAndFirstFrame(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	for _, test := range []struct {
		name        string
		bounds      image.Rectangle
		transparent bool
		globalTable bool
	}{
		{name: "offset-opaque", bounds: image.Rect(16, 8, 48, 24), globalTable: true},
		{name: "offset-transparent", bounds: image.Rect(16, 8, 48, 24), transparent: true, globalTable: true},
		{name: "offset-local-table", bounds: image.Rect(16, 8, 48, 24)},
		{name: "full-frame", bounds: image.Rect(0, 0, 64, 32), globalTable: true},
		{name: "full-frame-transparent", bounds: image.Rect(0, 0, 64, 32), transparent: true, globalTable: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			palette := color.Palette{color.Black, color.White, red, blue}
			if test.transparent {
				palette[0] = color.Transparent
			}
			first := image.NewPaletted(test.bounds, palette)
			for index := range first.Pix {
				first.Pix[index] = 2
			}
			if test.transparent {
				for y := test.bounds.Min.Y; y < test.bounds.Max.Y; y++ {
					for x := test.bounds.Min.X; x < test.bounds.Min.X+test.bounds.Dx()/2; x++ {
						first.SetColorIndex(x, y, 0)
					}
				}
			}
			second := image.NewPaletted(image.Rect(0, 0, 64, 32), palette)
			for index := range second.Pix {
				second.Pix[index] = 3
			}
			configuration := image.Config{Width: 64, Height: 32}
			if test.globalTable {
				configuration.ColorModel = palette
			}
			var encoded bytes.Buffer
			if err := gif.EncodeAll(&encoded, &gif.GIF{
				Image: []*image.Paletted{first, second}, Delay: []int{5, 5},
				Config: configuration, BackgroundIndex: 3,
			}); err != nil {
				t.Fatalf("gif.EncodeAll() error = %v", err)
			}
			input := writeFixture(t, encoded.Bytes())
			output := writeFixture(t, nil)
			if err := NewEngine().Thumbnail(input, output); err != nil {
				t.Fatalf("Thumbnail() error = %v", err)
			}
			file, err := os.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			thumbnail, err := jpeg.Decode(file)
			if err != nil {
				t.Fatalf("jpeg.Decode() error = %v", err)
			}
			if thumbnail.Bounds() != image.Rect(0, 0, 64, 32) {
				t.Fatalf("thumbnail bounds = %v, want logical 64x32 canvas", thumbnail.Bounds())
			}
			assertThumbnailColor(t, thumbnail, image.Pt(test.bounds.Max.X-8, 16), red)
			if test.transparent {
				assertThumbnailColor(t, thumbnail, image.Pt(test.bounds.Min.X+4, 16), color.White)
			}
			if test.bounds.Min.X != 0 {
				background := color.Color(blue)
				if test.transparent || !test.globalTable {
					background = color.White
				}
				assertThumbnailColor(t, thumbnail, image.Pt(4, 16), background)
			}
		})
	}
}

func TestPortableGIFThumbnailScalesLogicalCanvasAndFrameOffset(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	palette := color.Palette{color.White, red, blue}
	frame := image.NewPaletted(image.Rect(256, 128, 768, 384), palette)
	for index := range frame.Pix {
		frame.Pix[index] = 1
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{
		Image: []*image.Paletted{frame}, Delay: []int{5}, BackgroundIndex: 2,
		Config: image.Config{ColorModel: palette, Width: 1024, Height: 512},
	}); err != nil {
		t.Fatalf("gif.EncodeAll() error = %v", err)
	}
	input := writeFixture(t, encoded.Bytes())
	output := writeFixture(t, nil)
	if err := NewEngine().Thumbnail(input, output); err != nil {
		t.Fatalf("Thumbnail() error = %v", err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	thumbnail, err := jpeg.Decode(file)
	if err != nil {
		t.Fatalf("jpeg.Decode() error = %v", err)
	}
	if thumbnail.Bounds() != image.Rect(0, 0, 512, 256) {
		t.Fatalf("thumbnail bounds = %v, want scaled 512x256 canvas", thumbnail.Bounds())
	}
	assertThumbnailColor(t, thumbnail, image.Pt(256, 128), red)
	for _, point := range []image.Point{image.Pt(64, 128), image.Pt(448, 128), image.Pt(256, 32), image.Pt(256, 224)} {
		assertThumbnailColor(t, thumbnail, point, blue)
	}
}

func assertThumbnailColor(t *testing.T, thumbnail image.Image, point image.Point, expected color.Color) {
	t.Helper()
	actual := color.NRGBAModel.Convert(thumbnail.At(point.X, point.Y)).(color.NRGBA)
	want := color.NRGBAModel.Convert(expected).(color.NRGBA)
	for _, pair := range [][2]uint8{{actual.R, want.R}, {actual.G, want.G}, {actual.B, want.B}} {
		if difference := int(pair[0]) - int(pair[1]); difference < -30 || difference > 30 {
			t.Fatalf("thumbnail color at %v = %v, want approximately %v", point, actual, want)
		}
	}
}
