package assets

import (
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go-populous/internal/populous"
)

func TestLoadWithLocalDump(t *testing.T) {
	bundle, err := Load()
	if err != nil {
		t.Skip(err)
	}
	assertCompleteBundle(t, bundle)
}

func TestLoadEmbedded(t *testing.T) {
	bundle, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	assertCompleteBundle(t, bundle)
	if bundle.AmigaDir != "" || bundle.ExtractedDir != "" {
		t.Fatalf("embedded bundle exposes host directories: AmigaDir=%q ExtractedDir=%q", bundle.AmigaDir, bundle.ExtractedDir)
	}
	if len(bundle.Warnings) != 0 {
		t.Fatalf("embedded bundle warnings = %q", bundle.Warnings)
	}
}

func TestLoadFallsBackToEmbeddedOutsideRepository(t *testing.T) {
	t.Setenv("POPULOUS_AMIGA_DIR", "")
	t.Setenv("POPULOUS_EXTRACTED_IMAGE_DIR", "")
	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWorkingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	bundle, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	assertCompleteBundle(t, bundle)
}

func TestLoadFSRejectsMissingAmigaDirectory(t *testing.T) {
	if _, err := LoadFS(nil); err == nil {
		t.Fatal("LoadFS(nil) succeeded")
	}
	if _, err := LoadFS(emptyFS{}); err == nil {
		t.Fatal("LoadFS accepted a filesystem without amiga")
	}
}

type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func assertCompleteBundle(t *testing.T, bundle *Bundle) {
	t.Helper()
	if len(bundle.Levels) != 495 {
		t.Fatalf("len(Levels) = %d, want 495", len(bundle.Levels))
	}
	if bundle.Screens["qaz"] == nil {
		t.Fatal("qaz screen was not loaded")
	}
	if len(bundle.Lands) != 4 {
		t.Fatalf("len(Lands) = %d, want 4", len(bundle.Lands))
	}
	if len(bundle.TerrainRules) != 4 {
		t.Fatalf("len(TerrainRules) = %d, want 4", len(bundle.TerrainRules))
	}
	if bundle.TerrainRules[0].WalkDeath != 1 || bundle.TerrainRules[3].WalkDeath != 4 {
		t.Fatalf("unexpected walk death values: land0=%d land3=%d", bundle.TerrainRules[0].WalkDeath, bundle.TerrainRules[3].WalkDeath)
	}
	for i, land := range bundle.Lands {
		if land == nil {
			t.Fatalf("land %d was not loaded", i)
		}
		if got := land.Bounds().Dy() / populous.BlockHeight; got != populous.BlocksPerLand {
			t.Fatalf("land %d block count = %d, want 70", i, got)
		}
		assertPremultipliedTransparent(t, land)
	}
	if bundle.Sprites == nil {
		t.Fatal("sprites0.dat was not loaded")
	}
	if got := bundle.Sprites.Bounds().Dy() / populous.SpriteHeight; got != 147 {
		t.Fatalf("sprite count = %d, want 147", got)
	}
	if bundle.BigSprites == nil {
		t.Fatal("spr_320.dat was not loaded")
	}
	if got := bundle.BigSprites.Bounds().Dy() / populous.BigSpriteHeight; got != 26 {
		t.Fatalf("big sprite count = %d, want 26", got)
	}
	if bundle.SoundBank == nil || !bundle.SoundBank.HasSound(populous.WordSoundBase) {
		t.Fatal("gwords voice samples were not merged into the sound bank")
	}
	if bundle.Mouths == nil {
		t.Fatal("mouths.pic was not loaded")
	}
	if got := bundle.Mouths.Bounds().Dx() / populous.MouthWidth; got != populous.MouthFrames {
		t.Fatalf("mouth frame count = %d, want %d", got, populous.MouthFrames)
	}
	if got := bundle.Mouths.Bounds().Dy(); got != populous.MouthHeight {
		t.Fatalf("mouth height = %d, want %d", got, populous.MouthHeight)
	}
}

func assertPremultipliedTransparent(t *testing.T, rgba *image.RGBA) {
	t.Helper()
	for y := rgba.Bounds().Min.Y; y < rgba.Bounds().Max.Y; y++ {
		for x := rgba.Bounds().Min.X; x < rgba.Bounds().Max.X; x++ {
			offset := rgba.PixOffset(x, y)
			if rgba.Pix[offset+3] == 0 && (rgba.Pix[offset] != 0 || rgba.Pix[offset+1] != 0 || rgba.Pix[offset+2] != 0) {
				t.Fatalf("transparent pixel at %d,%d has non-zero RGB", x, y)
			}
		}
	}
}

func TestAmigaScreenDecodeMatchesExtractedQaz(t *testing.T) {
	bundle, err := Load()
	if err != nil {
		t.Skip(err)
	}
	refPath := filepath.Join(bundle.ExtractedDir, "qaz.pic.png")
	ref, err := loadPNG(os.DirFS(filepath.Dir(refPath)), filepath.Base(refPath))
	if err != nil {
		t.Skip(err)
	}
	got := bundle.Screens["qaz"]
	if got == nil {
		t.Fatal("qaz screen was not loaded")
	}
	if got.Bounds() != ref.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", got.Bounds(), ref.Bounds())
	}

	var diff int64
	for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
		for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
			a := got.RGBAAt(x, y)
			b := ref.RGBAAt(x, y)
			diff += abs(int(a.R) - int(b.R))
			diff += abs(int(a.G) - int(b.G))
			diff += abs(int(a.B) - int(b.B))
		}
	}
	if diff != 0 {
		t.Fatalf("decoded qaz differs from extracted PNG by %d total RGB units", diff)
	}
}

func TestEmbeddedScreensMatchDesktopScreens(t *testing.T) {
	desktop, err := Load()
	if err != nil || desktop.ExtractedDir == "" {
		t.Skipf("desktop reference images unavailable: %v", err)
	}
	embedded, err := LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"qaz", "demo", "lord", "load"} {
		if !reflect.DeepEqual(embedded.Screens[name], desktop.Screens[name]) {
			t.Errorf("embedded screen %q differs from desktop screen", name)
		}
	}
}

func abs(v int) int64 {
	if v < 0 {
		return int64(-v)
	}
	return int64(v)
}
