package mobileui

import (
	"image"
	"math"
	"testing"
)

func TestViewportMatchesOriginalTileProjection(t *testing.T) {
	// Original block origin is (176,64), and its diamond centre is (192,72).
	view := Viewport{Rect: image.Rect(32, 8, 352, 248), CenterX: 3.5, CenterY: 3.5, Zoom: 1}
	for y := range 8 {
		for x := range 8 {
			for height := range 9 {
				sx, sy := view.Project(float64(x), float64(y), float64(height))
				if wantX, wantY := float64(192+16*(x-y)), float64(72+8*(x+y)-8*height); sx != wantX || sy != wantY {
					t.Fatalf("(%d,%d,%d): got %g,%g; want %g,%g", x, y, height, sx, sy, wantX, wantY)
				}
			}
		}
	}
}

func TestViewportProjectionDoesNotWrapPastDesktopWidth(t *testing.T) {
	view := Viewport{Rect: image.Rect(0, 24, 539, 194), CenterX: 32, CenterY: 32, Zoom: 1}
	for _, point := range [...][3]float64{{0, 0, 0}, {63, 0, 8}, {0, 63, 4}, {63, 63, 7}, {40.5, 27.25, 2}} {
		x, y := view.Project(point[0], point[1], point[2])
		mx, my := view.Unproject(x, y, point[2])
		if math.Abs(mx-point[0])+math.Abs(my-point[1]) > 1e-9 {
			t.Fatalf("round trip %v returned %g,%g", point, mx, my)
		}
	}
	x0, y0 := view.Project(32, 32, 0)
	x1, y1 := view.Project(52, 32, 0)
	if x1-x0 != 320 || y1-y0 != 160 {
		t.Fatalf("wide projection wrapped: %g,%g", x1-x0, y1-y0)
	}
}

func TestViewportPanFollowsFingersAndSurvivesResize(t *testing.T) {
	view := Viewport{Rect: image.Rect(10, 26, 529, 190), CenterX: 23, CenterY: 31, Zoom: 1.5}
	x, y := view.Project(22, 33, 3)
	panned := view.Pan(41, -17)
	x1, y1 := panned.Project(22, 33, 3)
	if math.Abs(x1-x-41)+math.Abs(y1-y+17) > 1e-9 {
		t.Fatalf("drag displacement = %g,%g", x1-x, y1-y)
	}
	panned.Rect = image.Rect(10, 26, 630, 190)
	mx, my := panned.Unproject(320, 108, 0)
	if mx != panned.CenterX || my != panned.CenterY {
		t.Fatalf("resize changed camera centre: %g,%g", mx, my)
	}
	clamped := view.Pan(100000, -100000).Clamp(64, 64)
	if clamped.CenterX < 0 || clamped.CenterX > 63 || clamped.CenterY < 0 || clamped.CenterY > 63 {
		t.Fatalf("out of bounds camera: %+v", clamped)
	}
}

func TestViewportBoundsIncludeVisibleHighGroundAndEdges(t *testing.T) {
	for _, center := range [...][2]float64{{0, 0}, {32, 32}, {63, 63}, {0, 63}} {
		view := Viewport{Rect: image.Rect(10, 26, 529, 190), CenterX: center[0], CenterY: center[1], Zoom: 1}
		bounds := view.Bounds(64, 64, 8)
		if !bounds.In(image.Rect(0, 0, 64, 64)) {
			t.Fatalf("invalid map bounds %v", bounds)
		}
		for y := range 64 {
			for x := range 64 {
				for h := range 9 {
					px, py := view.Project(float64(x), float64(y), float64(h))
					if image.Pt(int(px), int(py)).In(view.Rect) && !image.Pt(x, y).In(bounds) {
						t.Fatalf("visible tile %d,%d,%d missing from %v", x, y, h, bounds)
					}
				}
			}
		}
	}
}

func TestViewportPicksAltitudeAndFrontmostOverlappingTile(t *testing.T) {
	view := Viewport{Rect: image.Rect(0, 24, 539, 194), CenterX: 32, CenterY: 32, Zoom: 1}
	altitude := func(x, y int) float64 {
		if x < 0 || x >= 64 || y < 0 || y >= 64 {
			t.Fatalf("invalid callback coordinate %d,%d", x, y)
		}
		if x == 33 && y == 33 {
			return 2 // This foreground tile covers the centre of (32,32).
		}
		return 0
	}
	px, py := view.Project(32, 32, 0)
	x, y, ok := view.PickTile(px, py, 64, 64, altitude)
	if !ok || x != 33 || y != 33 {
		t.Fatalf("overlapping tiles: picked %d,%d,%v", x, y, ok)
	}
	flatHeight := func(_, _ int) float64 { return 4 }
	for _, tile := range [...][2]int{{27, 31}, {39, 31}, {33, 37}} {
		px, py := view.Project(float64(tile[0]), float64(tile[1]), 4)
		x, y, ok := view.PickTile(px, py, 64, 64, flatHeight)
		if !ok || x != tile[0] || y != tile[1] {
			t.Fatalf("height 4, %v: picked %d,%d,%v", tile, x, y, ok)
		}
	}
	if _, _, ok := view.PickTile(30, 2, 64, 64, altitude); ok {
		t.Fatal("toolbar touch picked terrain")
	}
}

func TestViewportClassicLimitConstrainsRenderingAndPicking(t *testing.T) {
	view := Viewport{Rect: image.Rect(0, 24, 539, 194), CenterX: 32, CenterY: 32, Zoom: 1, Limit: image.Rect(28, 28, 36, 36)}
	if got := view.Bounds(64, 64, 8); got != view.Limit {
		t.Fatalf("classic rendering bounds %v; want %v", got, view.Limit)
	}
	x, y := view.Project(36, 32, 0)
	if _, _, ok := view.PickTile(x, y, 64, 64, func(_, _ int) float64 { return 0 }); ok {
		t.Fatal("tile beyond classic viewport could be selected")
	}
}
