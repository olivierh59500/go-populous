package mobileui

import "testing"

func testRegion(point Point) Region {
	if point.X < 0 || point.Y < 0 {
		return RegionNone
	}
	if point.Y >= 200 {
		return RegionUI
	}
	return RegionTerrain
}

func testContact(id int, x, y float64) Contact {
	return Contact{ID: id, X: x, Y: y}
}

func TestGestureCommitsTapOnlyOnRelease(t *testing.T) {
	var gesture Gesture
	for sample := 0; sample < 20; sample++ {
		frame := gesture.Update([]Contact{testContact(1, 80, 90)}, testRegion)
		if len(frame.Taps) != 0 || frame.Panning || !frame.HasPreview || frame.Preview != (Point{80, 90}) {
			t.Fatalf("sample %d emitted action or lost preview: %+v", sample, frame)
		}
	}
	frame := gesture.Update(nil, testRegion)
	if len(frame.Taps) != 1 || frame.Taps[0] != (Tap{Point: Point{80, 90}, Start: Point{80, 90}, Region: RegionTerrain}) {
		t.Fatalf("release = %+v, want one terrain tap", frame)
	}
	if repeated := gesture.Update(nil, testRegion); len(repeated.Taps) != 0 {
		t.Fatalf("tap repeated: %+v", repeated)
	}
}

func TestGestureUIReleaseRetainsStartAndEnd(t *testing.T) {
	var gesture Gesture
	frame := gesture.Update([]Contact{testContact(8, 30, 210)}, testRegion)
	if frame.HasPreview || len(frame.Taps) != 0 {
		t.Fatalf("UI down = %+v", frame)
	}
	gesture.Update([]Contact{testContact(8, 35, 212)}, testRegion)
	frame = gesture.Update(nil, testRegion)
	want := Tap{Point: Point{35, 212}, Start: Point{30, 210}, Region: RegionUI}
	if len(frame.Taps) != 1 || frame.Taps[0] != want {
		t.Fatalf("UI release = %+v, want %+v", frame, want)
	}
}

func TestGestureSecondFingerCancelsTapAndPansWithoutJump(t *testing.T) {
	var gesture Gesture
	gesture.Update([]Contact{testContact(1, 20, 20)}, testRegion)
	// The first finger moved while the second arrived: do not change the camera.
	frame := gesture.Update([]Contact{testContact(1, 23, 24), testContact(2, 120, 100)}, testRegion)
	if !frame.Panning || frame.Pan != (Point{}) || frame.HasPreview || len(frame.Taps) != 0 {
		t.Fatalf("second finger = %+v", frame)
	}
	// Contact ordering must not affect centroid motion.
	frame = gesture.Update([]Contact{testContact(2, 130, 94), testContact(1, 33, 18)}, testRegion)
	if !frame.Panning || frame.Pan != (Point{10, -6}) || len(frame.Taps) != 0 {
		t.Fatalf("pan = %+v, want (10, -6)", frame)
	}
	frame = gesture.Update([]Contact{testContact(1, 50, 30)}, testRegion)
	if !frame.Panning || frame.Pan != (Point{}) || frame.HasPreview || len(frame.Taps) != 0 {
		t.Fatalf("one remaining finger = %+v", frame)
	}
	frame = gesture.Update(nil, testRegion)
	if frame.Panning || len(frame.Taps) != 0 {
		t.Fatalf("pan release emitted tap: %+v", frame)
	}
	gesture.Update([]Contact{testContact(1, 50, 30)}, testRegion)
	if frame = gesture.Update(nil, testRegion); len(frame.Taps) != 1 {
		t.Fatalf("new gesture was not released from capture: %+v", frame)
	}
}

