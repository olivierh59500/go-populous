package populous

import "testing"

func advancedEmergencyWaterWorld() *World {
	w := &World{Rules: DefaultTerrainRules(), ComputerControlled: [2]bool{true, true}}
	for player := range w.Computer {
		w.Computer[player] = ComputerStats{Mode: computerLand, Speed: 3, Skill: 5}
		w.Magnets[player] = Magnet{Mana: 1000, Flags: SettleMode}
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	return w
}

func TestAdvancedEmergencyRescuesSwimmerWithPaidLocalAction(t *testing.T) {
	w := advancedEmergencyWaterWorld()
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove | InWater, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	beforePeep, beforeRNG := w.Peeps[0], w.rng
	if !w.advancedEmergencyLand(GodPlayer) {
		t.Fatal("strategic swimmer was not rescued")
	}
	if w.MapBlk[pos] == WaterBlock {
		t.Fatal("paid terrain action left the swimmer in water")
	}
	if got := w.Magnets[GodPlayer].Mana; got != 1000-ManaPointCost-4 {
		t.Fatalf("rescue mana=%d, want one changed vertex cost", got)
	}
	if w.Peeps[0] != beforePeep || w.rng != beforeRNG || len(w.SoundEvents) != 0 {
		t.Fatal("terrain rescue moved a follower, consumed RNG, or queued audio")
	}
}

func TestAdvancedEmergencyPaysPropagatedTerrainCost(t *testing.T) {
	w := advancedEmergencyWaterWorld()
	x, y := 20, 20
	pos := x + y*MapWidth
	for yy := y; yy <= y+1; yy++ {
		for xx := x; xx <= x+1; xx++ {
			w.Alt[xx+yy*EndWidth] = 1
		}
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	// Treat the occupied tile as freshly flooded. Every rescue corner is already
	// above the surrounding sea, so raising it propagates to several neighbours.
	w.MapBlk[pos] = WaterBlock
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove | InWater, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	beforeAlt, beforeMana := w.Alt, w.Magnets[GodPlayer].Mana
	if !w.advancedEmergencyLand(GodPlayer) {
		t.Fatal("propagated swimmer rescue was rejected")
	}
	changed := 0
	for i := range w.Alt {
		if w.Alt[i] != beforeAlt[i] {
			changed++
		}
	}
	if changed <= 1 {
		t.Fatalf("fixture changed %d vertices, want propagated terrain work", changed)
	}
	if spent := beforeMana - w.Magnets[GodPlayer].Mana; spent != ManaPointCost+4*changed {
		t.Fatalf("propagated rescue cost=%d, want %d for %d vertices", spent, ManaPointCost+4*changed, changed)
	}
}

func TestAdvancedEmergencyPreventsEntryIntoFatalWater(t *testing.T) {
	w := advancedEmergencyWaterWorld()
	w.Level.GameMode = GameWaterFatal
	pos := 20 + 20*MapWidth
	// A flood changes the tile before processTown adds InWater. At this point an
	// ordinary paid raise can still prevent the fatal state transition.
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	if !w.advancedEmergencyLand(GodPlayer) || w.MapBlk[pos] == WaterBlock {
		t.Fatal("freshly flooded follower was not rescued before fatal-water processing")
	}
}

func TestAdvancedEmergencyOpensPassageOnlyForClearlyBlockedSettler(t *testing.T) {
	t.Run("blocked by water", func(t *testing.T) {
		w := advancedEmergencyWaterWorld()
		pos := 20 + 20*MapWidth
		w.MapBlk[pos] = BadLand
		w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 150, AtPos: pos}}
		w.MapWho[pos] = 1
		if !w.advancedEmergencyLand(GodPlayer) {
			t.Fatal("isolated settler did not build a passage")
		}
		if got := w.Magnets[GodPlayer].Mana; got != 1000-ManaPointCost-4 {
			t.Fatalf("passage mana=%d, want one changed vertex cost", got)
		}
		if w.advancedSettlerClearlyBlocked(0) {
			t.Fatal("paid action did not open a traversable adjacent tile")
		}
	})

	t.Run("normal route exists", func(t *testing.T) {
		w := advancedEmergencyWaterWorld()
		pos := 20 + 20*MapWidth
		w.MapBlk[pos] = BadLand
		w.MapBlk[pos+1] = FlatBlock
		w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 150, AtPos: pos}}
		w.MapWho[pos] = 1
		before := w.StateHash()
		if w.advancedEmergencyLand(GodPlayer) || w.StateHash() != before {
			t.Fatal("emergency terrain replaced an available normal route")
		}
	})
}

