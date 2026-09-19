package populous

import (
	"fmt"
	"os"
	"testing"
)

func TestOriginalComputerReference(t *testing.T) {
	// Engine-3 regression baselines, after restoring C++ initialization, turn
	// ordering, attrition, combat and the original AI's missing behaviours.
	// These detect changes in Go replays; the independent C++ oracle checks
	// original-source parity rather than treating these Go hashes as proof.
	for seed, want := range map[uint16]string{
		25:    "d79591ad876f40acab6065de2e9641ed22f3125b3016491df9cb1bd96fc7dcbb",
		27068: "d27f8c8030efae8a84bed97e14a343107ac0680cc86a6981ea3c3a5628b7b829",
		4321:  "2e65e3c342d51612ea5f5381ec7e918de79289253e73ec2cea8892640f54988b",
	} {
		w := GenerateWorld(Level{SeedOffset: seed, PlayerPopulation: 10, EnemyPopulation: 10, PlayerPowers: 0x3f, EnemyPowers: 0x3f, EnemyRating: 5, EnemyReactionSpeed: 3})
		for tick := 0; tick < 512; tick++ {
			w.TickWithComputer([2]bool{true, true})
		}
		if got := fmt.Sprintf("%x", w.StateHash()); got != want {
			t.Errorf("seed %d original AI reference simulation changed: %s, want %s", seed, got, want)
		}
	}
}

func TestAdvancedComputerSeededEconomyObservation(t *testing.T) {
	if os.Getenv("POPULOUS_AI_ECONOMY_SAMPLE") == "" {
		t.Skip("optional economic observation; real victories are measured by the campaign audit")
	}
	advancedTotal, originalTotal, ahead := 0, 0, 0
	for _, seed := range []uint16{25, 27068, 4321, 12345} {
		for player := 0; player < 2; player++ {
			w := GenerateWorld(Level{SeedOffset: seed, PlayerPopulation: 10, EnemyPopulation: 10, PlayerPowers: 0x3f, EnemyPowers: 0x3f, EnemyRating: 5, EnemyReactionSpeed: 3})
			w.Magnets[0].Mana, w.Magnets[1].Mana = 11000, 11000
			for tick := 0; tick < 2400 && w.ResultFor(0) == ResultOngoing; tick++ {
				w.TickWithAdvancedComputer(player)
			}
			pop := w.PlayerPopulations()
			advancedTotal += pop[player]
			originalTotal += pop[player^1]
			if pop[player] > pop[player^1] {
				ahead++
			}
			t.Logf("seed=%d advanced=%d turns=%d populations=%v mana=%d/%d castles=%d/%d", seed, player, w.GameTurn, pop, w.Magnets[0].Mana, w.Magnets[1].Mana, w.Computer[0].NoCastles, w.Computer[1].NoCastles)
		}
	}
	// Economic observation only: a successful expedition can sacrifice growth
	// to eliminate the enemy. Population is deliberately not a pass criterion.
	t.Logf("advanced population leads %d/8; total advanced=%d original=%d", ahead, advancedTotal, originalTotal)
}

func TestAdvancedComputerDeterministicAcrossSnapshot(t *testing.T) {
	for _, player := range []int{GodPlayer, DevilPlayer, -1} {
		level := Level{SeedOffset: 27068, PlayerPopulation: 10, EnemyPopulation: 10, PlayerPowers: 0x3f, EnemyPowers: 0x3f, EnemyRating: 5, EnemyReactionSpeed: 3}
		left, right := GenerateWorld(level), GenerateWorld(level)
		for tick := 0; tick < 1024; tick++ {
			left.TickWithAdvancedComputer(player)
			if player < 0 {
				right.TickWithComputer([2]bool{true, true})
			} else {
				right.TickWithAdvancedComputer(player)
			}
			if tick == 511 {
				right = WorldFromSnapshot(right.Snapshot(), right.Rules)
			}
			if tick%64 == 63 && left.StateHash() != right.StateHash() {
				t.Fatalf("strategy %d diverged at tick %d", player, tick)
			}
		}
	}
}

