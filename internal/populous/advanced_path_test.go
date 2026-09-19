package populous

import (
	"reflect"
	"testing"
)

func advancedEscapeWorld(block byte) *World {
	w := &World{}
	for pos := range w.MapBlk {
		w.MapBlk[pos] = block
	}
	return w
}

func cloneAdvancedEscapeWorld(w *World) *World {
	clone := *w
	clone.Peeps = append([]Peep(nil), w.Peeps...)
	clone.SoundEvents = append([]int(nil), w.SoundEvents...)
	return &clone
}

func advancedUnreachableTargetEscapeWorld() *World {
	w := advancedEscapeWorld(RockBlock)
	start := 10 + 10*MapWidth
	current := 12 + 10*MapWidth
	reachable := 8 + 8*MapWidth
	for _, pos := range []int{start, 9 + 10*MapWidth, 8 + 10*MapWidth, 8 + 9*MapWidth, reachable, current} {
		w.MapBlk[pos] = FlatBlock
	}
	w.Peeps = []Peep{
		{Player: GodPlayer, Population: 3000, Flags: OnMove, Status: KnightStatus, HeadFor: 2, AtPos: start, MagnetLastMove: -1},
		{Player: DevilPlayer, Population: 100, Flags: InTown, AtPos: current},
		{Player: DevilPlayer, Population: 100, Flags: InTown, AtPos: reachable},
	}
	return w
}

func TestAdvancedKnightEscapeBacktracksAfterLegacyNoMove(t *testing.T) {
	w := advancedEscapeWorld(RockBlock)
	start := 10 + 10*MapWidth
	target := 14 + 10*MapWidth
	// The only exit is west, which the historical fallback rejects because it
	// equals MagnetLastMove. The carved corridor then loops safely to the target.
	for _, pos := range []int{
		start, 9 + 10*MapWidth, 8 + 10*MapWidth,
		8 + 9*MapWidth, 8 + 8*MapWidth, 9 + 8*MapWidth,
		10 + 8*MapWidth, 11 + 8*MapWidth, 12 + 8*MapWidth,
		13 + 8*MapWidth, 14 + 8*MapWidth, 14 + 9*MapWidth, target,
	} {
		w.MapBlk[pos] = FlatBlock
	}
	w.Peeps = []Peep{
		{Player: GodPlayer, Population: 3000, Flags: OnMove, Status: KnightStatus, HeadFor: 2, AtPos: start, MagnetLastMove: -1},
		{Player: DevilPlayer, Population: 100, Flags: InTown, AtPos: target},
	}

	if legacy := w.moveKnightPeep(0); legacy != noMove {
		t.Fatalf("legacy move=%d, want noMove fixture", legacy)
	}
	move, ok := w.advancedKnightEscape(0)
	if !ok || move != -1 {
		t.Fatalf("escape move=%d ok=%t, want west backtrack", move, ok)
	}
	if w.Peeps[0].HeadFor != 2 {
		t.Fatalf("escape retargeted reachable current enemy: HeadFor=%d", w.Peeps[0].HeadFor)
	}
}

func TestAdvancedKnightEscapeReplacesOnlyUnreachableTarget(t *testing.T) {
	w := advancedUnreachableTargetEscapeWorld()
	if legacy := w.moveKnightPeep(0); legacy != noMove {
		t.Fatalf("legacy move=%d, want noMove fixture", legacy)
	}
	move, ok := w.advancedKnightEscape(0)
	if !ok || move != -1 {
		t.Fatalf("move=%d ok=%t, want west toward reachable enemy", move, ok)
	}
	if w.Peeps[0].HeadFor != 3 {
		t.Fatalf("HeadFor=%d, want reachable peep 3", w.Peeps[0].HeadFor)
	}
}

func TestAdvancedKnightEscapeNoPathLeavesStateUntouched(t *testing.T) {
	for _, tc := range []struct {
		name     string
		obstacle byte
	}{
		{name: "water", obstacle: WaterBlock},
		{name: "rock", obstacle: RockBlock},
		{name: "swamp", obstacle: SwampBlock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedEscapeWorld(FlatBlock)
			start := 20 + 20*MapWidth
			w.Peeps = []Peep{
				{Player: GodPlayer, Population: 3000, Flags: OnMove, Status: KnightStatus, HeadFor: 2, AtPos: start},
				{Player: DevilPlayer, Population: 100, Flags: InTown, AtPos: start + 3},
			}
			for _, delta := range toOffset {
				w.MapBlk[start+delta] = tc.obstacle
			}
			if legacy := w.moveKnightPeep(0); legacy != noMove {
				t.Fatalf("legacy move=%d, want noMove fixture", legacy)
			}
			before := cloneAdvancedEscapeWorld(w)

			if move, ok := w.advancedKnightEscape(0); ok || move != 0 {
				t.Fatalf("obstacle=%d returned move=%d ok=%t", tc.obstacle, move, ok)
			}
			if !reflect.DeepEqual(w, before) {
				t.Fatal("failed escape changed world state")
			}
		})
	}
}

func TestAdvancedKnightEscapeOnlyChangesHeadFor(t *testing.T) {
	w := advancedUnreachableTargetEscapeWorld()
	w.rng = lcg(1234)
	w.Magnets[GodPlayer] = Magnet{Mana: 9876, GoTo: 42, Flags: SettleMode}
	w.SoundEvents = []int{TuneMagnet}
	if legacy := w.moveKnightPeep(0); legacy != noMove {
		t.Fatalf("legacy move=%d, want noMove fixture", legacy)
	}
	before := cloneAdvancedEscapeWorld(w)

	move, ok := w.advancedKnightEscape(0)
	if !ok || move == 0 {
		t.Fatalf("move=%d ok=%t, want reachable escape", move, ok)
	}
	before.Peeps[0].HeadFor = 3
	if !reflect.DeepEqual(w, before) {
		t.Fatal("escape changed state other than HeadFor")
	}
}
