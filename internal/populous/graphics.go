package populous

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

const (
	ScreenWidth     = 320
	ScreenHeight    = 200
	UIStripe        = 40
	MiniMapWidth    = 128
	MiniMapHeight   = 64
	MouthWidth      = 48
	MouthHeight     = 35
	MouthFrames     = 6
	BlockWidth      = 32
	BlockHeight     = 24
	SpriteWidth     = 16
	SpriteHeight    = 16
	BigSpriteWidth  = 16
	BigSpriteHeight = 32
)

type CursorMode uint8

const (
	CursorDefault CursorMode = iota
	CursorMagnet
	CursorSwamp
)

// CursorSprite returns the original placement pointer graphic. PUT_MAGNET used
// pointer player+2 (Ankh for good, skull for evil), while PUT_SWAMP used 5.
func CursorSprite(mode CursorMode, player int) int {
	switch mode {
	case CursorMagnet:
		switch player {
		case GodPlayer:
			return AnkhSprite
		case DevilPlayer:
			return SkullSprite
		}
	case CursorSwamp:
		return SwampHandSprite
	}
	return CrosshairSprite
}

// MiniMapPixelOffset projects one map tile into the original 128x64 minimap
// pixel buffer and returns its RGBA byte offset.
func MiniMapPixelOffset(mapX, mapY int) (int, bool) {
	if mapX < 0 || mapX >= MapWidth || mapY < 0 || mapY >= MapHeight {
		return 0, false
	}
	screenX := 64 + mapX - mapY
	screenY := (mapX + mapY) >> 1
	if screenX < 0 || screenX >= MiniMapWidth || screenY < 0 || screenY >= MiniMapHeight {
		return 0, false
	}
	return (screenY*MiniMapWidth + screenX) * 4, true
}

// MiniMapViewportCrosshairPosition returns the original screen coordinates for
// the crosshair marking the centre of the 8x8 world viewport on the minimap.
func MiniMapViewportCrosshairPosition(xoff, yoff int) (int, int) {
	centerX := xoff + 3
	centerY := yoff + 3
	return 64 + centerX - centerY - 3, (centerX+centerY)/2 - 3
}

func DecodeChunkyScreen4BPP(data []byte, width, height int, paletteVariant int) (*image.RGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid screen dimensions %dx%d", width, height)
	}
	need := width * height / 2
	if len(data) < need {
		return nil, fmt.Errorf("screen data too short: got %d, need %d", len(data), need)
	}
	if len(data) > need {
		data = data[len(data)-need:]
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	pixel := 0
	for _, b := range data[:need] {
		for _, index := range [...]byte{b & 0x0f, b >> 4} {
			writeRGBAPixel(img, pixel*4, Palette(int(index), paletteVariant))
			pixel++
		}
	}
	return img, nil
}

func DecodeMouths(data []byte, paletteVariant int) (*image.RGBA, error) {
	frameSize := (MouthWidth / 8) * 4 * MouthHeight
	need := frameSize * MouthFrames
	if len(data) < need {
		return nil, fmt.Errorf("mouth data too short: got %d, need %d", len(data), need)
	}
	img := image.NewRGBA(image.Rect(0, 0, MouthWidth*MouthFrames, MouthHeight))
	for frame := 0; frame < MouthFrames; frame++ {
		start := frame * frameSize
		decoded, err := DecodePlanarMasked(data[start:start+frameSize], MouthWidth, false, paletteVariant)
		if err != nil {
			return nil, fmt.Errorf("mouth frame %d: %w", frame, err)
		}
		dst := image.Rect(frame*MouthWidth, 0, (frame+1)*MouthWidth, MouthHeight)
		draw.Draw(img, dst, decoded, image.Point{}, draw.Src)
	}
	return img, nil
}

func DecodeAmigaScreen4BPP(data []byte, width, height int, paletteVariant int) (*image.RGBA, error) {
	if width <= 0 || height <= 0 || width%8 != 0 {
		return nil, fmt.Errorf("invalid Amiga screen dimensions %dx%d", width, height)
	}
	rowBytes := width / 8
	need := rowBytes * height * 4
	if len(data) < need {
		return nil, fmt.Errorf("Amiga screen data too short: got %d, need %d", len(data), need)
	}
	if len(data) > need {
		data = data[len(data)-need:]
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for xByte := 0; xByte < rowBytes; xByte++ {
			planes := [4]byte{
				data[y*rowBytes+xByte],
				data[rowBytes*height+y*rowBytes+xByte],
				data[rowBytes*height*2+y*rowBytes+xByte],
				data[rowBytes*height*3+y*rowBytes+xByte],
			}
			for bit := 7; bit >= 0; bit-- {
				index := int((planes[0] >> bit) & 1)
				index |= int(((planes[1] >> bit) & 1) << 1)
				index |= int(((planes[2] >> bit) & 1) << 2)
				index |= int(((planes[3] >> bit) & 1) << 3)
				x := xByte*8 + (7 - bit)
				writeRGBAPixel(img, y*img.Stride+x*4, Palette(index, paletteVariant))
			}
		}
	}
	return img, nil
}

func DecodePlanarMasked(data []byte, width int, hasMask bool, paletteVariant int) (*image.RGBA, error) {
	if width <= 0 || width > 64 || width%8 != 0 {
		return nil, fmt.Errorf("unsupported planar width %d", width)
	}
	stripSize := width / 8
	stripWidth := 4
	if hasMask {
		stripWidth = 5
	}
	chunkSize := stripSize * stripWidth
	if chunkSize == 0 || len(data) < chunkSize {
		return nil, errors.New("planar data too short")
	}

	height := len(data) / chunkSize
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	offset := 0
	for y := 0; y < height; y++ {
		var mask uint64
		if hasMask {
			for i := 0; i < stripSize; i++ {
				mask = (mask << 8) | uint64(data[offset])
				offset++
			}
		}

		var planes [4]uint64
		for plane := 0; plane < 4; plane++ {
			for i := 0; i < stripSize; i++ {
				planes[plane] = (planes[plane] << 8) | uint64(data[offset])
				offset++
			}
		}

		for x := 0; x < width; x++ {
			bit := width - 1 - x
			if hasMask && ((mask>>bit)&1) != 0 {
				continue
			}
			index := int(((planes[3] >> bit) & 1) << 3)
			index |= int(((planes[2] >> bit) & 1) << 2)
			index |= int(((planes[1] >> bit) & 1) << 1)
			index |= int((planes[0] >> bit) & 1)
			writeRGBAPixel(img, y*img.Stride+x*4, Palette(index, paletteVariant))
		}
	}
	return img, nil
}

func writeRGBAPixel(img *image.RGBA, offset int, c color.RGBA) {
	img.Pix[offset+0] = c.R
	img.Pix[offset+1] = c.G
	img.Pix[offset+2] = c.B
	img.Pix[offset+3] = c.A
}
