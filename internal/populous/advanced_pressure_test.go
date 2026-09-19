package populous

import "testing"

func advancedPressureWorld(player int) *World {
	w := advancedTestWorld()
	w.Rules.WalkDeath = 8
	w.Peeps = []Peep{
		{Player: byte(player), Flags: OnMove, Population: 100, AtPos: 13 + 13*MapWidth},
		{Player: byte(player ^ 1), Flags: OnMove, Population: 4000, AtPos: 10 + 10*MapWidth, HeadFor: 1},
	}
	for i, p := range w.Peeps {
		w.MapWho[p.AtPos] = byte(i + 1)
	}
	return w
}

func TestAdvancedPressureLowersKnightTileThroughPaidCommands(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := advancedPressureWorld(player)
		beforeRNG, beforePop := w.rng, w.PlayerPopulations()
		beforePeople := append([]Peep(nil), w.Peeps...)
		for step := 0; step < 4; step++ {
			if !w.advancedPressureLand(player) {
				t.Fatalf("legal trench step %d was not taken", step)
			}
		}
		if w.MapBlk[w.Peeps[1].AtPos] != WaterBlock || w.MapBlk[w.Peeps[0].AtPos] == WaterBlock {
			t.Fatal("trench did not isolate the knight safely")
		}
		if w.Magnets[player].Mana != 1000-4*(ManaPointCost+4) || w.rng != beforeRNG || w.PlayerPopulations() != beforePop {
			t.Fatal("trench bypassed normal resources or simulated people/RNG")
		}
		for i, p := range beforePeople {
			if w.Peeps[i] != p {
				t.Fatal("terrain attack directly changed an entity")
			}
		}
	}
}

func TestAdvancedPressureRespectsLocalConstructionAndAllies(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*World)
	}{
		{"no build", func(w *World) { w.Level.GameMode = GameNoBuild }},
		{"only raise", func(w *World) { w.Level.GameMode = GameOnlyRaise }},
		{"town required", func(w *World) { w.Level.GameMode = GameRaiseTown }},
		{"no land power", func(w *World) { w.Computer[0].Mode = computerKnight }},
		{"no mana", func(w *World) { w.Magnets[0].Mana = 0 }},
		{"remote enemy", func(w *World) { w.Peeps[1].AtPos = 50 + 50*MapWidth }},
		{"town at target", func(w *World) {
			w.MapWho[w.Peeps[0].AtPos] = 0
			w.Peeps[0].AtPos = w.Peeps[1].AtPos
			w.Peeps[0].Flags = InTown
			w.MapWho[w.Peeps[0].AtPos] = 1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedPressureWorld(0)
			tc.edit(w)
			before := w.StateHash()
			if w.advancedPressureLand(0) || w.StateHash() != before {
				t.Fatal("unsafe/forbidden terrain attack changed the world")
			}
		})
	}
}

func TestAdvancedPressureRequiresDominanceForOrdinaryEnemy(t *testing.T) {
	w := advancedPressureWorld(0)
	w.Peeps[1].HeadFor = 0
	before := w.StateHash()
	if w.advancedPressureLand(0) || w.StateHash() != before {
		t.Fatal("ordinary enemy triggered a risky trade without a decisive lead")
	}
	w.Peeps[0].Population = 15000
	if !w.advancedPressureLand(0) {
		t.Fatal("decisive lead did not permit the ordinary enemy siege")
	}
}
