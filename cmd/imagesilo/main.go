package main

import (
	"fmt"
	"os"

	"github.com/Willxup/imagesilo/internal/command"
)

func main() {
	if err := command.Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "imagesilo: %v\n", err)
		os.Exit(1)
	}
}
