package populous

import (
	"fmt"
	"testing"
)

// These fixtures exercise move_peeps/get_message ordering from the original
// populous_peeps.cpp and populous_io.cpp, independently of the advanced AI.
func legacyLoopWorld() *World {
	w := &World{Rules: DefaultTerrainRules()}
	for player := range w.Computer {
		w.Computer[player] = ComputerStats{
			Mode: computerLand, Skill: 1, Speed: 10,
			Best1: -1, Best2: -1, MyBest: -1, DoneTurn: 1,
		}
		w.Magnets[player] = Magnet{Flags: SettleMode, Mana: 399}
	}
	return w
}

func legacyLoopFlatWorld() *World {
	w := legacyLoopWorld()
	for i := range w.Alt {
		w.Alt[i] = 1
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	return w
}

func legacyLoopCastle(player, population int) *World {
	w := legacyLoopFlatWorld()
	w.GameTurn = 7
	pos := 20 + 20*MapWidth
	w.Peeps = []Peep{{
		Flags: InTown, Player: byte(player), IQ: 1, Population: population,
		AtPos: pos, Frame: LastTown, LandComplete: true,
	}}
	w.MapWho[pos] = 1
	w.Computer[player].NoCastles = 1 // Statistics collected on the preceding turn.
	w.Computer[player].DoneTurn = 0
	w.setTown(0, false)
	return w
}

func TestLegacyLoopCastleEmigrationIsLegacyAIOnly(t *testing.T) {
	// populous_peeps.cpp:197-221: an available computer action lets a castle
	// emigrate at MAX_FOOD, without reducing the castle's production stage.
	for player := 0; player < 2; player++ {
		for _, tc := range []struct {
			name       string
			controlled bool
			legacy     bool
			slotUsed   bool
			wantSplit  bool
		}{
			{"legacy", true, true, false, true},
			{"legacy_busy", true, true, true, false},
			{"advanced", true, false, false, false},
			{"human", false, true, false, false},
		} {
			t.Run(fmt.Sprintf("player%d/%s", player, tc.name), func(t *testing.T) {
				initialPopulation := MaxFood + 95
				w := legacyLoopCastle(player, initialPopulation)
				w.GameTurn = 8
				w.ComputerControlled[player] = tc.controlled
				if tc.slotUsed {
					w.Computer[player].DoneTurn = 1
				}
				if life := w.checkLife(player, w.Peeps[0].AtPos); life != CityFood {
					t.Fatalf("fixture castle capacity = %d, want %d", life, CityFood)
				}
				mana := w.Magnets[player].Mana
				w.processTownWithLandAI(0, tc.legacy)
				if tc.wantSplit {
					if len(w.Peeps) != 2 {
						t.Fatalf("groups = %d, want castle plus emigrants", len(w.Peeps))
					}
					wantTown := MaxFood/2 + w.Rules.PopulationAdd[LastTown-FirstTown]
					wantWalker := initialPopulation - MaxFood/2
					if w.Peeps[0].Population != wantTown || w.Peeps[1].Population != wantWalker {
						t.Errorf("populations = %d/%d, want %d/%d", w.Peeps[0].Population, w.Peeps[1].Population, wantTown, wantWalker)
					}
					if w.Computer[player].DoneTurn == 0 {
						t.Error("early emigration must consume the legacy computer action")
					}
				} else if len(w.Peeps) != 1 || w.Peeps[0].Population != initialPopulation+w.Rules.PopulationAdd[LastTown-FirstTown] {
					t.Fatalf("unexpected early emigration: %+v", w.Peeps)
				}
				if w.Peeps[0].Frame != LastTown || w.Magnets[player].Mana-mana != w.Rules.ManaAdd[LastTown-FirstTown] {
					t.Error("early emigration changed the castle's frame or mana production")
				}
			})
		}
	}
}

func TestLegacyLoopProcessesAppendedAndRecycledNewborns(t *testing.T) {
	// populous_peeps.cpp:90-93 and 218-219: no_peeps is read during iteration,
	// so a new last slot is processed on its birth turn, like a recycled slot.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			appended := legacyLoopCastle(player, CityFood+1)
			recycled := legacyLoopCastle(player, CityFood+1)
			recycled.Peeps = append(recycled.Peeps, Peep{})
			for _, w := range []*World{appended, recycled} {
				w.TickWithComputer([2]bool{})
				if len(w.Peeps) != 2 {
					t.Fatalf("groups = %d, want 2", len(w.Peeps))
				}
				if w.Peeps[1].Frame != 1 {
					t.Errorf("newborn frame = %d, want first animation step on birth turn", w.Peeps[1].Frame)
				}
			}
			if appended.Peeps[1] != recycled.Peeps[1] || appended.Magnets[player] != recycled.Magnets[player] {
				t.Errorf("dead tail changes newborn processing: appended=%+v recycled=%+v", appended.Peeps[1], recycled.Peeps[1])
			}
		})
	}
}

