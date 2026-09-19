package populous

import (
	"fmt"
	"testing"
)

func legacyCombatWorld() *World {
	w := &World{Rules: DefaultTerrainRules()}
	for i := range w.MapBlk {
		w.MapBlk[i] = FlatBlock
	}
	return w
}

// C++ populous_battle.cpp join_battle has no friendly-town exclusion, unlike
// join_forces. A knight can reinforce a town that is already defending itself.
func TestLegacyKnightReinforcesTownInBattle(t *testing.T) {
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyCombatWorld()
			w.Peeps = []Peep{
				{Player: byte(player), Flags: OnMove, Population: 1000, AtPos: 650, HeadFor: 3, Status: 1, IQ: 4, Weapons: 5},
				{Player: byte(player), Flags: InTown | InBattle, Population: 100, AtPos: 650, BattlePopulation: 2, Frame: BattleFirstFrame + 1, IQ: 1, Weapons: 2, Status: 2},
				{Player: byte(player ^ 1), Flags: InBattle, Population: 500, AtPos: 650, BattlePopulation: 1, Frame: BattleFirstFrame + 1},
			}
			w.MapWho[650] = 3
			w.moveExplorer(0)
			if w.Peeps[0].Population != 0 || w.Peeps[1].Population != 1100 || w.Peeps[1].HeadFor != 3 {
				t.Fatalf("knight did not reinforce friendly town: %+v", w.Peeps)
			}
			if town := w.Peeps[1]; town.Flags != InTown|InBattle || town.BattlePopulation != 2 || town.Frame != BattleFirstFrame+1 || town.IQ != 1 || town.Weapons != 5 || town.Status != 2 {
				t.Fatalf("reinforcement overwrote battle or surviving-group state: %+v", town)
			}
			if w.MapWho[650] != 3 {
				t.Fatalf("reinforcement replaced the visible battle participant: %d", w.MapWho[650])
			}
		})
	}
}

func TestLegacyBattleReinforcementCapsPopulationAndPreservesAnimation(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, town := range []bool{false, true} {
			t.Run(fmt.Sprintf("player%d/town%t", player, town), func(t *testing.T) {
				w := legacyCombatWorld()
				flags := byte(InBattle | OnMove)
				if town {
					flags = InBattle | InTown
				}
				w.Peeps = []Peep{
					{Player: byte(player), Flags: OnMove, Population: 1000, AtPos: 650, IQ: 4, Weapons: 5, HeadFor: 3, Status: 1},
					{Player: byte(player), Flags: flags, Population: 31900, AtPos: 650, IQ: 1, Weapons: 2, Frame: BattleFirstFrame + 2, BattlePopulation: 2},
					{Player: byte(player ^ 1), Flags: InBattle, Population: 500, AtPos: 650, BattlePopulation: 1},
				}
				w.Magnets[player].Carried = 1
				w.Magnets[player].Population = 1000
				w.joinBattle(0, 2)
				if got := w.Peeps[1]; got.Population != 32000 || got.Flags != flags || got.Frame != BattleFirstFrame+2 || got.BattlePopulation != 2 || got.IQ != 1 || got.Weapons != 5 || got.HeadFor != 3 || got.Status != 0 {
					t.Fatalf("wrong surviving reinforcement state: %+v", got)
				}
				if w.Magnets[player].Carried != 2 || w.Magnets[player].Population != 0 || w.Peeps[0].Population != 0 {
					t.Fatalf("reinforcement references/counts: magnet=%+v source=%+v", w.Magnets[player], w.Peeps[0])
				}
			})
		}
	}
}

func TestLegacyFriendlyMergeRetainsSurvivorIntelligenceAndStatus(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, sourceIndex := range []int{0, 1} {
			t.Run(fmt.Sprintf("player%d/source%d", player, sourceIndex), func(t *testing.T) {
				w := legacyCombatWorld()
				targetIndex := sourceIndex ^ 1
				w.Peeps = make([]Peep, 3)
				w.Peeps[sourceIndex] = Peep{Player: byte(player), Flags: OnMove, Population: 100, AtPos: 650, IQ: 4, Weapons: 5, HeadFor: 3, Status: 1}
				w.Peeps[targetIndex] = Peep{Player: byte(player), Flags: OnMove | WaitForMe, Population: 200, AtPos: 650, IQ: 1, Weapons: 2, Frame: FirstWaitSprite, Status: 2}
				w.Magnets[player] = Magnet{Carried: sourceIndex + 1, Population: 300}
				w.MapWho[650] = byte(targetIndex + 1)
				w.joinForces(sourceIndex, targetIndex)
				if got := w.Peeps[targetIndex]; got.Population != 300 || got.IQ != 1 || got.Status != 2 || got.HeadFor != 3 || got.Weapons != 5 || got.Flags != OnMove || got.Frame != 0 {
					t.Fatalf("merge differs from original surviving-group rules: %+v", got)
				}
				wantCachedPopulation := 300
				if targetIndex > sourceIndex {
					wantCachedPopulation -= 100
				}
				if w.Magnets[player].Population != wantCachedPopulation || w.Magnets[player].Carried != targetIndex+1 || w.MapWho[650] != byte(targetIndex+1) {
					t.Fatalf("wrong merge accounting/references: %+v, map=%d", w.Magnets[player], w.MapWho[650])
				}
			})
		}
	}
}

