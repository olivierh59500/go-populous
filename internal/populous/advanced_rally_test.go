package populous

import "testing"

func advancedRallyWorld(player int, knight bool) *World {
	w := advancedTestWorld()
	w.Rules.WalkDeath = 8
	for stage := 2; stage <= 9; stage++ {
		w.Rules.ManaAdd[stage] = 3
		w.Rules.PopulationAdd[stage] = 1
		w.Rules.WeaponsAdd[stage] = 3
	}
	w.GameTurn = 6 * 60 * 8
	if knight {
		w.Computer[player].Mode |= computerKnight
	}
	w.Magnets[player].Mana = 20000
	w.Magnets[player].Carried = 1
	w.Peeps = []Peep{
		{Player: byte(player), Flags: InTown, Population: 1800, Weapons: 3, AtPos: 10 + 10*MapWidth},
		{Player: byte(player), Flags: InTown, Population: 2300, Weapons: 3, AtPos: 14 + 10*MapWidth},
		{Player: byte(player ^ 1), Flags: InTown, Population: 4000, Weapons: 3, AtPos: 30 + 10*MapWidth},
	}
	for i, p := range w.Peeps {
		w.MapWho[p.AtPos] = byte(i + 1)
	}
	w.updateComputerStats()
	return w
}

func TestAdvancedRallyUsesOrdinaryMergerAndKnight(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := advancedRallyWorld(player, true)
		beforePop, beforeRNG := w.PlayerPopulations(), w.rng
		if handled, acted := w.advancedRallyTown(player); !handled || !acted {
			t.Fatal("rally did not start")
		}
		if w.Magnets[player].GoTo != w.Peeps[1].AtPos || w.Magnets[player].Mana != 20000-ManaMagnetCost || w.Peeps[0].Flags != OnMove {
			t.Fatal("rally did not use the paid magnet")
		}
		if w.PlayerPopulations() != beforePop || w.rng != beforeRNG {
			t.Fatal("rally directly modified population or RNG")
		}
		w.Peeps[0].AtPos = w.Peeps[1].AtPos
		w.resolveContact(0, 1) // Ordinary movement contact, not AI manipulation.
		if w.carriedPeepIndex(player) != 1 || w.Peeps[1].Population != 4100 {
			t.Fatal("normal merger did not transfer the carrier")
		}
		if handled, acted := w.advancedRallyTown(player); !handled || !acted || !isHeadedPeep(w.Peeps[1]) {
			t.Fatal("rallied group did not become a knight")
		}
		if w.Magnets[player].Mana != 20000-ManaMagnetCost-ManaKnightCost || w.PlayerPopulations() != beforePop || w.rng != beforeRNG {
			t.Fatal("knight did not use normal resources")
		}
	}
}

func TestAdvancedRallyWithoutKnightHandsOverToMarch(t *testing.T) {
	w := advancedRallyWorld(0, false)
	w.Peeps[0].Population = 800
	if handled, acted := w.advancedRallyTown(0); !handled || !acted {
		t.Fatal("conventional rally did not start")
	}
	w.Peeps[0].AtPos = w.Peeps[1].AtPos
	w.resolveContact(0, 1)
	before := w.Magnets[0].Mana
	if handled, acted := w.advancedRallyTown(0); handled || acted || w.Computer[0].QuakeCount != 0 {
		t.Fatal("mature conventional force was not released for marching")
	}
	if w.Magnets[0].Mana != before || w.Peeps[1].HeadFor != 0 {
		t.Fatal("rally cast an unavailable knight spell")
	}
}

func TestAdvancedRallyRequiresFundedSafeRoute(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*World)
	}{
		{"insufficient mana", func(w *World) { w.Magnets[0].Mana = ManaKnightCost - 1 }},
		{"swamp in route", func(w *World) { w.MapBlk[12+10*MapWidth] = SwampBlock }},
		{"not a flat-stage economy", func(w *World) { w.Rules.PopulationAdd[8]++ }},
		{"too far", func(w *World) { w.Peeps[1].AtPos = 55 + 10*MapWidth }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedRallyWorld(0, true)
			tc.edit(w)
			before := w.StateHash()
			if handled, acted := w.advancedRallyTown(0); handled || acted || w.StateHash() != before {
				t.Fatal("unsafe or unfunded recruitment changed the world")
			}
		})
	}
}

func TestAdvancedRallyResumesDeterministically(t *testing.T) {
	w := advancedRallyWorld(0, true)
	w.advancedRallyTown(0)
	other := WorldFromSnapshot(w.Snapshot(), w.Rules)
	for tick := 0; tick < 128; tick++ {
		w.TickWithAdvancedComputer(0)
		other.TickWithAdvancedComputer(0)
		if w.StateHash() != other.StateHash() {
			t.Fatalf("rally snapshot diverged at tick %d", tick)
		}
	}
}
