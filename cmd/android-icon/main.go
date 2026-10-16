// android-icon scales the original title artwork into Android launcher assets.
// No artwork is cropped or redrawn: the black margin fits adaptive icon masks.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	f, err := os.Open("assets/extracted-images/load.pic.png")
	if err != nil {
		return err
	}
	source, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	root := "android/app/src/main/res"
	for _, spec := range []struct {
		directory   string
		size, width int
		transparent bool
	}{
		{"mipmap-mdpi", 48, 40, false},
		{"mipmap-hdpi", 72, 60, false},
		{"mipmap-xhdpi", 96, 80, false},
		{"mipmap-xxhdpi", 144, 120, false},
		{"mipmap-xxxhdpi", 192, 160, false},
		// 56 of 108 dp keeps the complete 320x200 image in the safe circle.
		{"drawable-nodpi", 432, 224, true},
	} {
		canvas := image.NewNRGBA(image.Rect(0, 0, spec.size, spec.size))
		if !spec.transparent {
			draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
		}
		bounds := source.Bounds()
		height := spec.width * bounds.Dy() / bounds.Dx()
		left, top := (spec.size-spec.width)/2, (spec.size-height)/2
		for y := 0; y < height; y++ {
			for x := 0; x < spec.width; x++ {
				canvas.Set(left+x, top+y, source.At(bounds.Min.X+x*bounds.Dx()/spec.width, bounds.Min.Y+y*bounds.Dy()/height))
			}
		}
		name := "ic_launcher.png"
		if spec.transparent {
			name = "ic_launcher_foreground.png"
		}
		directory := filepath.Join(root, spec.directory)
		if err := os.MkdirAll(directory, 0755); err != nil {
			return err
		}
		output, err := os.Create(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		err = png.Encode(output, canvas)
		closeErr := output.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