func TestLegacyLoopSwimmerEmergencyRaise(t *testing.T) {
	// populous_peeps.cpp:98-113: an AI swimmer overrides its busy action slot;
	// no-build and fatal-water rules still prevent this rescue.
	for player := 0; player < 2; player++ {
		for _, tc := range []struct {
			name       string
			controlled bool
			mode       byte
			powers     int
			wantRaise  bool
			wantAlive  bool
		}{
			{"computer", true, 0, computerLand, true, true},
			{"computer_without_land", true, 0, 0, true, true},
			{"human", false, 0, computerLand, false, true},
			{"no_build", true, GameNoBuild, computerLand, false, true},
			{"fatal_water", true, GameWaterFatal, computerLand, false, false},
		} {
			t.Run(fmt.Sprintf("player%d/%s", player, tc.name), func(t *testing.T) {
				w := legacyLoopWorld()
				w.Level.GameMode = tc.mode
				w.Computer[player].Mode = tc.powers
				pos := 20 + 20*MapWidth
				w.Peeps = []Peep{{Flags: OnMove | InWater, Player: byte(player), Population: 100, AtPos: pos, Frame: FirstWaterSprite}}
				w.MapWho[pos] = 1
				controlled := [2]bool{}
				controlled[player] = tc.controlled
				w.TickWithComputer(controlled)
				wantAltitude := 0
				if tc.wantRaise {
					wantAltitude = 1
				}
				if got := w.Alt[20+20*EndWidth]; got != wantAltitude {
					t.Errorf("rescue altitude = %d, want %d", got, wantAltitude)
				}
				wantPopulation := 0
				if tc.wantAlive {
					wantPopulation = 100 - 2*w.Rules.WalkDeath
				}
				if got := w.Peeps[0].Population; got != wantPopulation {
					t.Errorf("population = %d, want %d", got, wantPopulation)
				}
			})
		}
	}
}

func TestLegacyLoopEmergencyRaiseUsesLastCommandPerSide(t *testing.T) {
	// The original send[] slot is overwritten by each swimmer, then applied
	// once in get_message after move_peeps (including during Armageddon).
	for player := 0; player < 2; player++ {
		for _, war := range []bool{false, true} {
			t.Run(fmt.Sprintf("player%d/war%v", player, war), func(t *testing.T) {
				w := legacyLoopWorld()
				w.War = war
				if war {
					w.Level.GameMode = GameNoBuild
					w.Magnets[player].Mana = ManaFloor
				}
				positions := []int{12 + 12*MapWidth, 45 + 45*MapWidth}
				for _, pos := range positions {
					w.Peeps = append(w.Peeps, Peep{Flags: OnMove | InWater, Player: byte(player), Population: 100, AtPos: pos, Frame: FirstWaterSprite})
					w.MapWho[pos] = byte(len(w.Peeps))
				}
				controlled := [2]bool{}
				controlled[player] = !war // Armageddon rescues human swimmers too.
				w.TickWithComputer(controlled)
				if first, last := w.Alt[12+12*EndWidth], w.Alt[45+45*EndWidth]; first != 0 || last != 1 {
					t.Errorf("raised points = %d/%d, want only the last queued command (0/1)", first, last)
				}
			})
		}
	}
}

func TestLegacyLoopWaterRecoveryStillTakesAttritionAndContinues(t *testing.T) {
	// populous_peeps.cpp:113-118 falls through to the normal movement branch
	// after applying water attrition; it does not return early on dry land.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{{Flags: OnMove | InWater, Player: byte(player), Population: 100, AtPos: pos, Frame: FirstWaterSprite}}
			w.MapWho[pos] = 1
			w.TickWithComputer([2]bool{})
			if p := w.Peeps[0]; p.Population != 100-2*w.Rules.WalkDeath || p.Frame != 1 || p.Flags != OnMove {
				t.Errorf("recovered swimmer = %+v, want water attrition, OnMove and frame1", p)
			}
		})
	}
}

