// Command export-images recreates the Populous screen images from local Amiga
// picture files, including the title's five bitplanes and embedded palettes.
package main

import (
	"flag"
	"fmt"
	"os"

	"go-populous/internal/assets"
)

func main() {
	amigaDir := flag.String("amiga", "assets/amiga", "directory containing imported Amiga data")
	outputDir := flag.String("out", "assets/extracted-images", "directory for reconstructed screen PNGs")
	flag.Parse()
	if err := assets.ExportScreenImages(*amigaDir, *outputDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Rebuilt demo, load, lord and qaz screen images in %s\n", *outputDir)
}
