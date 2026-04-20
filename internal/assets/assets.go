package assets

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"go-populous/internal/populous"
)

type Bundle struct {
	AmigaDir     string
	ExtractedDir string
	Screens      map[string]*image.RGBA
	Lands        []*image.RGBA
	Blocks       *image.RGBA
	Sprites      *image.RGBA
	BigSprites   *image.RGBA
	Font         *image.RGBA
	Mouths       *image.RGBA
	TerrainRules []populous.TerrainRules
	Levels       []populous.Level
	SoundBank    *populous.SoundBank
	Warnings     []string
}

func Load() (*Bundle, error) {
	b := &Bundle{
		AmigaDir:     resolveDir("POPULOUS_AMIGA_DIR", "assets/amiga", "populous-amiga"),
		ExtractedDir: resolveDir("POPULOUS_EXTRACTED_IMAGE_DIR", "assets/extracted-images", "populous-amiga-disass-main/docs/images"),
		Screens:      map[string]*image.RGBA{},
	}
	if b.AmigaDir == "" {
		return nil, fmt.Errorf("missing Amiga data directory; expected assets/amiga or set POPULOUS_AMIGA_DIR")
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
	path := filepath.Join(b.AmigaDir, name+".pic")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("screen %s: %w", name, err)
	}
	if len(data) == populous.ScreenWidth*populous.ScreenHeight/2 {
		return populous.DecodeAmigaScreen4BPP(data, populous.ScreenWidth, populous.ScreenHeight, 0)
	}
	if b.ExtractedDir != "" {
		if img, err := loadPNG(filepath.Join(b.ExtractedDir, name+".pic.png")); err == nil {
			return img, nil
		}
	}
	return populous.DecodeAmigaScreen4BPP(data, populous.ScreenWidth, populous.ScreenHeight, 0)
}

func (b *Bundle) loadPlanarAssets() error {
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("land%d", i)
		land, err := os.ReadFile(filepath.Join(b.AmigaDir, name))
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
		if i == 0 {
			b.Blocks = img
		}
	}

	sprites, err := os.ReadFile(filepath.Join(b.AmigaDir, "sprites0.dat"))
	if err != nil {
		return fmt.Errorf("sprites0.dat: %w", err)
	}
	b.Sprites, err = populous.DecodePlanarMasked(sprites, 16, true, 0)
	if err != nil {
		return fmt.Errorf("sprites0.dat: %w", err)
	}

	bigSprites, err := os.ReadFile(filepath.Join(b.AmigaDir, "spr_320.dat"))
	if err != nil {
		return fmt.Errorf("spr_320.dat: %w", err)
	}
	b.BigSprites, err = populous.DecodePlanarMasked(bigSprites, populous.BigSpriteWidth, true, 0)
	if err != nil {
		return fmt.Errorf("spr_320.dat: %w", err)
	}

	font, err := os.ReadFile(filepath.Join(b.AmigaDir, "font.dat"))
	if err != nil {
		return fmt.Errorf("font.dat: %w", err)
	}
	b.Font, err = populous.DecodePlanarMasked(font, 8, true, 0)
	if err != nil {
		return fmt.Errorf("font.dat: %w", err)
	}
	return nil
}

func (b *Bundle) loadMouths() error {
	data, err := os.ReadFile(filepath.Join(b.AmigaDir, "mouths.pic"))
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
	f, err := os.Open(filepath.Join(b.AmigaDir, "level.dat"))
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
	data, err := os.ReadFile(filepath.Join(b.AmigaDir, "gmusic1"))
	if err != nil {
		return fmt.Errorf("gmusic1: %w", err)
	}
	bank, err := populous.DecodeAmigaSoundBank(data)
	if err != nil {
		return fmt.Errorf("gmusic1: %w", err)
	}

	wordData, err := os.ReadFile(filepath.Join(b.AmigaDir, "gwords"))
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

func loadPNG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
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
