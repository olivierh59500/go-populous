package mobileui

import (
	"image"
	"testing"
)

func TestHUDControlRoutesStayDisjoint(t *testing.T) {
	for _, width := range []int{320, 539, 800} {
		for _, overlay := range []Overlay{OverlayNone, OverlayPowers, OverlayPeople, OverlayMenu, OverlaySettings, OverlayConfirm} {
			layout := NewHUDLayout(width, 240, overlay, true)
			bounds := image.Rect(0, 0, width, 240)
			for i, button := range layout.Buttons {
				if !button.Rect.In(bounds) || button.Rect.Empty() {
					t.Fatalf("width %d overlay %d: button %d out of bounds: %v", width, overlay, button.Action, button.Rect)
				}
				center := button.Rect.Min.Add(button.Rect.Size().Div(2))
				if got := layout.Hit(center.X, center.Y); got != button.Action {
					t.Fatalf("width %d overlay %d: route %d got %d", width, overlay, button.Action, got)
				}
				if layout.IsTerrainPoint(center.X, center.Y) || layout.IsMiniMapPoint(center.X, center.Y) {
					t.Fatalf("button %d also routes to the map", button.Action)
				}
				for _, other := range layout.Buttons[i+1:] {
					if button.Rect.Overlaps(other.Rect) {
						t.Fatalf("width %d overlay %d: buttons %d/%d overlap", width, overlay, button.Action, other.Action)
					}
				}
			}
		}
	}
}

func TestHUDTerrainAndModalSafety(t *testing.T) {
	l := NewHUDLayout(539, 240, OverlayNone, false)
	for _, p := range []image.Point{{250, 100}, {538, 195}, {0, 100}} {
		if !l.IsTerrainPoint(p.X, p.Y) {
			t.Fatalf("terrain point rejected: %v", p)
		}
	}
	for _, p := range []image.Point{{-1, 100}, {539, 100}, {250, 23}, {250, 196}, {10, 40}} {
		if l.IsTerrainPoint(p.X, p.Y) {
			t.Fatalf("unsafe terrain point accepted: %v", p)
		}
	}
	if !l.IsMiniMapPoint(10, 40) {
		t.Fatal("minimap must have its own touch route")
	}
	pad := NewHUDLayout(539, 240, OverlayNone, true)
	if pad.IsTerrainPoint(pad.DPadRect.Min.X, pad.DPadRect.Min.Y) {
		t.Fatal("empty pad corners must not terraform")
	}
	for overlay := OverlayPowers; overlay <= OverlayConfirm; overlay++ {
		modal := NewHUDLayout(539, 240, overlay, true)
		for y := 0; y < 240; y++ {
			for x := 0; x < 539; x++ {
				if modal.IsTerrainPoint(x, y) || modal.IsMiniMapPoint(x, y) {
					t.Fatalf("overlay %d exposes map input at %d,%d", overlay, x, y)
				}
			}
		}
	}
}