func TestLegacyLoopComputerReservesManaForKnight(t *testing.T) {
	// populous_computer.cpp:161-171 returns for a sufficiently large carrier
	// even before enough mana is available: cheaper spells must not consume it.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			w.Peeps = []Peep{
				{Flags: InTown, Player: byte(player ^ 1), Population: 100, AtPos: 20 + 20*MapWidth, Frame: FirstTown},
				{Flags: OnMove, Player: byte(player), Population: DevilMakesKnight + 1000, AtPos: 40 + 40*MapWidth},
			}
			w.Magnets[player].Carried = 2
			w.Magnets[player].Mana = ManaKnightCost + 100
			w.Computer[player] = ComputerStats{Mode: computerKnight | computerQuake, Best2: 0, NoQuakes: 2}
			mana := w.Magnets[player].Mana
			if w.computerEffect(player) || w.Magnets[player].Mana != mana {
				t.Errorf("computer spent mana needed for knight: before=%d after=%d", mana, w.Magnets[player].Mana)
			}
		})
	}
}

func TestLegacyLoopMagnetModeCommandTakesEffectAfterMovement(t *testing.T) {
	// animate calls move_peeps before get_message. A mode selected by the AI
	// at the start of move_peeps must not change that same turn's movement.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			w.rng = 1 // First newrand()%3 == 0, selecting SETTLE_MODE.
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{{Flags: OnMove, Player: byte(player), Population: 100, AtPos: pos, Frame: 6, IQ: 1}}
			w.MapWho[pos] = 1
			w.Magnets[player] = Magnet{Carried: 1, GoTo: pos + 4, Flags: MagnetMode, Mana: 399}
			w.Computer[player].DoneTurn = 0
			controlled := [2]bool{}
			controlled[player] = true
			w.TickWithComputer(controlled)
			if p := w.Peeps[0]; p.AtPos != pos+1 || p.Flags != OnMove {
				t.Errorf("mode applied before movement: got %+v, want magnet movement to %d", p, pos+1)
			}
			if mode := w.Magnets[player].Flags; mode != SettleMode {
				t.Errorf("mode after command application = %d, want SettleMode", mode)
			}
		})
	}
}

func TestLegacyLoopDropsKnightCarrierBeforeMovement(t *testing.T) {
	// populous_peeps.cpp:59-71 also handles a carrier that became a knight by
	// merging with one, rather than directly invoking the knight power.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{
				{Flags: OnMove, Player: byte(player), Population: 100, AtPos: pos, HeadFor: 2},
				{Flags: OnMove, Player: byte(player ^ 1), Population: 100, AtPos: 40 + 40*MapWidth},
			}
			w.MapWho[pos] = 1
			w.MapWho[w.Peeps[1].AtPos] = 2
			w.Magnets[player].Carried = 1
			w.Magnets[player].GoTo = pos + 5
			w.TickWithComputer([2]bool{})
			if m := w.Magnets[player]; m.Carried != 0 || m.GoTo != pos {
				t.Errorf("knight retained or misplaced magnet: %+v, want dropped at %d", m, pos)
			}
		})
	}
}

func TestLegacyLoopWaitTimeoutUsesPreviousCounter(t *testing.T) {
	// populous_peeps.cpp:310 uses battle_population++ > 14, not ++counter > 14.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{{Flags: OnMove | IAmWaiting, Player: byte(player), Population: 100, AtPos: pos, Frame: FirstWaitSprite, BattlePopulation: 14, IQ: 1}}
			w.MapWho[pos] = 1
			w.TickWithComputer([2]bool{})
			if p := w.Peeps[0]; p.Flags&IAmWaiting == 0 || p.BattlePopulation != 15 {
				t.Fatalf("wait expired at old counter14: %+v", p)
			}
			w.TickWithComputer([2]bool{})
			if p := w.Peeps[0]; p.Flags&(IAmWaiting|WaitForMe) != 0 {
				t.Errorf("wait did not expire at old counter15: %+v", p)
			}
		})
	}
}

func TestLegacyLoopWalkerMakesStationaryOccupantWait(t *testing.T) {
	// populous_peeps.cpp:283-298 signals the map occupant even between the
	// moving walker's full movement steps (after the set_frame condition).
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{
				{Flags: OnMove, Player: byte(player), Population: 100, AtPos: pos, Frame: 2},
				{Flags: OnMove, Player: byte(player), Population: 100, AtPos: pos, Frame: 0},
			}
			w.MapWho[pos] = 2
			w.TickWithComputer([2]bool{})
			if p := w.Peeps[1]; p.Flags&WaitForMe == 0 || p.BattlePopulation != 1 || p.Frame != FirstWaitSprite+1 {
				t.Errorf("stationary occupant was not told to wait: %+v", p)
			}
		})
	}
}

