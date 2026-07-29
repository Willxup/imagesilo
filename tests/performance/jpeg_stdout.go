package main

import (
	"flag"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"os"
)

func main() {
	width := flag.Int("width", 3000, "synthetic JPEG width")
	height := flag.Int("height", 2000, "synthetic JPEG height")
	quality := flag.Int("quality", 90, "JPEG quality")
	flag.Parse()
	if *width <= 0 || *height <= 0 || int64(*width)*int64(*height) > 40_000_000 {
		log.Fatal("dimensions must be positive and no more than 40 million pixels")
	}

	value := image.NewRGBA(image.Rect(0, 0, *width, *height))
	for y := 0; y < *height; y++ {
		for x := 0; x < *width; x++ {
			value.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / *width),
				G: uint8((y * 255) / *height),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	if err := jpeg.Encode(os.Stdout, value, &jpeg.Options{Quality: *quality}); err != nil {
		log.Fatal(err)
	}
}
