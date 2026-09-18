package touchui

import "testing"

func TestLogicalWidth(t *testing.T) {
	tests := []struct {
		name          string
		outsideWidth  int
		outsideHeight int
		want          int
	}{
		{name: "invalid", want: SceneWidth},
		{name: "original aspect", outsideWidth: 960, outsideHeight: 720, want: SceneWidth},
		{name: "pixel landscape", outsideWidth: 2424, outsideHeight: 1080, want: 539},
		{name: "portrait", outsideWidth: 1080, outsideHeight: 2424, want: SceneWidth},
		{name: "ultrawide cap", outsideWidth: 4000, outsideHeight: 500, want: MaxWidth},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := LogicalWidth(test.outsideWidth, test.outsideHeight); got != test.want {
				t.Fatalf("LogicalWidth(%d, %d) = %d, want %d", test.outsideWidth, test.outsideHeight, got, test.want)
			}
		})
	}
}

func TestLayoutCentersOriginalCanvas(t *testing.T) {
	layout := NewLayout(539)
	if !layout.Enabled {
		t.Fatal("wide Pixel layout should enable touch controls")
	}
	if layout.SceneX != 109 {
		t.Fatalf("SceneX = %d, want 109", layout.SceneX)
	}
	if x, y, ok := layout.ScenePosition(109, 37); !ok || x != 0 || y != 37 {
		t.Fatalf("left scene mapping = (%d, %d, %v), want (0, 37, true)", x, y, ok)
	}
	if x, y, ok := layout.ScenePosition(428, 239); !ok || x != 319 || y != 239 {
		t.Fatalf("right scene mapping = (%d, %d, %v), want (319, 239, true)", x, y, ok)
	}
	if _, _, ok := layout.ScenePosition(429, 120); ok {
		t.Fatal("right side-band point mapped into the scene")
	}
}

func TestLayoutDisabledWithoutSideBands(t *testing.T) {
	layout := NewLayout(SceneWidth)
	if layout.Enabled {
		t.Fatal("original 320x240 canvas must not overlay touch controls")
	}
	if x, y, ok := layout.ScenePosition(17, 29); !ok || x != 17 || y != 29 {
		t.Fatalf("scene mapping = (%d, %d, %v), want unchanged", x, y, ok)
	}
}

func TestControlsCombineNavigationAndRightAction(t *testing.T) {
	layout := NewLayout(539)
	points := []Point{
		{X: layout.DPad.X + layout.DPad.Radius/2, Y: layout.DPad.Y - layout.DPad.Radius/2, Just: true},
		{X: layout.Lower.X, Y: layout.Lower.Y},
		{X: layout.SceneX + 160, Y: 92, Just: true},
	}
	controls := Evaluate(layout, points)
	if controls.DX != 1 || controls.DY != -1 {
		t.Fatalf("d-pad direction = (%d, %d), want (1, -1)", controls.DX, controls.DY)
	}
	if !controls.DPadJust || !controls.Lower || controls.Raise || !controls.SceneJust {
		t.Fatalf("unexpected combined controls: %+v", controls)
	}
	if controls.LowerJust {
		t.Fatal("held LOWER modifier must not be reported as newly pressed")
	}
}

func TestDPadHasDeadZone(t *testing.T) {
	layout := NewLayout(539)
	if dx, dy, ok := layout.DPadDirection(layout.DPad.X, layout.DPad.Y); ok || dx != 0 || dy != 0 {
		t.Fatalf("d-pad center = (%d, %d, %v), want dead zone", dx, dy, ok)
	}
	if dx, dy, ok := layout.DPadDirection(layout.DPad.X-layout.DPad.Radius, layout.DPad.Y); !ok || dx != -1 || dy != 0 {
		t.Fatalf("d-pad left edge = (%d, %d, %v), want (-1, 0, true)", dx, dy, ok)
	}
}

func TestMenuRequiresNewContact(t *testing.T) {
	layout := NewLayout(539)
	held := Evaluate(layout, []Point{{X: layout.Menu.X, Y: layout.Menu.Y}})
	if !held.Menu || held.MenuJust {
		t.Fatalf("held menu state = %+v, want held without activation", held)
	}
	pressed := Evaluate(layout, []Point{{X: layout.Menu.X, Y: layout.Menu.Y, Just: true}})
	if !pressed.Menu || !pressed.MenuJust {
		t.Fatalf("pressed menu state = %+v, want just activation", pressed)
	}
}