func TestLegacyLoopExpiredRuinClearsMapReference(t *testing.T) {
	// populous_peeps.cpp:329-339 runs zero_population after a ruin expires.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{{Flags: InRuin, Player: byte(player), Population: 1, AtPos: pos, BattlePopulation: 0}}
			w.MapWho[pos] = 1
			w.MapBk2[pos] = FirstRuinTown
			w.TickWithComputer([2]bool{})
			if w.Peeps[0].Population != 0 || w.MapWho[pos] != 0 {
				t.Errorf("expired ruin retains population or occupancy: peep=%+v mapWho=%d", w.Peeps[0], w.MapWho[pos])
			}
			if w.MapBk2[pos] != FirstRuinTown {
				t.Error("expiring the ruin's population removed its visual remains")
			}
		})
	}
}

func TestLegacyLoopCollectsStatisticsWhileProcessingGroups(t *testing.T) {
	// populous_peeps.cpp:154-181 and 273-274 collect only the branch visited
	// this turn: walkers enter Best2 at a movement step, newly settled towns
	// become Best1/MyBest and affect town counts on the following turn.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopFlatWorld()
			pos := 20 + 20*MapWidth
			w.Peeps = []Peep{{Flags: OnMove, Player: byte(player), Population: 100, AtPos: pos, Frame: 5, IQ: 1}}
			w.MapWho[pos] = 1
			w.TickWithComputer([2]bool{})
			if got := w.Computer[player^1].Best2; got != -1 {
				t.Fatalf("walker between movement steps became Best2: %d", got)
			}
			w.TickWithComputer([2]bool{})
			if w.Peeps[0].Flags != InTown {
				t.Fatalf("fixture walker did not settle: %+v", w.Peeps[0])
			}
			if w.Computer[player^1].Best2 != 0 || w.Computer[player^1].Best1 != -1 || w.Computer[player].NoCastles != 0 || w.Computer[player].NoTowns != 0 {
				t.Errorf("new settlement counted too early or walker target omitted: own=%+v enemy=%+v", w.Computer[player], w.Computer[player^1])
			}
			w.TickWithComputer([2]bool{})
			if w.Computer[player].NoCastles != 1 || w.Computer[player].NoTowns != 0 || w.Computer[player^1].Best1 != 0 || w.Computer[player].MyBest != 0 {
				t.Errorf("settled castle missing from next turn statistics: own=%+v enemy=%+v", w.Computer[player], w.Computer[player^1])
			}
		})
	}
}

func TestLegacyLoopBlockedExpeditionRequestsTerrain(t *testing.T) {
	// populous_peeps.cpp:669-685: a legacy computer also builds a passage
	// before entering water, or raises the swamp that its knight cannot cross.
	// These emergency orders bypass computer_done and C_LAND, but not the
	// applicable no-build / only-raise rules. The strategic AI plans its own.
	for player := 0; player < 2; player++ {
		for _, tc := range []struct {
			name      string
			swamp     bool
			mode      byte
			advanced  bool
			wantRaise bool
		}{
			{"water", false, 0, false, true},
			{"water_no_build", false, GameNoBuild, false, false},
			{"water_only_raise", false, GameOnlyRaise, false, true},
			{"water_advanced", false, 0, true, false},
			{"swamp_knight", true, 0, false, true},
			{"swamp_no_build", true, GameNoBuild, false, false},
			{"swamp_only_raise", true, GameOnlyRaise, false, false},
			{"swamp_advanced", true, 0, true, false},
		} {
			t.Run(fmt.Sprintf("player%d/%s", player, tc.name), func(t *testing.T) {
				w := legacyLoopWorld()
				pos := 20 + 20*MapWidth
				raiseX := 20
				if tc.swamp {
					w = legacyLoopFlatWorld()
					w.MapBlk[pos-1] = SwampBlock
					raiseX = 19
				} else {
					// A shore tile whose desired neighbour to the west is water.
					w.Alt[21+20*EndWidth] = 1
					w.makeMap(0, 0, MapWidth-1, MapHeight-1)
					if w.MapBlk[pos] == WaterBlock || w.MapBlk[pos-1] != WaterBlock {
						t.Fatal("invalid shoreline fixture")
					}
				}
				w.Level.GameMode = tc.mode
				w.Computer[player].Mode = 0
				w.Peeps = []Peep{{Flags: OnMove, Player: byte(player), Population: 100, AtPos: pos, Frame: 6, IQ: 1}}
				w.MapWho[pos] = 1
				w.Magnets[player] = Magnet{Carried: 1, GoTo: pos - 5, Flags: MagnetMode, Mana: 399}
				if tc.swamp {
					w.Peeps[0].HeadFor = 2
					w.Magnets[player].Carried = 0
					w.Peeps = append(w.Peeps, Peep{Flags: OnMove, Player: byte(player ^ 1), Population: 100, AtPos: pos - 5})
					w.MapWho[pos-5] = 2
				}
				before := w.Alt[raiseX+20*EndWidth]
				controlled := [2]bool{}
				controlled[player] = true
				advancedPlayer := -1
				if tc.advanced {
					advancedPlayer = player
				}
				w.tickWithComputerStrategy(controlled, advancedPlayer)
				want := before
				if tc.wantRaise {
					want++
				}
				if got := w.Alt[raiseX+20*EndWidth]; got != want {
					t.Errorf("expedition terrain height = %d, want %d", got, want)
				}
			})
		}
	}
}

