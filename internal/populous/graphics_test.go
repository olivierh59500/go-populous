package populous

import "testing"

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
