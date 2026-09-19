package mobileui

import (
	"image"
	"math"
)

// Viewport projects the map independently of the original 8-by-8 desktop view.
// Coordinates are in logical screen pixels. Project returns a tile's centre,
// with a diamond half-width of 16 and half-height of 8 at Zoom 1.
type Viewport struct {
	Rect             image.Rectangle
	CenterX, CenterY float64
	Zoom             float64
	// Limit optionally restricts rendering and picking to an exclusive map
	// rectangle, for the original 8-by-8 view. Empty means the whole world.
	Limit image.Rectangle
}

func (v Viewport) scale() float64 {
	if v.Zoom <= 0 || math.IsNaN(v.Zoom) || math.IsInf(v.Zoom, 0) {
		return 1
	}
	return v.Zoom
}

func (v Viewport) Project(mapX, mapY, height float64) (screenX, screenY float64) {
	x, y := mapX-v.CenterX, mapY-v.CenterY
	z := v.scale()
	return float64(v.Rect.Min.X+v.Rect.Max.X)/2 + 16*(x-y)*z,
		float64(v.Rect.Min.Y+v.Rect.Max.Y)/2 + (8*(x+y)-8*height)*z
}

// Unproject intersects a screen point with a plane at the specified altitude.
func (v Viewport) Unproject(screenX, screenY, height float64) (mapX, mapY float64) {
	z := v.scale()
	x := (screenX - float64(v.Rect.Min.X+v.Rect.Max.X)/2) / (16 * z)
	y := (screenY-float64(v.Rect.Min.Y+v.Rect.Max.Y)/2)/(8*z) + height
	return v.CenterX + (x+y)/2, v.CenterY + (y-x)/2
}

// Pan makes the terrain follow a finger's displacement, rather than reversing
// the drag as a directional pad would. Call Clamp afterwards at map edges.
func (v Viewport) Pan(screenDX, screenDY float64) Viewport {
	z := v.scale()
	v.CenterX -= screenDX/(32*z) + screenDY/(16*z)
	v.CenterY -= screenDY/(16*z) - screenDX/(32*z)
	return v
}

func (v Viewport) Clamp(mapWidth, mapHeight int) Viewport {
	v.CenterX = max(0, min(float64(max(0, mapWidth-1)), v.CenterX))
	v.CenterY = max(0, min(float64(max(0, mapHeight-1)), v.CenterY))
	return v
}

// Bounds returns exclusive map bounds, including high ground whose sea-level
// projection is below the screen and sprites extending beyond their tile.
func (v Viewport) Bounds(mapWidth, mapHeight, maxAltitude int) image.Rectangle {
	if v.Rect.Empty() || mapWidth <= 0 || mapHeight <= 0 {
		return image.Rectangle{}
	}
	padding := 32 * v.scale()
	left, right := float64(v.Rect.Min.X)-padding, float64(v.Rect.Max.X)+padding
	top, bottom := float64(v.Rect.Min.Y)-padding, float64(v.Rect.Max.Y)+padding
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, height := range [...]float64{0, float64(max(0, maxAltitude))} {
		for _, point := range [...][2]float64{{left, top}, {right, top}, {left, bottom}, {right, bottom}} {
			x, y := v.Unproject(point[0], point[1], height)
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}
	bounds := image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1).
		Intersect(image.Rect(0, 0, mapWidth, mapHeight))
	if !v.Limit.Empty() {
		bounds = bounds.Intersect(v.Limit)
	}
	return bounds
}

// PickTile selects the frontmost diamond at a screen position using the same
// back-to-front diagonal order as the terrain renderer. Populous caps terrain
// at altitude 8. The callback is only called with valid map coordinates.
func (v Viewport) PickTile(screenX, screenY float64, mapWidth, mapHeight int, altitude func(x, y int) float64) (mapX, mapY int, ok bool) {
	if screenX < float64(v.Rect.Min.X) || screenX >= float64(v.Rect.Max.X) ||
		screenY < float64(v.Rect.Min.Y) || screenY >= float64(v.Rect.Max.Y) || altitude == nil {
		return 0, 0, false
	}
	bounds := v.Bounds(mapWidth, mapHeight, 8)
	z := v.scale()
	for diagonal := bounds.Max.X + bounds.Max.Y - 2; diagonal >= bounds.Min.X+bounds.Min.Y; diagonal-- {
		for x := min(bounds.Max.X-1, diagonal-bounds.Min.Y); x >= max(bounds.Min.X, diagonal-bounds.Max.Y+1); x-- {
			y := diagonal - x
			px, py := v.Project(float64(x), float64(y), altitude(x, y))
			if math.Abs(screenX-px)/(16*z)+math.Abs(screenY-py)/(8*z) <= 1 {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}
