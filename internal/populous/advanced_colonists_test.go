package populous

import "testing"

func colonistReleaseWorld(player int) *World {
	w := legacyLoopCastle(player, 700)
	w.ComputerControlled = [2]bool{true, true}
	w.Computer[player].Mode = computerLand
	w.Magnets[player].Mana = 1000
	w.GameTurn = 6
	return w
}

func TestAdvancedColonistsUsePaidLandAndNormalEmigration(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := colonistReleaseWorld(player)
		pos := w.Peeps[0].AtPos
		beforeAlt, beforePeep, beforeRNG := w.Alt, w.Peeps[0], w.rng
		if !w.advancedReleaseColonists(player) {
			t.Fatal("no legal release selected")
		}
		if w.Peeps[0] != beforePeep || w.rng != beforeRNG || len(w.Peeps) != 1 {
			t.Fatal("planning changed population, people or RNG")
		}
		if cost := 1000 - w.Magnets[player].Mana; cost != 14 {
			t.Fatalf("lowering cost=%d", cost)
		}
		life := w.checkLife(player, pos)
		if life <= 0 || life >= CityFood {
			t.Fatalf("temporary capacity=%d", life)
		}
		w.GameTurn = 8
		if w.advancedRestoreColonistLand(player) {
			t.Fatal("restored before normal growth tick")
		}
		w.processTownWithLandAI(0, false)
		if len(w.Peeps) != 2 || w.Peeps[1].Flags != OnMove {
			t.Fatal("ordinary rules did not produce the emigrant")
		}
		if w.Peeps[0].Population+w.Peeps[1].Population != 700+w.Rules.PopulationAdd[w.Peeps[0].Frame-FirstTown] {
			t.Fatal("population not conserved except normal growth")
		}
		w.GameTurn = 9
		income := w.Rules.ManaAdd[w.Peeps[0].Frame-FirstTown]
		if !w.advancedRestoreColonistLand(player) || w.Computer[player].Arrived != 0 {
			t.Fatal("paid restoration failed")
		}
		if w.Alt != beforeAlt || w.checkLife(player, pos) != CityFood || w.Magnets[player].Mana != 1000-28+income {
			t.Fatal("terrain or mana was not restored through normal paid actions")
		}
	}
}

func TestAdvancedColonistProjectSurvivesSnapshot(t *testing.T) {
	w := colonistReleaseWorld(0)
	if !w.advancedReleaseColonists(0) {
		t.Fatal("no project")
	}
	other := WorldFromSnapshot(w.Snapshot(), w.Rules)
	for i := 0; i < 32; i++ {
		w.TickWithAdvancedComputer(0)
		other.TickWithAdvancedComputer(0)
		if w.StateHash() != other.StateHash() {
			t.Fatalf("resumption diverged at %d", i)
		}
	}
}

func TestAdvancedColonistsRespectConstructionRestrictions(t *testing.T) {
	for _, mode := range []byte{GameOnlyRaise, GameNoBuild} {
		w := colonistReleaseWorld(0)
		w.Level.GameMode = mode
		before := w.StateHash()
		if w.advancedReleaseColonists(0) || w.StateHash() != before {
			t.Fatal("illegal release changed world")
		}
	}
	w := colonistReleaseWorld(0)
	w.Magnets[0].Mana = 10
	before := w.StateHash()
	if w.advancedReleaseColonists(0) || w.StateHash() != before {
		t.Fatal("unfunded release changed world")
	}
}

func TestAdvancedColonistRestoreWaitsForTownTickAndActionSlot(t *testing.T) {
	w := colonistReleaseWorld(0)
	w.Computer[0].Speed = 1
	if !w.advancedReleaseColonists(0) {
		t.Fatal("no release")
	}
	for _, tick := range []int{6, 7, 8} {
		w.GameTurn = tick
		before := w.StateHash()
		w.runAdvancedComputerPlayer(0)
		if w.StateHash() != before {
			t.Fatalf("restored before normal town tick %d", tick)
		}
	}
	w.GameTurn = 9
	w.runAdvancedComputerPlayer(0)
	if w.Computer[0].Arrived != 0 || w.Computer[0].DoneTurn != 9 || w.Magnets[0].Mana != 972 {
		t.Fatal("restoration did not consume the next paid action slot")
	}
	before := w.StateHash()
	w.runAdvancedComputerPlayer(0)
	if w.StateHash() != before {
		t.Fatal("second command used a closed action slot")
	}
}

func TestAdvancedColonistRestoreCancelsAfterLosingPresence(t *testing.T) {
	w := colonistReleaseWorld(0)
	w.advancedReleaseColonists(0)
	w.Peeps[0].Population = 0
	w.GameTurn = 9
	beforeAlt, beforeMana := w.Alt, w.Magnets[0].Mana
	if w.advancedRestoreColonistLand(0) || w.Computer[0].Arrived != 0 || w.Alt != beforeAlt || w.Magnets[0].Mana != beforeMana {
		t.Fatal("orphaned restoration modified terrain or stayed pending")
	}
}

func TestAdvancedColonistSafetyProtectsOtherTownFoodAndWalkers(t *testing.T) {
	w := colonistReleaseWorld(0)
	pos := 10 + 10*MapWidth
	w.Peeps = append(w.Peeps, Peep{Player: 0, Population: 200, Flags: InTown, AtPos: pos})
	w.MapWho[pos] = 2
	trial := *w
	trial.Alt[10+10*EndWidth] = 0
	trial.makeMap(0, 0, MapWidth-1, MapHeight-1)
	if w.advancedReleaseSafe(&trial, 0, 0) {
		t.Fatal("release accepted collateral damage to another town")
	}
	w.Peeps[1].Flags = OnMove
	for _, corner := range [...]int{0, 1, EndWidth, EndWidth + 1} {
		trial.Alt[10+10*EndWidth+corner] = 0
	}
	trial.makeMap(0, 0, MapWidth-1, MapHeight-1)
	if w.advancedReleaseSafe(&trial, 0, 0) {
		t.Fatal("release accepted a drowned walker")
	}
}
