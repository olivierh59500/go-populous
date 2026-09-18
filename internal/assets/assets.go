package assets

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"

	embeddedassets "go-populous/assets"
	"go-populous/internal/populous"
)

type Bundle struct {
	AmigaDir     string
	ExtractedDir string
	Screens      map[string]*image.RGBA
	Lands        []*image.RGBA
	Sprites      *image.RGBA
	BigSprites   *image.RGBA
	Mouths       *image.RGBA
	TerrainRules []populous.TerrainRules
	Levels       []populous.Level
	SoundBank    *populous.SoundBank
	Warnings     []string

	amigaFS     fs.FS
	extractedFS fs.FS
}

// Load reads assets from the local data directories used by desktop
// development. If no local Amiga directory can be found, it falls back to the
// data embedded in the executable so a desktop build also works independently
// of its current working directory.
func Load() (*Bundle, error) {
	amigaDir := resolveDir("POPULOUS_AMIGA_DIR", "assets/amiga", "populous-amiga")
	if amigaDir == "" {
		return LoadEmbedded()
	}
	extractedDir := resolveDir("POPULOUS_EXTRACTED_IMAGE_DIR", "assets/extracted-images", "populous-amiga-disass-main/docs/images")

	var extractedFS fs.FS
	if extractedDir != "" {
		extractedFS = os.DirFS(extractedDir)
	}
	return loadBundle(os.DirFS(amigaDir), extractedFS, amigaDir, extractedDir)
}

// LoadEmbedded reads the assets compiled into the executable. It is the
// loader intended for Android and other platforms where no repository working
// directory exists.
func LoadEmbedded() (*Bundle, error) {
	return LoadFS(embeddedassets.Files)
}

// LoadFS reads a bundle from an fs.FS whose canonical data is rooted at
// "amiga". An optional "extracted-images" directory supplies PNG fallbacks
// for screen dumps that contain additional non-planar data. The embedded
// source includes only the two fallbacks needed by the checked-in Amiga data.
func LoadFS(files fs.FS) (*Bundle, error) {
	if files == nil {
		return nil, fmt.Errorf("nil asset filesystem")
	}
	info, err := fs.Stat(files, "amiga")
	if err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return nil, fmt.Errorf("missing Amiga data directory in asset filesystem: %w", err)
	}
	amigaFS, err := fs.Sub(files, "amiga")
	if err != nil {
		return nil, fmt.Errorf("missing Amiga data directory in asset filesystem: %w", err)
	}
	extractedFS, err := fs.Sub(files, "extracted-images")
	if err != nil {
		extractedFS = nil
	}
	return loadBundle(amigaFS, extractedFS, "", "")
}

func loadBundle(amigaFS, extractedFS fs.FS, amigaDir, extractedDir string) (*Bundle, error) {
	if amigaFS == nil {
		return nil, fmt.Errorf("nil Amiga asset filesystem")
	}
	b := &Bundle{
		AmigaDir:     amigaDir,
		ExtractedDir: extractedDir,
		Screens:      map[string]*image.RGBA{},
		amigaFS:      amigaFS,
		extractedFS:  extractedFS,
	}

	for _, name := range []string{"qaz", "demo", "lord", "load"} {
		img, err := b.loadScreen(name)
		if err != nil {
			b.Warnings = append(b.Warnings, err.Error())
			continue
		}
		b.Screens[name] = img
	}

	if err := b.loadPlanarAssets(); err != nil {
		b.Warnings = append(b.Warnings, err.Error())
	}
	if err := b.loadMouths(); err != nil {
		b.Warnings = append(b.Warnings, err.Error())
	}
	if err := b.loadLevels(); err != nil {
		b.Warnings = append(b.Warnings, err.Error())
	}
	if err := b.loadSoundBank(); err != nil {
		b.Warnings = append(b.Warnings, err.Error())
	}
	return b, nil
}