func advancedTestWorld() *World {
	w := &World{Rules: DefaultTerrainRules(), GameTurn: 1, ComputerControlled: [2]bool{true, true}}
	for i := range w.Alt {
		w.Alt[i] = 1
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	for player := range w.Computer {
		w.Computer[player] = ComputerStats{Mode: computerLand | computerTown | computerLeader, Speed: 3, Skill: 5}
		w.Magnets[player] = Magnet{Mana: 1000, Flags: SettleMode}
	}
	return w
}

func TestAdvancedLandImpossibleFootprintsDoNotStarveViableTown(t *testing.T) {
	for _, gameMode := range []byte{0, GameRaiseTown} {
		w := advancedTestWorld()
		w.Level.GameMode = gameMode
		if gameMode == GameRaiseTown {
			// Equal intermediate production makes a hard-rock footprint a dead
			// end even in the town-only construction rule.
			for stage := 2; stage <= 9; stage++ {
				w.Rules.ManaAdd[stage] = 3
				w.Rules.PopulationAdd[stage] = 1
				w.Rules.WeaponsAdd[stage] = 3
			}
		}
		for _, pos := range []int{10 + 10*MapWidth, 20 + 10*MapWidth, 30 + 10*MapWidth, 40 + 10*MapWidth, 50 + 40*MapWidth} {
			w.Peeps = append(w.Peeps, Peep{Player: 0, Flags: InTown, Population: 100, AtPos: pos})
			w.MapWho[pos] = byte(len(w.Peeps))
		}
		w.Alt[52+40*EndWidth] = 2
		w.makeMap(0, 0, MapWidth-1, MapHeight-1)
		for _, p := range w.Peeps[:4] {
			w.MapBlk[p.AtPos+2] = RockBlock
		}
		pos := w.Peeps[4].AtPos
		if w.checkLife(0, w.Peeps[0].AtPos) <= w.checkLife(0, pos) {
			t.Fatal("fixture does not fill the old shortlist with impossible towns")
		}
		if !w.advancedLand(0) || w.checkLife(0, pos) != CityFood {
			t.Fatal("four impossible projects starved the viable fifth town")
		}
	}
}

func TestAdvancedComputerTerrainFinishesCastleLegally(t *testing.T) {
	w := advancedTestWorld()
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	w.SoundEvents = []int{TuneMagnet}
	w.rng = lcg(1234)
	peepBefore := w.Peeps[0]
	w.Alt[22+20*EndWidth] = 2
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	if w.checkLife(GodPlayer, pos) >= CityFood {
		t.Fatal("test setup is already a castle")
	}
	w.runAdvancedComputerPlayer(GodPlayer)
	if got := w.checkLife(GodPlayer, pos); got != CityFood {
		t.Fatalf("food after terrain action = %d, want castle %d", got, CityFood)
	}
	if got := w.Magnets[GodPlayer].Mana; got != 1000-ManaPointCost-4 {
		t.Fatalf("terrain mana = %d, want legal single-point cost", got)
	}
	if w.Computer[GodPlayer].DoneTurn != w.GameTurn {
		t.Fatal("terrain edit did not consume action slot")
	}
	if w.Peeps[0] != peepBefore || w.rng != lcg(1234) || len(w.SoundEvents) != 1 || w.SoundEvents[0] != TuneMagnet {
		t.Fatal("terrain lookahead modified people, RNG or queued audio")
	}
	hash := w.StateHash()
	w.runAdvancedComputerPlayer(GodPlayer)
	if w.StateHash() != hash {
		t.Fatal("AI took a second action in the same slot")
	}
}

func TestAdvancedComputerAbandonsHardRockCastleFootprint(t *testing.T) {
	w := advancedTestWorld()
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	w.MapBlk[pos+2] = RockBlock
	w.Alt[22+20*EndWidth] = 2 // A tempting paid edit cannot remove hard rock.
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.MapBlk[pos+2] = RockBlock
	before := w.StateHash()
	if w.advancedLand(GodPlayer) || w.StateHash() != before {
		t.Fatal("AI spent mana on a castle footprint blocked by permanent rock")
	}
}

func TestAdvancedComputerImprovesReachableHardRockTownOnlyWorld(t *testing.T) {
	w := advancedTestWorld()
	w.Level.GameMode = GameRaiseTown
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	w.Alt[20+18*EndWidth] = 2
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.MapBlk[pos+2] = RockBlock
	beforeLife := w.checkLife(GodPlayer, pos)
	if !w.advancedLand(GodPlayer) {
		t.Fatal("AI abandoned its only legal town-only construction base")
	}
	if got := w.checkLife(GodPlayer, pos); got <= beforeLife {
		t.Fatalf("town food=%d, want improvement above %d", got, beforeLife)
	}
	if w.MapBlk[pos+2] != RockBlock {
		t.Fatal("AI removed permanent rock without lowering it to sea level")
	}
}

func TestAdvancedComputerRespectsPowerManaAndActionGate(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		mode, mana, doneTurn int
		wantCast             bool
	}{
		{"locked power", computerLand, 100000, 0, false},
		{"insufficient mana", computerWar, ManaWarCost - 1, 0, false},
		{"closed action slot", computerWar, ManaWarCost + 1000, 1, false},
		{"legal war", computerWar, ManaWarCost + 1000, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedTestWorld()
			w.Computer[GodPlayer].Mode = tc.mode
			w.Computer[GodPlayer].DoneTurn = tc.doneTurn
			w.Magnets[GodPlayer].Mana = tc.mana
			w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 200, AtPos: 20 + 20*MapWidth}, {Player: DevilPlayer, Flags: OnMove, Population: 50, AtPos: 40 + 40*MapWidth}}
			w.runAdvancedComputerPlayer(GodPlayer)
			if w.War != tc.wantCast {
				t.Fatalf("war=%v, want %v", w.War, tc.wantCast)
			}
			wantMana := tc.mana
			if tc.wantCast {
				wantMana -= ManaWarCost
			}
			if w.Magnets[GodPlayer].Mana != wantMana {
				t.Fatalf("mana=%d, want %d", w.Magnets[GodPlayer].Mana, wantMana)
			}
		})
	}
}