func TestLegacyKnightTownMergeExclusionUsesExactFlags(t *testing.T) {
	for _, flags := range []byte{InTown, InTown | InEffect} {
		w := legacyCombatWorld()
		w.Peeps = []Peep{
			{Player: GodPlayer, Flags: OnMove, Population: 100, AtPos: 650, HeadFor: 3},
			{Player: GodPlayer, Flags: flags, Population: 200, AtPos: 650},
		}
		w.joinForces(0, 1)
		wantSource, wantTarget := 0, 300
		if flags == InTown {
			wantSource, wantTarget = 100, 200
		}
		if w.Peeps[0].Population != wantSource || w.Peeps[1].Population != wantTarget {
			t.Fatalf("town flags %d: populations=%d/%d want=%d/%d", flags, w.Peeps[0].Population, w.Peeps[1].Population, wantSource, wantTarget)
		}
	}
}

func TestLegacyBattleDrawsDefenderRandomBeforeAttacker(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, seed := range []lcg{0, 2, 5, 8, 12345, 32767} {
			w := legacyCombatWorld()
			w.rng = seed
			w.Peeps = []Peep{
				{Player: byte(player), Flags: InBattle, Population: 1000, Weapons: 3, AtPos: 650, BattlePopulation: 1},
				{Player: byte(player ^ 1), Flags: InBattle | OnMove, Population: 2000, Weapons: 5, AtPos: 650},
			}
			// Literal evaluation order of populous_battle.cpp do_battle.
			referenceRNG := seed
			defenderPower := 2000 * (referenceRNG.next()%3 + 1)
			attackerPower := 1000 * (referenceRNG.next()%3 + 1)
			power := min(defenderPower, attackerPower)
			wantAttacker, wantDefender := 1000-(power/100)*5-10, 2000-(power/100)*3-10
			w.doBattle(0)
			if w.Peeps[0].Population != wantAttacker || w.Peeps[1].Population != wantDefender || w.rng != referenceRNG {
				t.Fatalf("player%d seed%d: populations=%d/%d rng=%d; want=%d/%d rng=%d", player, seed, w.Peeps[0].Population, w.Peeps[1].Population, w.rng, wantAttacker, wantDefender, referenceRNG)
			}
			if seed == 2 && (wantAttacker != 940 || wantDefender != 1960) {
				t.Fatal("reference combat fixture changed")
			}
		}
	}
}

func TestLegacyBattleReportsFirstKnightVictoryAndLoss(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := legacyCombatWorld()
		w.Peeps = []Peep{
			{Player: byte(player), Flags: InBattle, Population: 1000, AtPos: 650, HeadFor: 3, Status: 1, BattlePopulation: 1},
			{Player: byte(player ^ 1), Flags: InBattle | OnMove, Population: -1, AtPos: 650, HeadFor: 1, Status: 1},
			{Player: byte(player ^ 1), Flags: OnMove, Population: 100, AtPos: 660},
		}
		w.Computer[player].Arrived = 7
		w.Computer[player^1].Arrived = 9
		w.Magnets[player^1].Mana = 5000
		w.battleOver(0, 1)
		if w.Computer[player].LastBattle != 650 || w.Computer[player].Arrived != 2 || w.Computer[player^1].Arrived != 0 || w.Peeps[0].Status != 0 {
			t.Fatalf("victory did not update original AI state: winner=%+v loser=%+v peep=%+v", w.Computer[player], w.Computer[player^1], w.Peeps[0])
		}
		if !isHeadedPeep(w.Peeps[0]) || w.Peeps[0].HeadFor != 3 || w.Peeps[0].Flags != OnMove|InEffect || w.Peeps[0].Frame != VictorySprite {
			t.Fatalf("first victory removed knight identity or animation: %+v", w.Peeps[0])
		}
		w.Computer[player].LastBattle = 700
		w.Computer[player].Arrived = 7
		w.Peeps[2].Population = -1
		w.battleOver(0, 2)
		if w.Computer[player].LastBattle != 700 || w.Computer[player].Arrived != 7 {
			t.Fatalf("later victory reset first-knight-victory state: %+v", w.Computer[player])
		}
	}
}

func TestLegacyCarrierDeathDropsFlagWithoutChargingManaOrChangingMode(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := legacyCombatWorld()
		w.Peeps = []Peep{{Player: byte(player), Flags: OnMove, Population: 100, AtPos: 650, Direction: 1}}
		w.Magnets[player] = Magnet{GoTo: 700, Carried: 1, Flags: FightMode, Mana: 1234}
		w.MapWho[649], w.MapWho[650] = 1, 1
		w.zeroPopulation(0)
		if got := w.Magnets[player]; got.GoTo != 650 || got.Carried != 0 || got.Flags != FightMode || got.Mana != 1234 {
			t.Fatalf("wrong dropped flag: %+v", got)
		}
		if w.MapWho[649] != 0 || w.MapWho[650] != 0 || w.Peeps[0].Population != 0 {
			t.Fatalf("carrier death left stale map references or population: %+v", w.Peeps[0])
		}
	}
}