func TestLegacyLoopWarBridgeIsDeferredAndCanBeOverridden(t *testing.T) {
	// A later swimmer overwrites the bridge requested by an earlier marcher.
	// An immediate forceRaiseAt during path finding cannot reproduce this.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopWorld()
			w.War = true
			w.Level.GameMode = GameNoBuild
			w.Magnets[player].Mana = ManaFloor
			marcherPos := 20 + 32*MapWidth
			swimmerPos := 45 + 45*MapWidth
			w.Alt[20+32*EndWidth] = 1
			w.makeMap(0, 0, MapWidth-1, MapHeight-1)
			if w.MapBlk[marcherPos] == WaterBlock || w.MapBlk[marcherPos+1] != WaterBlock {
				t.Fatal("invalid shoreline toward Armageddon centre")
			}
			w.Peeps = []Peep{
				{Flags: OnMove, Player: byte(player), Population: 100, AtPos: marcherPos, Frame: 6, IQ: 1},
				{Flags: OnMove | InWater, Player: byte(player), Population: 100, AtPos: swimmerPos, Frame: FirstWaterSprite},
			}
			w.MapWho[marcherPos], w.MapWho[swimmerPos] = 1, 2
			w.TickWithComputer([2]bool{})
			if bridge, rescue := w.Alt[20+32*EndWidth], w.Alt[45+45*EndWidth]; bridge != 1 || rescue != 1 {
				t.Errorf("bridge/rescue altitude = %d/%d, want unchanged bridge and executed final rescue (1/1)", bridge, rescue)
			}
		})
	}
}

func TestLegacyLoopTransientOrdersDoNotAliasWorldCopies(t *testing.T) {
	w := legacyLoopFlatWorld()
	w.legacyTurn = legacyTurnState{active: true, advancedPlayer: -1}
	w.legacyOrder(GodPlayer, CommandRaise, 10, 10, 0)
	probe := *w
	probe.legacyOrder(GodPlayer, CommandLower, 10, 10, 0)
	if w.legacyTurn.orders[GodPlayer].Kind != CommandRaise || probe.legacyTurn.orders[GodPlayer].Kind != CommandLower {
		t.Fatal("a copied world's plan changed the live world's queued order")
	}
	// Public terrain calls used by strategic lookahead retain their immediate
	// contract even while the copied world contains a legacy turn in progress.
	if !probe.RaiseAt(GodPlayer, 30, 30) || probe.Alt[30+30*EndWidth] != 2 || w.Alt[30+30*EndWidth] != 1 {
		t.Fatal("public terrain planning was deferred or changed the live world")
	}
	w.finishLegacyTurn()
	if w.legacyTurn != (legacyTurnState{}) {
		t.Fatal("transient turn state survived command application")
	}
}