func TestGesturePanRebasesWhenFingerReplaced(t *testing.T) {
	var gesture Gesture
	gesture.Update([]Contact{testContact(1, 20, 20), testContact(2, 80, 80)}, testRegion)
	gesture.Update([]Contact{testContact(1, 30, 20)}, testRegion)
	frame := gesture.Update([]Contact{testContact(1, 30, 20), testContact(3, 180, 180)}, testRegion)
	if !frame.Panning || frame.Pan != (Point{}) {
		t.Fatalf("rejoin jumped: %+v", frame)
	}
	frame = gesture.Update([]Contact{testContact(1, 32, 24), testContact(3, 182, 184)}, testRegion)
	if frame.Pan != (Point{2, 4}) {
		t.Fatalf("rejoined motion = %+v", frame)
	}
	frame = gesture.Update([]Contact{testContact(1, 32, 24), {ID: 3, X: 10, Y: 10, Started: true}}, testRegion)
	if !frame.Panning || frame.Pan != (Point{}) {
		t.Fatalf("reused ID jumped: %+v", frame)
	}
	if frame = gesture.Update(nil, testRegion); len(frame.Taps) != 0 {
		t.Fatalf("reused pan ID emitted tap: %+v", frame)
	}
}

func TestGesturePanCapturesTerrainAcrossUI(t *testing.T) {
	var gesture Gesture
	gesture.Update([]Contact{testContact(1, 40, 190), testContact(2, 80, 190)}, testRegion)
	frame := gesture.Update([]Contact{testContact(1, 40, 220), testContact(2, 80, 220)}, testRegion)
	if !frame.Panning || frame.Pan != (Point{0, 30}) {
		t.Fatalf("captured fingers crossed UI: %+v", frame)
	}
	if frame = gesture.Update(nil, testRegion); len(frame.Taps) != 0 {
		t.Fatalf("pan ending over UI activated it: %+v", frame)
	}
}

