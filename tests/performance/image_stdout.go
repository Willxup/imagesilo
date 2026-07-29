package main

import (
	"flag"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
)

func main() {
	format := flag.String("format", "jpeg", "fixture format: jpeg, png, or gif")
	width := flag.Int("width", 1600, "fixture width")
	height := flag.Int("height", 1200, "fixture height")
	quality := flag.Int("quality", 90, "JPEG quality")
	flag.Parse()
	if *width <= 0 || *height <= 0 {
		log.Fatal("width and height must be positive")
	}
	switch *format {
	case "jpeg":
		value := rgbaFixture(*width, *height)
		if err := jpeg.Encode(os.Stdout, value, &jpeg.Options{Quality: *quality}); err != nil {
			log.Fatal(err)
		}
	case "png":
		value := rgbaFixture(*width, *height)
		if err := png.Encode(os.Stdout, value); err != nil {
			log.Fatal(err)
		}
	case "gif":
		palette := color.Palette{color.Black, color.White, color.RGBA{R: 40, G: 160, B: 220, A: 255}}
		first := image.NewPaletted(image.Rect(0, 0, *width, *height), palette)
		second := image.NewPaletted(image.Rect(0, 0, *width, *height), palette)
		for y := 0; y < *height; y++ {
			for x := 0; x < *width; x++ {
				first.SetColorIndex(x, y, uint8((x/16+y/16)%2))
				second.SetColorIndex(x, y, uint8(1+(x/16+y/16)%2))
			}
		}
		if err := gif.EncodeAll(os.Stdout, &gif.GIF{
			Image: []*image.Paletted{first, second}, Delay: []int{8, 8}, LoopCount: 0,
		}); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unsupported fixture format %q", *format)
	}
}

func rgbaFixture(width, height int) *image.RGBA {
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value.SetRGBA(x, y, color.RGBA{
				R: uint8((x*13 + y*3) % 256),
				G: uint8((x*5 + y*11) % 256),
				B: uint8((x + y*7) % 256),
				A: 255,
			})
		}
	}
	return value
}