func TestAdvancedEmergencyClearsOnlySwampExit(t *testing.T) {
	w := advancedTestWorld()
	w.Level.GameMode = GameOnlyRaise
	pos := 40 + 40*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 150, AtPos: pos}}
	w.MapWho[pos] = 1
	w.MapBlk[pos] = BadLand
	for _, delta := range toOffset {
		w.MapBlk[pos+delta] = RockBlock
	}
	target := pos + 1
	w.MapBlk[target] = SwampBlock
	before := w.Magnets[GodPlayer].Mana
	if !w.advancedEmergencyLand(GodPlayer) {
		t.Fatal("settler with only a swamp exit was not helped")
	}
	if w.MapBlk[target] == SwampBlock || w.validMove(pos, 1) != 0 {
		t.Fatal("emergency action did not clear the swamp exit")
	}
	if spent := before - w.Magnets[GodPlayer].Mana; spent != ManaPointCost+4 {
		t.Fatalf("swamp passage cost=%d, want %d", spent, ManaPointCost+4)
	}
}

func TestAdvancedEmergencyHonoursTownOnlyConstruction(t *testing.T) {
	w := advancedTestWorld()
	w.Level.GameMode = GameRaiseTown
	pos := 40 + 40*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 150, AtPos: pos}}
	w.MapWho[pos] = 1
	w.MapBlk[pos] = BadLand
	for _, delta := range toOffset {
		w.MapBlk[pos+delta] = RockBlock
	}
	w.MapBlk[pos+1] = SwampBlock
	before := w.StateHash()
	if w.advancedEmergencyLand(GodPlayer) || w.StateHash() != before {
		t.Fatal("walker built above sea level in town-only construction mode")
	}
}

func TestAdvancedEmergencyHonoursRestrictionsAndAffordableFullCost(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       byte
		computer   int
		mana       int
		presence   bool
		fatalWater bool
		war        bool
	}{
		{name: "no build", mode: GameNoBuild, computer: computerLand, mana: 1000, presence: true},
		{name: "land disabled", computer: computerTown, mana: 1000, presence: true},
		{name: "full cost unaffordable", computer: computerLand, mana: ManaPointCost, presence: true},
		{name: "no local presence", computer: computerLand, mana: 1000},
		{name: "fatal swimmer cannot be saved", computer: computerLand, mana: 1000, presence: true, fatalWater: true},
		{name: "armageddon", computer: computerLand, mana: 1000, presence: true, war: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedEmergencyWaterWorld()
			w.Level.GameMode = tc.mode
			if tc.fatalWater {
				w.Level.GameMode |= GameWaterFatal
			}
			w.War = tc.war
			w.Computer[GodPlayer].Mode = tc.computer
			w.Magnets[GodPlayer].Mana = tc.mana
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove | InWater, Population: 200, AtPos: pos}}
			if tc.presence {
				w.MapWho[pos] = 1
			}
			before := w.StateHash()
			if w.advancedEmergencyLand(GodPlayer) || w.StateHash() != before {
				t.Fatal("emergency action ignored a normal terrain restriction")
			}
		})
	}
}

func TestAdvancedEmergencyPreservesFriendlyTown(t *testing.T) {
	w := advancedTestWorld()
	townPos := 20 + 20*MapWidth
	settlerPos := 23 + 20*MapWidth
	w.Peeps = []Peep{
		{Player: GodPlayer, Flags: InTown, Population: 300, AtPos: townPos},
		{Player: GodPlayer, Flags: OnMove, Population: 150, AtPos: settlerPos},
	}
	w.MapWho[townPos], w.MapWho[settlerPos] = 1, 2
	w.setTown(0, false)
	w.MapBlk[settlerPos] = BadLand
	for _, delta := range toOffset {
		w.MapBlk[settlerPos+delta] = RockBlock
	}
	// The planner may clear this soft obstacle only through a corner whose net
	// terrain change preserves the neighbouring town's food supply.
	target := settlerPos - 1
	w.MapBlk[target] = SwampBlock
	beforeLife := w.checkLife(GodPlayer, townPos)
	if !w.advancedEmergencyLand(GodPlayer) {
		t.Fatal("safe emergency passage was not built")
	}
	if got := w.checkLife(GodPlayer, townPos); got < beforeLife {
		t.Fatalf("emergency passage reduced friendly food from %d to %d", beforeLife, got)
	}
}

func TestAdvancedEmergencyRejectsCandidateThatDamagesFriendlyTown(t *testing.T) {
	w := advancedTestWorld()
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 300, AtPos: pos}}
	w.MapWho[pos] = 1
	w.setTown(0, false)
	trial := *w
	if !trial.advancedSculpt(GodPlayer, 20, 20, true) {
		t.Fatal("fixture terrain action was rejected")
	}
	if trial.checkLife(GodPlayer, pos) >= w.checkLife(GodPlayer, pos) {
		t.Fatal("fixture terrain action does not damage the town")
	}
	if w.advancedEmergencyFriendlySafe(&trial, GodPlayer) {
		t.Fatal("emergency safety check accepted damage to a friendly town")
	}
}