func TestAdvancedComputerAvoidsFriendlyFire(t *testing.T) {
	w := advancedTestWorld()
	w.Computer[GodPlayer].Mode |= computerVolcano | computerQuake
	w.Magnets[GodPlayer].Mana = 20000
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: InTown, Population: 1500, AtPos: 20 + 20*MapWidth},
		{Player: GodPlayer, Flags: InTown, Population: 5000, AtPos: 23 + 20*MapWidth},
	}
	before := w.StateHash()
	if w.advancedPower(GodPlayer) || w.StateHash() != before {
		t.Fatal("AI attacked a target surrounded by more valuable friendly people")
	}
	w.Peeps[1].AtPos = 50 + 50*MapWidth
	if !w.advancedPower(GodPlayer) || w.Magnets[GodPlayer].Mana != 20000-ManaQuakeCost {
		t.Fatal("AI did not prefer a cheaper quake against the isolated enemy town")
	}
	w = advancedTestWorld()
	w.Computer[GodPlayer].Mode |= computerVolcano | computerQuake
	w.Magnets[GodPlayer].Mana = 20000
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: InTown, Population: 1500, AtPos: 20 + 20*MapWidth},
		{Player: DevilPlayer, Flags: InTown, Population: 5000, AtPos: 22 + 22*MapWidth},
	}
	if !w.advancedPower(GodPlayer) || w.Magnets[GodPlayer].Mana != 20000-ManaVolcanoCost {
		t.Fatal("AI did not reserve volcano for a dense high-value enemy cluster")
	}
}

func TestAdvancedComputerHonoursTerrainRestrictions(t *testing.T) {
	for _, mode := range []byte{GameNoBuild, GameOnlyRaise} {
		w := advancedTestWorld()
		w.Level.GameMode = mode
		w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: 20 + 20*MapWidth}}
		w.Alt[22+20*EndWidth] = 2
		w.makeMap(0, 0, MapWidth-1, MapHeight-1)
		before := w.StateHash()
		w.runAdvancedComputerPlayer(GodPlayer)
		if w.StateHash() != before {
			t.Fatalf("AI changed terrain despite restriction %d", mode)
		}
	}
}