func TestLegacyLoopSnapshotKeepsCompletedTurnDeterministic(t *testing.T) {
	for _, advancedPlayer := range []int{-1, GodPlayer, DevilPlayer} {
		t.Run(fmt.Sprintf("advanced%d", advancedPlayer), func(t *testing.T) {
			w := GenerateWorld(Level{SeedOffset: 27068, PlayerPopulation: 5, EnemyPopulation: 5, PlayerPowers: 0x3f, EnemyPowers: 0x3f, EnemyRating: 5, EnemyReactionSpeed: 3})
			w.Magnets[0].Mana, w.Magnets[1].Mana = 11000, 11000
			resumed := WorldFromSnapshot(w.Snapshot(), w.Rules)
			for tick := 0; tick < 520; tick++ {
				w.tickWithComputerStrategy([2]bool{true, true}, advancedPlayer)
				resumed.tickWithComputerStrategy([2]bool{true, true}, advancedPlayer)
				if w.legacyTurn != (legacyTurnState{}) || resumed.legacyTurn != (legacyTurnState{}) {
					t.Fatalf("transient commands survive completed tick %d", tick+1)
				}
				if tick == 499 {
					resumed = WorldFromSnapshot(resumed.Snapshot(), resumed.Rules)
				}
				if w.StateHash() != resumed.StateHash() {
					t.Fatalf("state differs after tick %d", tick+1)
				}
			}
		})
	}
}

func TestLegacyLoopCastleEmigrationWithoutComputerPowers(t *testing.T) {
	// The castle branch in populous_peeps.cpp:199-205 checks the controller
	// type and computer_done, not the selected computer_mode bit mask.
	for player := 0; player < 2; player++ {
		t.Run(fmt.Sprintf("player%d", player), func(t *testing.T) {
			w := legacyLoopCastle(player, MaxFood+95)
			w.Computer[player].Mode = 0
			controlled := [2]bool{}
			controlled[player] = true
			w.TickWithComputer(controlled)
			if len(w.Peeps) != 2 {
				t.Fatalf("disabled powers suppressed normal AI castle emigration: groups=%d", len(w.Peeps))
			}
			if w.Peeps[0].Population != MaxFood/2+w.Rules.PopulationAdd[LastTown-FirstTown] || w.Peeps[1].Population != MaxFood+95-MaxFood/2 {
				t.Errorf("unexpected castle division with no powers: %+v", w.Peeps)
			}
			if w.Computer[player].DoneTurn == 0 {
				t.Error("emigration failed to consume the available action slot")
			}
		})
	}
}

func TestLegacyLoopSwampOrderCanBeReplacedByTerrain(t *testing.T) {
	// populous_computer.cpp:198-202 does not set computer_done for swamp.
	// A later make_level order therefore overwrites it before get_message.
	for player := 0; player < 2; player++ {
		for _, terraform := range []bool{false, true} {
			t.Run(fmt.Sprintf("player%d/terraform%v", player, terraform), func(t *testing.T) {
				w := legacyLoopCastle(player, 200)
				w.GameTurn = 6 // Next turn has no periodic castle growth or income.
				w.Computer[player].Mode = computerLand | computerSwamp
				w.Computer[player].Best2 = 1 // Enemy carrier observed last turn.
				w.Magnets[player].Mana = 6000
				enemyPos := 40 + 40*MapWidth
				w.Peeps = append(w.Peeps, Peep{Flags: OnMove, Player: byte(player ^ 1), Population: 1000, AtPos: enemyPos})
				w.MapWho[enemyPos] = 2
				w.Magnets[player^1].Carried = 2
				if terraform {
					w.Alt[16+16*EndWidth] = 2
					w.makeMap(15, 15, 16, 16)
					w.Peeps[0].LandComplete = false
				}
				controlled := [2]bool{}
				controlled[player] = true
				w.TickWithComputer(controlled)
				if w.Computer[player].QuakeCount != 1 {
					t.Fatal("fixture did not select the initial swamp order")
				}
				if terraform {
					if w.Alt[16+16*EndWidth] != 1 || countBlocks(w, SwampBlock) != 0 {
						t.Error("terrain did not replace the pending swamp order")
					}
					if w.Magnets[player].Mana != 6000-ManaPointCost-4 || w.Computer[player].DoneTurn == 0 {
						t.Errorf("terrain action cost/slot = mana%d done%d", w.Magnets[player].Mana, w.Computer[player].DoneTurn)
					}
				} else {
					if w.Magnets[player].Mana != 6000-ManaSwampCost || countBlocks(w, SwampBlock) == 0 {
						t.Error("unreplaced swamp order was not applied")
					}
					if w.Computer[player].DoneTurn != 0 {
						t.Error("swamp alone locked the legacy action slot")
					}
				}
			})
		}
	}
}
