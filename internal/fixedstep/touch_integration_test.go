package fixedstep_test

import (
	"testing"

	"go-populous/internal/fixedstep"
	"go-populous/internal/touchui"
)

func TestSixtyHertzPollingDeliversShortTapToEightHertzStep(t *testing.T) {
	scheduler := fixedstep.New(8, 60)
	var latch touchui.Latch
	var delivered []touchui.Point

	for update := 1; update <= 8; update++ {
		var active []touchui.Contact
		var just []int
		if update == 2 || update == 3 {
			active = []touchui.Contact{{ID: 9, X: 181, Y: 73}}
		}
		if update == 2 {
			just = []int{9}
		}
		latch.Observe(active, just)

		if steps := scheduler.Advance(); steps != 0 {
			if steps != 1 {
				t.Fatalf("steps at update %d = %d, want 1", update, steps)
			}
			delivered = append(delivered, latch.Consume(active)...)
		}
	}

	want := touchui.Point{X: 181, Y: 73, Just: true}
	if len(delivered) != 1 || delivered[0] != want {
		t.Fatalf("delivered = %+v, want [%+v]", delivered, want)
	}
}
