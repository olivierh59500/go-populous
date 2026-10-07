package assets

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"go-populous/internal/populous"
)

// ExportScreenImages rebuilds the four screen PNGs from their original picture
// files. It does not need reference images, an emulator, or an executable.
func ExportScreenImages(amigaDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create image output: %w", err)
	}
	for _, name := range []string{"demo", "load", "lord", "qaz"} {
		data, err := os.ReadFile(filepath.Join(amigaDir, name+".pic"))
		if err != nil {
			return fmt.Errorf("read %s.pic: %w", name, err)
		}
		img, err := populous.DecodeAmigaPicture(data, populous.ScreenWidth, populous.ScreenHeight)
		if err != nil {
			return fmt.Errorf("decode %s.pic: %w", name, err)
		}
		file, err := os.CreateTemp(outputDir, ".screen-*.png")
		if err != nil {
			return fmt.Errorf("create %s picture: %w", name, err)
		}
		path := file.Name()
		encodeErr := png.Encode(file, img)
		closeErr := file.Close()
		if encodeErr != nil || closeErr != nil {
			_ = os.Remove(path)
			if encodeErr != nil {
				return fmt.Errorf("encode %s picture: %w", name, encodeErr)
			}
			return fmt.Errorf("close %s picture: %w", name, closeErr)
		}
		if err := os.Rename(path, filepath.Join(outputDir, name+".pic.png")); err != nil {
			_ = os.Remove(path)
			return fmt.Errorf("publish %s picture: %w", name, err)
		}
	}
	return nil
}