func TestActionButtonsTrackNewContacts(t *testing.T) {
	layout := NewLayout(539)
	controls := Evaluate(layout, []Point{
		{X: layout.Raise.X, Y: layout.Raise.Y, Just: true},
		{X: layout.Lower.X, Y: layout.Lower.Y},
	})
	if !controls.Raise || !controls.RaiseJust {
		t.Fatalf("RAISE state = %+v, want newly pressed", controls)
	}
	if !controls.Lower || controls.LowerJust {
		t.Fatalf("LOWER state = %+v, want held", controls)
	}
}

func TestOppositeDPadContactsCancel(t *testing.T) {
	layout := NewLayout(539)
	controls := Evaluate(layout, []Point{
		{X: layout.DPad.X - layout.DPad.Radius, Y: layout.DPad.Y},
		{X: layout.DPad.X + layout.DPad.Radius, Y: layout.DPad.Y},
	})
	if controls.DX != 0 || controls.DY != 0 {
		t.Fatalf("opposite contacts = (%d, %d), want cancellation", controls.DX, controls.DY)
	}
}

func TestLatchKeepsShortReleasedTapUntilSimulationStep(t *testing.T) {
	var latch Latch
	latch.Observe([]Contact{{ID: 7, X: 120, Y: 80}}, []int{7})
	if latch.Pending() != 1 {
		t.Fatalf("Pending() = %d, want 1", latch.Pending())
	}
	got := latch.Consume(nil)
	if len(got) != 1 || got[0] != (Point{X: 120, Y: 80, Just: true}) {
		t.Fatalf("Consume(nil) = %+v, want released tap", got)
	}
	if latch.Pending() != 0 {
		t.Fatalf("Pending() after consume = %d, want 0", latch.Pending())
	}
}

func TestLatchMarksHeldContactJustOnce(t *testing.T) {
	var latch Latch
	active := []Contact{{ID: 4, X: 20, Y: 30}}
	latch.Observe(active, []int{4})
	first := append([]Point(nil), latch.Consume(active)...)
	second := latch.Consume(active)
	if len(first) != 1 || !first[0].Just {
		t.Fatalf("first consume = %+v, want Just", first)
	}
	if len(second) != 1 || second[0].Just {
		t.Fatalf("second consume = %+v, want held without Just", second)
	}
}

func TestLatchUsesLatestPositionAndDoesNotDuplicate(t *testing.T) {
	var latch Latch
	latch.Observe([]Contact{{ID: 2, X: 10, Y: 11}}, []int{2})
	latch.Observe([]Contact{{ID: 2, X: 40, Y: 41}}, nil)
	got := latch.Consume([]Contact{{ID: 2, X: 50, Y: 51}})
	if len(got) != 1 || got[0] != (Point{X: 50, Y: 51, Just: true}) {
		t.Fatalf("Consume(active) = %+v, want one latest active contact", got)
	}
}

func TestLatchKeepsTwoPressEventsWhenIDIsReused(t *testing.T) {
	var latch Latch
	latch.Observe([]Contact{{ID: 0, X: 10, Y: 11}}, []int{0})
	latch.Observe(nil, nil)
	latch.Observe([]Contact{{ID: 0, X: 90, Y: 91}}, []int{0})
	got := latch.Consume([]Contact{{ID: 0, X: 90, Y: 91}})
	if len(got) != 2 {
		t.Fatalf("Consume(active) returned %d events, want 2: %+v", len(got), got)
	}
	if got[0] != (Point{X: 90, Y: 91, Just: true}) || got[1] != (Point{X: 10, Y: 11, Just: true}) {
		t.Fatalf("Consume(active) = %+v, want current press then queued earlier tap", got)
	}
}

func TestLatchPreservesMultitouchModifierAndTarget(t *testing.T) {
	var latch Latch
	latch.Observe([]Contact{
		{ID: 1, X: 500, Y: 194},
		{ID: 2, X: 250, Y: 90},
	}, []int{1, 2})
	got := latch.Consume(nil)
	if len(got) != 2 || !got[0].Just || !got[1].Just {
		t.Fatalf("Consume(nil) = %+v, want both latched contacts", got)
	}
}
