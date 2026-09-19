package mobileui

import (
	"image"
	"testing"
)

func TestFrontLayoutTouchTargets(t *testing.T) {
	for _, width := range []int{320, 539, 800} {
		for page := FrontHome; page <= FrontHelp; page++ {
			for _, requestedPage := range []int{-1, 0, 1, 2, 99} {
				l := NewFrontLayout(width, 240, page, requestedPage, 16)
				bounds := image.Rect(0, 0, width, 240)
				for i, button := range l.Buttons {
					if !button.Rect.In(bounds) || button.Rect.Dx() < 32 || button.Rect.Dy() < 32 {
						t.Fatalf("width %d page %d index %d: small or clipped button %+v", width, page, requestedPage, button)
					}
					for _, other := range l.Buttons[i+1:] {
						if button.Rect.Overlaps(other.Rect) {
							t.Fatalf("width %d page %d: overlapping targets %+v and %+v", width, page, button, other)
						}
					}
					center := button.Rect.Min.Add(button.Rect.Size().Div(2))
					action, index, ok := l.Hit(center.X, center.Y)
					if !ok || action != button.Action || index != button.Index {
						t.Fatalf("button center activates (%d, %d, %v), want %+v", action, index, ok, button)
					}
				}
				for _, point := range []image.Point{{0, 0}, {width - 1, 100}, {width / 2, 37}, {-1, 100}, {width, 100}, {10, 240}} {
					if action, _, ok := l.Hit(point.X, point.Y); ok || action != FrontActionNone {
						t.Fatalf("width %d page %d: background point %v activates %d", width, page, point, action)
					}
				}
			}
		}
	}
}

func TestFrontMenusExposeEveryDestination(t *testing.T) {
	l := NewFrontLayout(539, 240, FrontHome, 9, 0)
	seen := map[FrontAction]bool{}
	for index, b := range l.Buttons {
		if b.Index != index {
			t.Errorf("home item %d has index %d", index, b.Index)
		}
		seen[b.Action] = true
	}
	for _, action := range []FrontAction{FrontActionTutorial, FrontActionConquest, FrontActionCustom, FrontActionSetup, FrontActionPreferences, FrontActionLoad, FrontActionHelp, FrontActionDemo} {
		if !seen[action] {
			t.Errorf("home is missing action %d", action)
		}
	}
	if l.PageIndex != 0 || l.PageCount != 1 || len(l.Buttons) != 8 {
		t.Fatalf("home unexpectedly paginated or has extra targets: %+v", l)
	}
}

func TestFrontPaginationKeepsAbsoluteItemIndices(t *testing.T) {
	for _, test := range []struct {
		page           FrontPage
		count, perPage int
	}{
		{FrontSetup, 14, 6}, {FrontOptions, 16, 4},
	} {
		seen := make([]bool, test.count)
		lastPage := NewFrontLayout(320, 240, test.page, 999, test.count)
		for pageIndex := 0; pageIndex < lastPage.PageCount; pageIndex++ {
			l := NewFrontLayout(320, 240, test.page, pageIndex, test.count)
			if l.FirstItem != pageIndex*test.perPage || l.LastItem != min(test.count, l.FirstItem+test.perPage) {
				t.Fatalf("incorrect item interval on page %d: %+v", pageIndex, l)
			}
			for _, row := range l.Rows {
				if row.Index < 0 || row.Index >= test.count || seen[row.Index] {
					t.Fatalf("duplicate or invalid absolute row: %+v", row)
				}
				seen[row.Index] = true
			}
			for _, button := range l.Buttons {
				if button.Action == FrontActionPrevPage && pageIndex == 0 || button.Action == FrontActionNextPage && pageIndex == lastPage.PageIndex {
					t.Fatal("navigation exposes an unavailable page")
				}
			}
		}
		for index, visible := range seen {
			if !visible {
				t.Errorf("page %d hides item %d", test.page, index)
			}
		}
		if l := NewFrontLayout(320, 240, test.page, -10, test.count); l.PageIndex != 0 {
			t.Fatalf("negative page was not clamped: %+v", l)
		}
		if lastPage.PageIndex != lastPage.PageCount-1 {
			t.Fatalf("oversized page was not clamped: %+v", lastPage)
		}
	}
}