func TestLegacyMagnetSwampAvoidanceDependsOnKnightHeading(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, war := range []bool{false, true} {
			for _, headed := range []bool{false, true} {
				w := legacyCombatWorld()
				w.War = war
				w.Peeps = []Peep{{Player: byte(player), Flags: OnMove, Population: 100, AtPos: 650, Status: KnightStatus}}
				if headed {
					w.Peeps[0].HeadFor = 2
				}
				w.Magnets[player] = Magnet{Carried: 1, Flags: MagnetMode, GoTo: 654}
				w.MapBlk[651] = SwampBlock
				delta := w.moveMagnetPeeps(0)
				if headed && delta == 1 || !headed && delta != 1 {
					t.Fatalf("player%d war%t headed%t: swamp movement delta=%d", player, war, headed, delta)
				}
			}
		}
	}
}

func legacyCompletedTownWorld(player, altitude int) *World {
	w := &World{Rules: DefaultTerrainRules(), GameTurn: 1}
	for i := range w.Alt {
		w.Alt[i] = altitude
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.ComputerControlled[player] = true
	w.Computer[player].Mode, w.Computer[player].Speed = computerLand, 1
	w.Magnets[player].Mana = 10000
	w.Peeps = []Peep{
		{Player: byte(player), Flags: InTown, Population: 1000, AtPos: 650, Frame: LastTown},
		{Player: byte(player ^ 1), Flags: OnMove, Population: 1000, AtPos: 660},
	}
	w.setTown(0, false)
	return w
}

// C++ uses one status byte for both make_level's completion result and the
// first-knight-battle notification. The Go split must preserve both meanings.
func TestLegacyCompletedTownReportsBattleOutcome(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, townWins := range []bool{false, true} {
			w := legacyCompletedTownWorld(player, 1)
			w.processTown(0)
			if !w.Peeps[0].LandComplete || w.Peeps[0].Status != 0 {
				t.Fatalf("fixture did not create a completed town: %+v", w.Peeps[0])
			}
			w.Computer[player].Arrived, w.Computer[player].LastBattle = 7, 700
			w.setBattle(1, 0)
			if townWins {
				w.Peeps[1].Population = -1
				w.battleOver(0, 1)
				if w.Computer[player].Arrived != 2 || w.Computer[player].LastBattle != 650 || w.Peeps[0].LandComplete || w.Peeps[0].Status != 0 {
					t.Fatalf("player%d town victory: stats=%+v town=%+v", player, w.Computer[player], w.Peeps[0])
				}
			} else {
				w.Peeps[0].Population = -1
				w.battleOver(1, 0)
				if w.Computer[player].Arrived != 0 {
					t.Fatalf("player%d completed-town loss left Arrived=%d", player, w.Computer[player].Arrived)
				}
			}
		}
	}
}

func TestLegacyKnightVictoryConsumesPreviousTownCompletion(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := legacyCompletedTownWorld(player, 1)
		w.processTown(0)
		w.Computer[player].Mode |= computerKnight
		w.Magnets[player].Carried = 1
		if !w.Peeps[0].LandComplete || !w.Knight(player) {
			t.Fatal("fixture could not convert a completed town's carrier")
		}
		w.setBattle(0, 1)
		w.Peeps[1].Population = -1
		w.battleOver(0, 1)
		if w.Peeps[0].Status != 0 || w.Peeps[0].LandComplete || w.Computer[player].Arrived != 2 {
			t.Fatalf("converted knight retained a second battle notification: %+v", w.Peeps[0])
		}
	}
}

func TestLegacyLowTownLevelResultReplacesPriorStatus(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, complete := range []bool{false, true} {
			w := legacyCompletedTownWorld(player, 1)
			w.Peeps[0].Status, w.Peeps[0].LandComplete = 1, true
			if !complete {
				// Outside the town footprint but inside make_level's 9x9
				// area: a legal terrain repair is still needed.
				w.Alt[14+14*EndWidth]++
				w.makeMap(13, 13, 14, 14)
			}
			// The restored flat tile triggers original a_flat_block even
			// though the town previously carried a nonzero status marker.
			w.MapBlk[650] = FlatBlock
			w.processTown(0)
			if w.Peeps[0].Status != 0 || w.Peeps[0].LandComplete != complete {
				t.Fatalf("player%d complete%t: level result did not replace prior status: %+v", player, complete, w.Peeps[0])
			}
		}
	}
}

func TestLegacyHighTownIgnoresLevelResultAndPreservesStatus(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, complete := range []bool{false, true} {
			for _, status := range []int{0, 1} {
				w := legacyCompletedTownWorld(player, 2)
				w.GameTurn = 300
				w.Peeps[0].Status, w.Peeps[0].LandComplete = status, complete
				w.processTown(0)
				if w.Peeps[0].Status != status || w.Peeps[0].LandComplete != complete {
					t.Fatalf("player%d high town stored an ignored make_level result: %+v", player, w.Peeps[0])
				}
			}
		}
	}
}