func (b *Bundle) loadScreen(name string) (*image.RGBA, error) {
	data, err := fs.ReadFile(b.amigaFS, name+".pic")
	if err != nil {
		return nil, fmt.Errorf("screen %s: %w", name, err)
	}
	if len(data) == populous.ScreenWidth*populous.ScreenHeight/2 {
		return populous.DecodeAmigaScreen4BPP(data, populous.ScreenWidth, populous.ScreenHeight, 0)
	}
	if b.extractedFS != nil {
		if img, err := loadPNG(b.extractedFS, name+".pic.png"); err == nil {
			return img, nil
		}
	}
	return populous.DecodeAmigaScreen4BPP(data, populous.ScreenWidth, populous.ScreenHeight, 0)
}

func (b *Bundle) loadPlanarAssets() error {
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("land%d", i)
		land, err := fs.ReadFile(b.amigaFS, name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(land) < populous.LandHeaderSize {
			return fmt.Errorf("%s: file too short", name)
		}
		rules, err := populous.DecodeTerrainRules(land[:populous.LandHeaderSize])
		if err != nil {
			return fmt.Errorf("%s rules: %w", name, err)
		}
		b.TerrainRules = append(b.TerrainRules, rules)
		img, err := populous.DecodePlanarMasked(land[populous.LandHeaderSize:], populous.BlockWidth, true, 0)
		if err != nil {
			return fmt.Errorf("%s blocks: %w", name, err)
		}
		b.Lands = append(b.Lands, img)
	}

	sprites, err := fs.ReadFile(b.amigaFS, "sprites0.dat")
	if err != nil {
		return fmt.Errorf("sprites0.dat: %w", err)
	}
	b.Sprites, err = populous.DecodePlanarMasked(sprites, 16, true, 0)
	if err != nil {
		return fmt.Errorf("sprites0.dat: %w", err)
	}

	bigSprites, err := fs.ReadFile(b.amigaFS, "spr_320.dat")
	if err != nil {
		return fmt.Errorf("spr_320.dat: %w", err)
	}
	b.BigSprites, err = populous.DecodePlanarMasked(bigSprites, populous.BigSpriteWidth, true, 0)
	if err != nil {
		return fmt.Errorf("spr_320.dat: %w", err)
	}

	return nil
}

func (b *Bundle) loadMouths() error {
	data, err := fs.ReadFile(b.amigaFS, "mouths.pic")
	if err != nil {
		return fmt.Errorf("mouths.pic: %w", err)
	}
	img, err := populous.DecodeMouths(data, 1)
	if err != nil {
		return fmt.Errorf("mouths.pic: %w", err)
	}
	b.Mouths = img
	return nil
}

func (b *Bundle) loadLevels() error {
	f, err := b.amigaFS.Open("level.dat")
	if err != nil {
		return fmt.Errorf("level.dat: %w", err)
	}
	defer f.Close()

	levels, err := populous.LoadLevels(f)
	if err != nil {
		return fmt.Errorf("level.dat: %w", err)
	}
	b.Levels = levels
	return nil
}

func (b *Bundle) loadSoundBank() error {
	data, err := fs.ReadFile(b.amigaFS, "gmusic1")
	if err != nil {
		return fmt.Errorf("gmusic1: %w", err)
	}
	bank, err := populous.DecodeAmigaSoundBank(data)
	if err != nil {
		return fmt.Errorf("gmusic1: %w", err)
	}

	wordData, err := fs.ReadFile(b.amigaFS, "gwords")
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("gwords: %v", err))
		b.SoundBank = bank
		return nil
	}
	wordBank, err := populous.DecodeAmigaSoundBank(wordData)
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("gwords: %v", err))
		b.SoundBank = bank
		return nil
	}
	if err := bank.InsertBankAt(wordBank, populous.WordSoundBase); err != nil {
		return fmt.Errorf("gwords: %w", err)
	}
	b.SoundBank = bank
	return nil
}

func loadPNG(files fs.FS, name string) (*image.RGBA, error) {
	f, err := files.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	rgba := image.NewRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba, nil
}

func resolveDir(envKey string, relatives ...string) string {
	if fromEnv := os.Getenv(envKey); fromEnv != "" {
		if st, err := os.Stat(fromEnv); err == nil && st.IsDir() {
			return fromEnv
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		for _, relative := range relatives {
			candidate := filepath.Join(dir, relative)
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}