func TestFrontOptionsPowerControlsAndLabelDeadSpace(t *testing.T) {
	for page := 0; page < 4; page++ {
		l := NewFrontLayout(539, 240, FrontOptions, page, 16)
		startCount := 0
		for _, button := range l.Buttons {
			if button.Action == FrontActionStart {
				startCount++
				if !button.Rect.In(l.FooterRect) {
					t.Fatal("Start must remain in the footer on every options page")
				}
			}
		}
		if startCount != 1 {
			t.Fatalf("options page %d has %d Start buttons", page, startCount)
		}
		for _, row := range l.Rows {
			if _, _, ok := l.Hit(row.Rect.Min.X+8, (row.Rect.Min.Y+row.Rect.Max.Y)/2); ok {
				t.Fatalf("option label must not change its value: row %d", row.Index)
			}
			var actions []FrontAction
			for _, button := range l.Buttons {
				if button.Index == row.Index && button.Rect.In(row.Rect) {
					actions = append(actions, button.Action)
				}
			}
			left, right := FrontActionDecrease, FrontActionIncrease
			if row.Index >= 9 {
				left, right = FrontActionTogglePlayerPower, FrontActionToggleEnemyPower
			}
			if len(actions) != 2 || actions[0] != left || actions[1] != right {
				t.Fatalf("wrong controls for row %d: %v", row.Index, actions)
			}
		}
	}
}

func TestFrontPowerBoundaryIsConfigurable(t *testing.T) {
	for _, firstPower := range []int{9, 10} {
		l := NewFrontLayout(320, 240, FrontOptions, 2, 15, firstPower)
		for _, row := range l.Rows {
			for _, button := range l.Buttons {
				if !button.Rect.In(row.Rect) {
					continue
				}
				powerControl := button.Action == FrontActionTogglePlayerPower || button.Action == FrontActionToggleEnemyPower
				if powerControl != (row.Index >= firstPower) {
					t.Fatalf("first power %d: row %d has action %d", firstPower, row.Index, button.Action)
				}
			}
		}
	}
}

func TestFrontWorldNavigationStepMagnitudes(t *testing.T) {
	l := NewFrontLayout(320, 240, FrontConquest, 0, 495)
	want := map[FrontAction]map[int]bool{
		FrontActionPrevWorld: {1: false, 10: false},
		FrontActionNextWorld: {1: false, 10: false},
	}
	for _, button := range l.Buttons {
		if steps, ok := want[button.Action]; ok {
			if _, ok := steps[button.Index]; !ok {
				t.Fatalf("unexpected world navigation step %d", button.Index)
			}
			steps[button.Index] = true
		}
	}
	for action, steps := range want {
		for step, found := range steps {
			if !found {
				t.Errorf("missing action %d with step %d", action, step)
			}
		}
	}
}

func TestFrontEmptyPagesAndMinimumDimensions(t *testing.T) {
	for _, page := range []FrontPage{FrontSetup, FrontOptions, FrontHelp} {
		l := NewFrontLayout(1, 1, page, 10, -3)
		if l.Width != 320 || l.Height != 240 || l.PageIndex != 0 || l.PageCount != 1 || l.FirstItem != 0 || l.LastItem != 0 {
			t.Fatalf("invalid empty layout: %+v", l)
		}
		wantButtons := 1
		if page == FrontOptions {
			wantButtons = 2
		}
		if len(l.Buttons) != wantButtons || l.Buttons[0].Action != FrontActionBack {
			t.Fatalf("empty page must keep Back and, for options, Start: %+v", l.Buttons)
		}
	}
}
