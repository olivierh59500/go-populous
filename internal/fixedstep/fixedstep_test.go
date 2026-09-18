package fixedstep

import "testing"

func TestEightStepsOverSixtyUpdates(t *testing.T) {
	scheduler := New(8, 60)
	total := 0
	for range 60 {
		total += scheduler.Advance()
	}
	if total != 8 {
		t.Fatalf("steps after 60 updates = %d, want 8", total)
	}
}

func TestEightHertzUsesSevenAndEightUpdateIntervals(t *testing.T) {
	scheduler := New(8, 60)
	previous := 0
	var intervals []int
	for update := 1; update <= 60; update++ {
		if steps := scheduler.Advance(); steps != 0 {
			if steps != 1 {
				t.Fatalf("steps at update %d = %d, want 1", update, steps)
			}
			intervals = append(intervals, update-previous)
			previous = update
		}
	}

	want := []int{8, 7, 8, 7, 8, 7, 8, 7}
	if len(intervals) != len(want) {
		t.Fatalf("interval count = %d, want %d (%v)", len(intervals), len(want), intervals)
	}
	for index := range want {
		if intervals[index] != want[index] {
			t.Fatalf("intervals = %v, want %v", intervals, want)
		}
	}
}

func TestMatchingRatesAdvanceOneForOne(t *testing.T) {
	scheduler := New(8, 8)
	for update := 1; update <= 32; update++ {
		if steps := scheduler.Advance(); steps != 1 {
			t.Fatalf("steps at update %d = %d, want 1", update, steps)
		}
	}
}

func TestSetUpdateRateResetsPhase(t *testing.T) {
	scheduler := New(8, 60)
	for update := 0; update < 7; update++ {
		if steps := scheduler.Advance(); steps != 0 {
			t.Fatalf("steps before rate change at update %d = %d, want 0", update+1, steps)
		}
	}

	scheduler.SetUpdateRate(8)
	if steps := scheduler.Advance(); steps != 1 {
		t.Fatalf("steps at matching rate = %d, want 1", steps)
	}

	scheduler.SetUpdateRate(60)
	for update := 1; update < 8; update++ {
		if steps := scheduler.Advance(); steps != 0 {
			t.Fatalf("steps after reset at update %d = %d, want 0", update, steps)
		}
	}
	if steps := scheduler.Advance(); steps != 1 {
		t.Fatalf("steps after reset at update 8 = %d, want 1", steps)
	}
}

func TestRatesAreClampedAndMultipleStepsAreSupported(t *testing.T) {
	tests := []struct {
		name       string
		stepRate   int
		updateRate int
		want       int
	}{
		{name: "both zero", stepRate: 0, updateRate: 0, want: 1},
		{name: "negative step rate", stepRate: -8, updateRate: 1, want: 1},
		{name: "negative update rate", stepRate: 8, updateRate: -60, want: 8},
		{name: "multiple steps", stepRate: 60, updateRate: 8, want: 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheduler := New(test.stepRate, test.updateRate)
			if steps := scheduler.Advance(); steps != test.want {
				t.Fatalf("Advance() = %d, want %d", steps, test.want)
			}
		})
	}

	scheduler := New(8, 60)
	scheduler.SetUpdateRate(0)
	if steps := scheduler.Advance(); steps != 8 {
		t.Fatalf("Advance() after clamped SetUpdateRate = %d, want 8", steps)
	}
}