func TestAdvancedComputerDoesNotRaiseAPlateauForFlood(t *testing.T) {
	w := advancedTestWorld()
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	w.Magnets[GodPlayer].Mana = 5000
	w.Computer[DevilPlayer].Mode |= computerFlood
	before := w.StateHash()
	if w.advancedLand(GodPlayer) || w.StateHash() != before {
		t.Fatal("AI spent mana raising an already productive plateau for a possible flood")
	}
	for _, altitude := range w.Alt {
		if altitude != 1 {
			t.Fatal("AI introduced a forced target altitude")
		}
	}
}

func TestAdvancedComputerCancelsRetiredPlateauPlanOnResume(t *testing.T) {
	w := advancedTestWorld()
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	w.Magnets[GodPlayer].Mana = 5000
	w.Alt[22+20*EndWidth] = 2
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.Computer[GodPlayer].Arrived = advancedRetiredPlateauPlan + 2
	w.Computer[GodPlayer].LastBattle = pos
	resumed := WorldFromSnapshot(w.Snapshot(), w.Rules)
	if !w.advancedLand(GodPlayer) || !resumed.advancedLand(GodPlayer) {
		t.Fatal("AI did not resume with an ordinary current-altitude terrain action")
	}
	if w.StateHash() != resumed.StateHash() {
		t.Fatal("retired plan cancellation diverged after snapshot")
	}
	if w.Computer[GodPlayer].Arrived >= advancedRetiredPlateauPlan && w.Computer[GodPlayer].Arrived < advancedRepairPlan {
		t.Fatal("retired level-two project remained active")
	}
	if w.Alt[20+20*EndWidth] != 1 || w.checkLife(GodPlayer, pos) != CityFood {
		t.Fatal("AI did not finish the castle at its existing altitude")
	}
}

func TestAdvancedComputerRepairsCheapRocksAtExistingHeight(t *testing.T) {
	for _, altitude := range []int{3, 4} {
		for _, block := range []int{RockBlock, RockBlock + 1, RockBlock + 2, SwampBlock, BadLand} {
			t.Run(fmt.Sprintf("alt%d_block%d", altitude, block), func(t *testing.T) {
				w := advancedTestWorld()
				for i := range w.Alt {
					w.Alt[i] = altitude
				}
				w.makeMap(0, 0, MapWidth-1, MapHeight-1)
				pos := 20 + 20*MapWidth
				w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
				w.MapWho[pos] = 1
				w.MapBlk[pos+2] = byte(block)
				before := w.StateHash()
				changed := w.advancedRepair(GodPlayer)
				if block == RockBlock {
					if changed || w.StateHash() != before {
						t.Fatal("AI attempted to excavate hard rock")
					}
					return
				}
				if !changed {
					t.Fatal("AI ignored an affordable soft obstacle")
				}
				resumed := WorldFromSnapshot(w.Snapshot(), w.Rules)
				w.advancedFinishRepair(GodPlayer)
				resumed.advancedFinishRepair(GodPlayer)
				if w.StateHash() != resumed.StateHash() {
					t.Fatal("repair did not survive snapshot")
				}
				if w.checkLife(GodPlayer, pos) != CityFood {
					t.Fatal("repair did not finish the castle footprint")
				}
				for _, alt := range w.Alt {
					if alt != altitude {
						t.Fatal("repair levelled the high plateau to another altitude")
					}
				}
				if cost := 1000 - w.Magnets[GodPlayer].Mana; cost != 2*(ManaPointCost+4) {
					t.Fatalf("repair cost=%d, want two legal point edits", cost)
				}
			})
		}
	}
}

