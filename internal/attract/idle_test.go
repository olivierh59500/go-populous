package attract

import "testing"

func TestIdleRequiresContinuousInactivity(t *testing.T) {
	idle := NewIdle(3)
	if idle.Tick(false) || idle.Tick(false) {
		t.Fatal("demo became eligible before the timeout")
	}
	if idle.Tick(true) || idle.ElapsedTicks != 0 {
		t.Fatal("activity did not reset inactivity")
	}
	if idle.Tick(false) || idle.Tick(false) || !idle.Tick(false) {
		t.Fatal("demo did not become eligible after the full timeout")
	}
	for range 10 {
		if !idle.Tick(false) || idle.ElapsedTicks != 3 {
			t.Fatal("elapsed inactivity did not saturate at the timeout")
		}
	}
	idle.Reset()
	if idle.Tick(false) {
		t.Fatal("explicit reset kept the timer expired")
	}
}

func TestIdleCanBeDisabled(t *testing.T) {
	for _, limit := range []int{0, -1} {
		idle := NewIdle(limit)
		for range DefaultIdleTicks * 2 {
			if idle.Tick(false) || idle.ElapsedTicks != 0 {
				t.Fatalf("disabled timer with limit %d expired", limit)
			}
		}
	}
}
