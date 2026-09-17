package populous

import (
	"image"
	"testing"
)

func TestCursorSpriteMatchesOriginalPlacementPointers(t *testing.T) {
	tests := []struct {
		name   string
		mode   CursorMode
		player int
		want   int
	}{
		{name: "default", mode: CursorDefault, player: GodPlayer, want: CrosshairSprite},
		{name: "good magnet", mode: CursorMagnet, player: GodPlayer, want: AnkhSprite},
		{name: "evil magnet", mode: CursorMagnet, player: DevilPlayer, want: SkullSprite},
		{name: "swamp", mode: CursorSwamp, player: DevilPlayer, want: SwampHandSprite},
		{name: "invalid magnet player", mode: CursorMagnet, player: 2, want: CrosshairSprite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CursorSprite(tt.mode, tt.player); got != tt.want {
				t.Fatalf("CursorSprite(%d, %d) = %d, want %d", tt.mode, tt.player, got, tt.want)
			}
		})
	}
}

func TestMiniMapPixelOffset(t *testing.T) {
	tests := []struct {
		name       string
		mapX, mapY int
		want       int
		ok         bool
	}{
		{name: "north corner", mapX: 0, mapY: 0, want: 64 * 4, ok: true},
		{name: "east corner", mapX: MapWidth - 1, mapY: 0, want: (31*MiniMapWidth + 127) * 4, ok: true},
		{name: "south corner", mapX: MapWidth - 1, mapY: MapHeight - 1, want: (63*MiniMapWidth + 64) * 4, ok: true},
		{name: "outside", mapX: MapWidth, mapY: 0, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := MiniMapPixelOffset(tt.mapX, tt.mapY)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("MiniMapPixelOffset(%d, %d) = (%d, %t), want (%d, %t)", tt.mapX, tt.mapY, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestMiniMapViewportCrosshairPositionMatchesOriginalProjection(t *testing.T) {
	tests := []struct {
		xoff, yoff   int
		wantX, wantY int
	}{
		{xoff: 0, yoff: 0, wantX: 61, wantY: 0},
		{xoff: 10, yoff: 20, wantX: 51, wantY: 15},
		{xoff: MapWidth - 8, yoff: MapHeight - 8, wantX: 61, wantY: 56},
	}
	for _, tt := range tests {
		gotX, gotY := MiniMapViewportCrosshairPosition(tt.xoff, tt.yoff)
		if gotX != tt.wantX || gotY != tt.wantY {
			t.Errorf("position(%d, %d) = (%d, %d), want (%d, %d)", tt.xoff, tt.yoff, gotX, gotY, tt.wantX, tt.wantY)
		}
	}
}

func TestDecodeChunkyScreen4BPPNibbleOrder(t *testing.T) {
	img, err := DecodeChunkyScreen4BPP([]byte{0x21}, 2, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(0, 0); got != Palette(1, 0) {
		t.Fatalf("first pixel = %#v, want palette 1", got)
	}
	if got := img.RGBAAt(1, 0); got != Palette(2, 0) {
		t.Fatalf("second pixel = %#v, want palette 2", got)
	}
}

func TestDecodeAmigaScreen4BPP(t *testing.T) {
	data := []byte{
		0b01000000,
		0b00100000,
		0b00010000,
		0b00001000,
	}
	img, err := DecodeAmigaScreen4BPP(data, 8, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(1, 0); got != Palette(1, 0) {
		t.Fatalf("plane 0 pixel = %#v, want palette 1", got)
	}
	if got := img.RGBAAt(2, 0); got != Palette(2, 0) {
		t.Fatalf("plane 1 pixel = %#v, want palette 2", got)
	}
	if got := img.RGBAAt(3, 0); got != Palette(4, 0) {
		t.Fatalf("plane 2 pixel = %#v, want palette 4", got)
	}
	if got := img.RGBAAt(4, 0); got != Palette(8, 0) {
		t.Fatalf("plane 3 pixel = %#v, want palette 8", got)
	}
}

func TestDecodePlanarMasked(t *testing.T) {
	data := []byte{
		0x80,
		0x40,
		0x00,
		0x00,
		0x00,
	}
	img, err := DecodePlanarMasked(data, 8, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if img.RGBAAt(0, 0).A != 0 {
		t.Fatalf("masked pixel alpha = %d, want 0", img.RGBAAt(0, 0).A)
	}
	if got := img.RGBAAt(0, 0); got.R != 0 || got.G != 0 || got.B != 0 {
		t.Fatalf("masked pixel RGB = %#v, want premultiplied transparent black", got)
	}
	if got := img.RGBAAt(1, 0); got != Palette(1, 0) {
		t.Fatalf("visible pixel = %#v, want palette 1", got)
	}
}

func TestDecodeMouthsPlanarFrames(t *testing.T) {
	frameSize := (MouthWidth / 8) * 4 * MouthHeight
	data := make([]byte, frameSize*MouthFrames)
	frame := 2
	data[frame*frameSize+MouthWidth/8] = 0x80

	img, err := DecodeMouths(data, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := img.Bounds().Dx(), MouthWidth*MouthFrames; got != want {
		t.Fatalf("mouth atlas width = %d, want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), MouthHeight; got != want {
		t.Fatalf("mouth atlas height = %d, want %d", got, want)
	}
	if got := img.RGBAAt(frame*MouthWidth, 0); got != Palette(2, 1) {
		t.Fatalf("decoded mouth pixel = %#v, want palette 2 from lord palette", got)
	}
}

var benchmarkDecodedImage *image.RGBA

func BenchmarkDecodeChunkyScreen4BPP(b *testing.B) {
	data := make([]byte, ScreenWidth*ScreenHeight/2)
	fillBenchmarkGraphicsData(data)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkDecodedImage, err = DecodeChunkyScreen4BPP(data, ScreenWidth, ScreenHeight, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeAmigaScreen4BPP(b *testing.B) {
	data := make([]byte, ScreenWidth*ScreenHeight/2)
	fillBenchmarkGraphicsData(data)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkDecodedImage, err = DecodeAmigaScreen4BPP(data, ScreenWidth, ScreenHeight, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodePlanarMaskedSprites(b *testing.B) {
	data := make([]byte, 23520)
	fillBenchmarkGraphicsData(data)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkDecodedImage, err = DecodePlanarMasked(data, SpriteWidth, true, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func fillBenchmarkGraphicsData(data []byte) {
	for i := range data {
		data[i] = byte(i*31 + 7)
	}
}