func TestAdvancedComputerBanksDecisiveSpellAndAttacksMobileSurvivors(t *testing.T) {
	w := advancedTestWorld()
	w.Computer[GodPlayer].Mode = computerQuake | computerWar
	w.Computer[GodPlayer].NoCastles = 3
	w.Computer[DevilPlayer].Mode |= computerQuake
	w.Magnets[GodPlayer].Mana = 10000
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 20000, AtPos: 20 + 20*MapWidth}, {Player: DevilPlayer, Flags: InTown, Population: 10000, AtPos: 45 + 45*MapWidth}}
	if w.advancedPower(GodPlayer) || w.Magnets[GodPlayer].Mana != 10000 {
		t.Fatal("AI spent its decisive spell savings on harassment")
	}
	w.Magnets[GodPlayer].Mana = ManaWarCost + 1000
	if !w.advancedReadyWar(GodPlayer) || !w.War {
		t.Fatal("AI failed to convert its advantage into Armageddon")
	}
	w = advancedTestWorld()
	w.Computer[GodPlayer].Mode = computerQuake
	w.Computer[DevilPlayer].Mode |= computerQuake
	w.Magnets[GodPlayer].Mana = ManaQuakeCost + 500
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 3000, AtPos: 20 + 20*MapWidth}, {Player: DevilPlayer, Flags: OnMove, Population: 150, AtPos: 45 + 45*MapWidth}}
	if !w.advancedPower(GodPlayer) || w.Magnets[GodPlayer].Mana != 500 {
		t.Fatal("AI ignored the mobile enemy after its towns were destroyed")
	}
}

func TestAdvancedComputerWaitsForOriginalKnightStrength(t *testing.T) {
	w := advancedTestWorld()
	w.Computer[GodPlayer].Mode |= computerKnight
	w.Computer[GodPlayer].NoCastles = 3
	w.Computer[DevilPlayer].Mode |= computerQuake
	w.Magnets[GodPlayer].Mana = ManaKnightCost + 500
	w.Magnets[GodPlayer].Carried = 1
	w.Peeps = []Peep{
		{Player: GodPlayer, Flags: OnMove, Population: DevilMakesKnight, AtPos: 20 + 20*MapWidth},
		{Player: DevilPlayer, Flags: InTown, Population: 1000, AtPos: 40 + 40*MapWidth},
	}
	before := w.StateHash()
	if w.advancedReadyKnight(GodPlayer) || w.StateHash() != before {
		t.Fatal("AI spent 7500 mana on an undersized knight")
	}
	w.Peeps[0].Population++
	if !w.advancedReadyKnight(GodPlayer) || w.Peeps[0].Status != KnightStatus || w.Magnets[GodPlayer].Mana != 500 {
		t.Fatal("AI did not launch a legally funded, mature knight")
	}
}

func BenchmarkComputerStrategies(b *testing.B) {
	typical := GenerateWorld(Level{SeedOffset: 27068, PlayerPopulation: 10, EnemyPopulation: 10, PlayerPowers: 0x3f, EnemyPowers: 0x3f, EnemyRating: 5, EnemyReactionSpeed: 3})
	typical.Magnets[0].Mana, typical.Magnets[1].Mana = 11000, 11000
	for tick := 0; tick < 512; tick++ {
		typical.TickWithAdvancedComputer(GodPlayer)
	}
	full := advancedTestWorld()
	for i := 0; i < MaxPeeps; i++ {
		x, y := 2+(i%16)*4, 2+(i/16)*4
		pos := x + y*MapWidth
		full.Peeps = append(full.Peeps, Peep{Flags: InTown, Player: byte(i % 2), Population: 200, AtPos: pos})
		full.MapWho[pos] = byte(i + 1)
		full.Alt[x+2+(y+1)*EndWidth] = 2
	}
	full.makeMap(0, 0, MapWidth-1, MapHeight-1)
	for _, fixture := range []struct {
		name  string
		world *World
	}{{"typical", typical}, {"max_towns", full}} {
		for _, advanced := range []bool{false, true} {
			name := "original"
			if advanced {
				name = "advanced"
			}
			b.Run(fixture.name+"/"+name, func(b *testing.B) {
				base := fixture.world
				base.Computer[0].DoneTurn, base.Computer[1].DoneTurn = 0, 0
				peeps := make([]Peep, len(base.Peeps), MaxPeeps)
				events := make([]int, 0, 32)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Restore the same live position and open action slot every
					// iteration, so a long benchmark cannot settle into a cheap
					// empty world. Copying the fixture is included for both AIs.
					w := *base
					copy(peeps, base.Peeps)
					w.Peeps, w.SoundEvents = peeps, events[:0]
					if advanced {
						w.TickWithAdvancedComputer(GodPlayer)
					} else {
						w.TickWithComputer([2]bool{true, true})
					}
				}
			})
		}
	}
}
