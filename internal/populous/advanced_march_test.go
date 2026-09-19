package populous

import "testing"

func advancedMarchWorld(player int) *World {
	w := advancedTestWorld()
	w.GameTurn = 6 * 60 * 8
	w.Magnets[player].Mana = 20000
	w.Magnets[player].Carried = 1
	w.Peeps = []Peep{
		{Player: byte(player), Flags: InTown, Population: 12000, Weapons: 3, AtPos: 10 + 10*MapWidth},
		{Player: byte(player ^ 1), Flags: InTown, Population: 500, Weapons: 3, AtPos: 30 + 10*MapWidth},
	}
	for i, p := range w.Peeps {
		w.MapWho[p.AtPos] = byte(i + 1)
	}
	return w
}

func TestAdvancedMarchUsesPaidCommandsAndOneActionSlot(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := advancedMarchWorld(player)
		beforePop, beforeRNG := w.PlayerPopulations(), w.rng
		w.runAdvancedComputerPlayer(player)
		if w.Computer[player].DoneTurn != w.GameTurn || w.Computer[player].QuakeCount >= 0 {
			t.Fatal("expedition did not consume its action slot")
		}
		if w.Magnets[player].Mana != 20000-ManaMagnetCost || w.Peeps[0].Flags != OnMove || w.Peeps[0].AtPos != 10+10*MapWidth {
			t.Fatal("expedition did not use the ordinary paid magnet command")
		}
		if w.PlayerPopulations() != beforePop || w.rng != beforeRNG {
			t.Fatal("expedition modified population or random state")
		}
		before := w.StateHash()
		w.runAdvancedComputerPlayer(player)
		if w.StateHash() != before {
			t.Fatal("second action used a closed slot")
		}
	}
}

func TestAdvancedMarchRouteDetoursWithoutChangingWorld(t *testing.T) {
	w := advancedMarchWorld(0)
	w.MapBlk[14+10*MapWidth] = RockBlock
	w.MapBlk[13+10*MapWidth] = SwampBlock
	before := w.StateHash()
	target, waypoint, distance := w.advancedMarchRoute(0)
	if target != 1 || distance < 20 || waypoint == w.Peeps[0].AtPos || !w.advancedStraightMarch(w.Peeps[0].AtPos, waypoint) {
		t.Fatalf("unsafe or missing detour: target=%d waypoint=%d distance=%d", target, waypoint, distance)
	}
	if w.StateHash() != before {
		t.Fatal("route search changed the world")
	}
	for y := 0; y < MapHeight; y++ {
		w.MapBlk[20+y*MapWidth] = WaterBlock
	}
	before = w.StateHash()
	if target, _, _ := w.advancedMarchRoute(0); target >= 0 || w.StateHash() != before {
		t.Fatal("route crossed an impassable sea or modified the world")
	}
}

func TestAdvancedMarchRouteRejectsInvalidAndOverpoweringTargets(t *testing.T) {
	w := advancedMarchWorld(0)
	w.Peeps[1].Population = 32000
	w.Peeps[1].Weapons = 20
	before := w.StateHash()
	for _, index := range []int{-1, 0, 1000} {
		if target, _, _ := w.advancedMarchRoute(index); target >= 0 || w.StateHash() != before {
			t.Fatal("route accepted an invalid group or a suicidal battle")
		}
	}
	if w.advancedStraightMarch(-1, 20) || w.advancedStraightMarch(20, MapWidth*MapHeight) {
		t.Fatal("route accepted off-map coordinates")
	}
}

func TestAdvancedMarchWakesCarrierAfterFriendlyMerger(t *testing.T) {
	w := advancedMarchWorld(0)
	if handled, acted := w.advancedMarch(0); !handled || !acted {
		t.Fatal("expedition did not start")
	}
	w.Magnets[0].GoTo = 18 + 10*MapWidth
	w.Peeps[0].Flags = InTown // Result of the ordinary friendly-town merger.
	before := w.Magnets[0].Mana
	if handled, acted := w.advancedMarch(0); !handled || !acted || w.Peeps[0].Flags != OnMove || w.Magnets[0].Mana != before-ManaMagnetCost {
		t.Fatal("captured/merged carrier was not woken with a paid command")
	}
}

func TestAdvancedMarchHonoursMilitaryAndResourceRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*World)
	}{
		{"war already active", func(w *World) { w.War = true }},
		{"knights available", func(w *World) { w.Computer[0].Mode |= computerKnight }},
		{"armageddon available", func(w *World) { w.Computer[0].Mode |= computerWar }},
		{"insufficient mana", func(w *World) { w.Magnets[0].Mana = ManaMagnetCost - 1 }},
		{"too weak", func(w *World) { w.Peeps[0].Population = 100 }},
		{"too early", func(w *World) { w.GameTurn = 60 * 8 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedMarchWorld(0)
			tc.edit(w)
			before := w.StateHash()
			if handled, acted := w.advancedMarch(0); handled || acted || w.StateHash() != before {
				t.Fatal("ineligible expedition changed the world")
			}
		})
	}
}

func TestAdvancedMarchResumesDeterministically(t *testing.T) {
	w := advancedMarchWorld(0)
	w.advancedMarch(0)
	other := WorldFromSnapshot(w.Snapshot(), w.Rules)
	for tick := 0; tick < 128; tick++ {
		w.TickWithAdvancedComputer(0)
		other.TickWithAdvancedComputer(0)
		if w.StateHash() != other.StateHash() {
			t.Fatalf("expedition snapshot diverged at tick %d", tick)
		}
	}
}