func TestGestureUnsafeSequencesNeverTap(t *testing.T) {
	tests := []struct {
		name   string
		frames [][]Contact
	}{
		{"terrain then UI", [][]Contact{
			{testContact(1, 40, 100)},
			{testContact(1, 40, 100), testContact(2, 40, 210)},
			{testContact(1, 40, 100)},
		}},
		{"UI then terrain", [][]Contact{
			{testContact(1, 40, 210)},
			{testContact(1, 40, 210), testContact(2, 40, 100)},
			{testContact(2, 40, 100)},
		}},
		{"two UI fingers", [][]Contact{{testContact(1, 40, 210), testContact(2, 80, 210)}}},
		{"third finger", [][]Contact{
			{testContact(1, 40, 100), testContact(2, 80, 100)},
			{testContact(1, 40, 100), testContact(2, 80, 100), testContact(3, 120, 100)},
			{testContact(1, 40, 100), testContact(2, 80, 100)},
		}},
		{"drag and return", [][]Contact{{testContact(1, 40, 100)}, {testContact(1, 60, 100)}, {testContact(1, 40, 100)}}},
		{"terrain crosses UI and returns", [][]Contact{{testContact(1, 40, 199)}, {testContact(1, 40, 201)}, {testContact(1, 40, 199)}}},
		{"UI crosses terrain", [][]Contact{{testContact(1, 40, 201)}, {testContact(1, 40, 199)}}},
		{"unclassified", [][]Contact{{testContact(1, -1, 100)}, {testContact(1, 1, 100)}}},
		{"new ID without release sample", [][]Contact{{testContact(1, 40, 100)}, {testContact(2, 40, 100)}}},
		{"reused ID without release sample", [][]Contact{{testContact(1, 40, 100)}, {{ID: 1, X: 40, Y: 100, Started: true}}}},
		{"duplicate IDs", [][]Contact{{testContact(1, 40, 100), testContact(1, 80, 100)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gesture Gesture
			for i, contacts := range test.frames {
				if frame := gesture.Update(contacts, testRegion); len(frame.Taps) != 0 {
					t.Fatalf("frame %d emitted tap: %+v", i, frame)
				}
			}
			if frame := gesture.Update(nil, testRegion); len(frame.Taps) != 0 {
				t.Fatalf("release emitted tap: %+v", frame)
			}
		})
	}
}

func TestGestureCancelCapturesUntilAllReleased(t *testing.T) {
	for _, active := range []bool{false, true} {
		var gesture Gesture
		if active {
			gesture.Update([]Contact{testContact(1, 40, 100)}, testRegion)
		}
		gesture.Cancel()
		for i := 0; i < 3; i++ {
			frame := gesture.Update([]Contact{testContact(1, 40, 100)}, testRegion)
			if frame.HasPreview || frame.Panning || len(frame.Taps) != 0 {
				t.Fatalf("active=%v: canceled input escaped capture: %+v", active, frame)
			}
		}
		if frame := gesture.Update(nil, testRegion); len(frame.Taps) != 0 {
			t.Fatalf("active=%v: canceled input released a tap: %+v", active, frame)
		}
		gesture.Update([]Contact{testContact(1, 40, 100)}, testRegion)
		if frame := gesture.Update(nil, testRegion); len(frame.Taps) != 1 {
			t.Fatalf("active=%v: capture not released: %+v", active, frame)
		}
	}
}

func TestGestureTapSlopAndNilClassifier(t *testing.T) {
	gesture := Gesture{TapSlop: 2}
	gesture.Update([]Contact{testContact(1, 40, 100)}, testRegion)
	gesture.Update([]Contact{testContact(1, 42, 100)}, testRegion)
	if frame := gesture.Update(nil, testRegion); len(frame.Taps) != 1 {
		t.Fatalf("tap on slop boundary discarded: %+v", frame)
	}
	gesture.Update([]Contact{testContact(1, 40, 100)}, testRegion)
	gesture.Update([]Contact{testContact(1, 42.1, 100)}, testRegion)
	if frame := gesture.Update(nil, testRegion); len(frame.Taps) != 0 {
		t.Fatalf("configured slop lost after reset: %+v", frame)
	}
	gesture.Update([]Contact{testContact(1, 40, 100)}, nil)
	if frame := gesture.Update(nil, nil); len(frame.Taps) != 0 {
		t.Fatalf("nil classifier emitted tap: %+v", frame)
	}
}

func TestGestureUIHeldOnlyForValidSingleContact(t *testing.T) {
	var gesture Gesture
	frame := gesture.Update([]Contact{testContact(1, 40, 210)}, testRegion)
	if !frame.UIHeld || frame.UIStart != (Point{40, 210}) || frame.UIPoint != (Point{40, 210}) {
		t.Fatalf("initial UI press = %+v", frame)
	}
	frame = gesture.Update([]Contact{testContact(1, 42, 212)}, testRegion)
	if !frame.UIHeld || frame.UIStart != (Point{40, 210}) || frame.UIPoint != (Point{42, 212}) {
		t.Fatalf("held UI press = %+v", frame)
	}
	if frame = gesture.Update(nil, testRegion); frame.UIHeld {
		t.Fatalf("released UI contact still held: %+v", frame)
	}

	tests := []struct {
		name   string
		frames [][]Contact
		cancel bool
	}{
		{"mixed contacts then remaining UI", [][]Contact{
			{testContact(1, 40, 210), testContact(2, 80, 100)},
			{testContact(1, 40, 210)},
		}, false},
		{"two UI contacts then remaining UI", [][]Contact{
			{testContact(1, 40, 210), testContact(2, 80, 210)},
			{testContact(1, 40, 210)},
		}, false},
		{"UI drag then return", [][]Contact{
			{testContact(1, 55, 210)},
			{testContact(1, 40, 210)},
		}, false},
		{"UI leaves region then returns", [][]Contact{
			{testContact(1, 40, 199)},
			{testContact(1, 40, 210)},
		}, false},
		{"cancel then held UI", [][]Contact{{testContact(1, 40, 210)}}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gesture Gesture
			gesture.Update([]Contact{testContact(1, 40, 210)}, testRegion)
			if test.cancel {
				gesture.Cancel()
			}
			for i, contacts := range test.frames {
				if frame := gesture.Update(contacts, testRegion); frame.UIHeld {
					t.Fatalf("frame %d: invalid gesture exposed held UI: %+v", i, frame)
				}
			}
		})
	}
}

func TestGesturePanEndingOverUIDoesNotHoldUI(t *testing.T) {
	var gesture Gesture
	for i, contacts := range [][]Contact{
		{testContact(1, 40, 190), testContact(2, 80, 190)},
		{testContact(1, 40, 210), testContact(2, 80, 210)},
		{testContact(1, 40, 210)},
		nil,
	} {
		if frame := gesture.Update(contacts, testRegion); frame.UIHeld {
			t.Fatalf("frame %d: pan exposed held UI: %+v", i, frame)
		}
	}
}
