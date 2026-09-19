package populous

import (
	"fmt"
	"os"
	"testing"
)

func TestAdvancedFrontierUsesLocalPaidAction(t *testing.T) {
	w := advancedTestWorld()
	own, enemy := 20+20*MapWidth, 23+20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 100, AtPos: own}, {Player: DevilPlayer, Flags: InTown, Population: 200, AtPos: enemy}}
	w.MapWho[own], w.MapWho[enemy] = 1, 2
	beforeMana, beforeFood, beforeRNG := w.Magnets[0].Mana, w.checkLife(1, enemy), w.rng
	w.runAdvancedComputerPlayer(0)
	if w.checkLife(1, enemy) >= beforeFood || w.Magnets[0].Mana != beforeMana-ManaPointCost-4 {
		t.Fatal("frontier did not deprive the enemy of food with one ordinary paid point edit")
	}
	if w.Computer[0].DoneTurn != w.GameTurn || w.rng != beforeRNG || w.Peeps[1].Population != 200 {
		t.Fatal("frontier bypassed cadence, simulated RNG or directly damaged population")
	}
	hash := w.StateHash()
	w.runAdvancedComputerPlayer(0)
	if hash != w.StateHash() {
		t.Fatal("frontier performed two actions in one slot")
	}
}

func TestAdvancedFrontierHonoursBuildRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mode     byte
		ownPos   int
		ownFlags byte
		mana     int
	}{
		{"remote", 0, 50 + 50*MapWidth, OnMove, 1000},
		{"only towns", GameRaiseTown, 20 + 20*MapWidth, OnMove, 1000},
		{"no building", GameNoBuild, 20 + 20*MapWidth, InTown, 1000},
		{"only raising", GameOnlyRaise, 20 + 20*MapWidth, OnMove, 1000},
		{"no mana", 0, 20 + 20*MapWidth, OnMove, ManaPointCost - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedTestWorld()
			w.Level.GameMode, w.Magnets[0].Mana = tc.mode, tc.mana
			enemy := 23 + 20*MapWidth
			w.Peeps = []Peep{{Player: GodPlayer, Flags: tc.ownFlags, Population: 100, AtPos: tc.ownPos}, {Player: DevilPlayer, Flags: InTown, Population: 200, AtPos: enemy}}
			w.MapWho[tc.ownPos], w.MapWho[enemy] = 1, 2
			before := w.StateHash()
			if w.advancedFrontierLand(0) || w.StateHash() != before {
				t.Fatal("frontier action ignored construction restriction")
			}
		})
	}
}

func TestAdvancedFrontierSupportsKnightAcrossWater(t *testing.T) {
	w := advancedTestWorld()
	for i := range w.Alt {
		w.Alt[i] = 0
	}
	for y := 10; y <= 11; y++ {
		for x := 10; x <= 11; x++ {
			w.Alt[x+y*EndWidth] = 1
		}
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	own, enemy := 11+10*MapWidth, 40+10*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Status: KnightStatus, HeadFor: 2, Population: 1500, AtPos: own}, {Player: DevilPlayer, Flags: OnMove, Population: 500, AtPos: enemy}}
	w.MapWho[own], w.MapWho[enemy] = 1, 2
	next := own + 1
	if w.MapBlk[next] != WaterBlock {
		t.Fatal("fixture has no water in the expedition's way")
	}
	before := w.Magnets[0].Mana
	if !w.advancedFrontierLand(0) || w.MapBlk[next] == WaterBlock || w.Peeps[0].AtPos != own || w.Magnets[0].Mana != before-ManaPointCost-4 {
		t.Fatal("expedition did not pay for a local causeway, or teleported its knight")
	}
}

func TestAdvancedFrontierProtectsFriendlyTown(t *testing.T) {
	w := advancedTestWorld()
	own, enemy := 20+20*MapWidth, 21+20*MapWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: own}, {Player: DevilPlayer, Flags: InTown, Population: 200, AtPos: enemy}}
	w.MapWho[own], w.MapWho[enemy] = 1, 2
	before := w.StateHash()
	if w.advancedFrontierLand(0) || w.StateHash() != before {
		t.Fatal("frontier sacrificed an overlapping friendly town's food")
	}
}

func TestAdvancedSculptAtMapEdges(t *testing.T) {
	for _, point := range [][2]int{{0, 0}, {MapWidth, 0}, {0, MapHeight}, {MapWidth, MapHeight}} {
		w := advancedTestWorld()
		x, y := min(point[0], MapWidth-1), min(point[1], MapHeight-1)
		pos := x + y*MapWidth
		w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 100, AtPos: pos}}
		w.MapWho[pos] = 1
		if !w.advancedSculpt(0, point[0], point[1], true) || w.Magnets[0].Mana != 1000-ManaPointCost-4 {
			t.Fatalf("legal boundary construction rejected at %v", point)
		}
		before := w.StateHash()
		if w.advancedSculpt(0, -1, point[1], true) || w.advancedSculpt(0, MapWidth+1, point[1], true) || before != w.StateHash() {
			t.Fatal("out-of-bounds construction modified the world")
		}
	}
}

func TestAdvancedFoundingTileHandlesTwoLevelSaddle(t *testing.T) {
	for _, mode := range []byte{0, GameOnlyRaise} {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			w := advancedTestWorld()
			w.Level.GameMode = mode
			x, y := 20, 20
			pos := x + y*MapWidth
			w.Alt[x+y*EndWidth] = 1
			w.Alt[x+1+y*EndWidth] = 1
			w.Alt[x+(y+1)*EndWidth] = 2
			w.Alt[x+1+(y+1)*EndWidth] = 2
			w.makeMap(0, 0, MapWidth-1, MapHeight-1)
			w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 150, AtPos: pos}}
			w.MapWho[pos] = 1
			beforeRoughness := w.advancedFoundingRoughness(x, y)
			beforeMana := w.Magnets[GodPlayer].Mana
			if !w.advancedFoundingTile(GodPlayer, pos) {
				t.Fatal("AI ignored the two-low/two-high founding saddle")
			}
			if got := w.advancedFoundingRoughness(x, y); got >= beforeRoughness {
				t.Fatalf("roughness=%d, want below %d", got, beforeRoughness)
			}
			if spent := beforeMana - w.Magnets[GodPlayer].Mana; spent != ManaPointCost+4 {
				t.Fatalf("founding saddle cost=%d, want one changed vertex", spent)
			}
		})
	}
}

func TestAdvancedUrgentFoundingProtectsOnlyFragileHighAttritionSettler(t *testing.T) {
	makeWorld := func(population int) *World {
		w := advancedTestWorld()
		w.Rules.WalkDeath = 8
		x, y := 20, 20
		pos := x + y*MapWidth
		w.Alt[x+y*EndWidth] = 1
		w.Alt[x+1+y*EndWidth] = 1
		w.Alt[x+(y+1)*EndWidth] = 2
		w.Alt[x+1+(y+1)*EndWidth] = 2
		w.makeMap(0, 0, MapWidth-1, MapHeight-1)
		w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: population, AtPos: pos}}
		w.MapWho[pos] = 1
		return w
	}

	fragile := makeWorld(8 * 16)
	beforeMana := fragile.Magnets[GodPlayer].Mana
	beforeRoughness := fragile.advancedFoundingRoughness(20, 20)
	if !fragile.advancedUrgentFounding(GodPlayer) {
		t.Fatal("fragile high-attrition settler did not receive a founding action")
	}
	if fragile.Magnets[GodPlayer].Mana >= beforeMana || fragile.advancedFoundingRoughness(20, 20) >= beforeRoughness {
		t.Fatal("urgent founding did not pay for a useful terrain improvement")
	}

	mature := makeWorld(8*16 + 1)
	before := mature.StateHash()
	if mature.advancedUrgentFounding(GodPlayer) || mature.StateHash() != before {
		t.Fatal("non-fragile settler stole an urgent action slot")
	}
}

func TestAdvancedRepairAbandonsLostConstructionPresence(t *testing.T) {
	w := advancedTestWorld()
	pos, point := 20+20*MapWidth, 22+20*EndWidth
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos}}
	w.MapWho[pos] = 1
	w.MapBlk[pos+2] = RockBlock + 1
	if !w.advancedRepair(0) {
		t.Fatal("fixture did not start a repair")
	}
	w.Peeps[0].Population = 0
	w.MapWho[pos] = 0
	beforeAlt, beforeMana := w.Alt, w.Magnets[0].Mana
	if w.advancedFinishRepair(0) || w.Alt != beforeAlt || w.Magnets[0].Mana != beforeMana || w.Computer[0].Arrived != 0 {
		t.Fatalf("repair continued after losing its only construction presence at %d", point)
	}
}

func TestAdvancedFloodForecastMatchesActualFloodWithoutSideEffects(t *testing.T) {
	w := advancedTestWorld()
	flat, slope, high := 10+10*MapWidth, 30+30*MapWidth, 50+50*MapWidth
	w.Alt[30+30*EndWidth] = 2
	for y := 49; y <= 52; y++ {
		for x := 49; x <= 52; x++ {
			w.Alt[x+y*EndWidth] = 2
		}
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.Peeps = []Peep{{Player: GodPlayer, Flags: OnMove, Population: 200, AtPos: slope}, {Player: DevilPlayer, Flags: InTown, Population: 300, AtPos: flat}, {Player: GodPlayer, Flags: InTown, Population: 400, AtPos: high}}
	w.SoundEvents = []int{TuneMagnet}
	w.Computer[0].Mode |= computerFlood
	w.Magnets[0].Mana = ManaFloodCost
	before := w.StateHash()
	got := w.advancedFloodExposure()
	if got != [2]int{0, 300} || before != w.StateHash() || len(w.SoundEvents) != 1 || w.SoundEvents[0] != TuneMagnet {
		t.Fatalf("forecast=%v, expected only low flat enemy; forecast changed game state or audio", got)
	}
	if !w.Flood(0) {
		t.Fatal("fixture could not flood")
	}
	want := [2]int{}
	for _, p := range w.Peeps {
		if w.MapBlk[p.AtPos] == WaterBlock {
			want[p.Player] += p.Population
		}
	}
	if got != want {
		t.Fatalf("forecast=%v actual=%v", got, want)
	}
}

func TestAdvancedFloodConsidersSurvivingForcesAndUsesNormalCost(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		ownDry, ownWet, enemyDry, enemyWet int
		want                               bool
	}{
		{"decisive despite greater absolute losses", 14000, 6000, 1000, 3000, true},
		{"would lose most of own army", 4000, 6000, 1000, 3000, false},
		{"would leave stronger enemy", 800, 400, 1000, 2100, false},
		{"enemy mostly sheltered", 14000, 6000, 3000, 1000, false},
		{"last exposed survivors", 1000, 0, 0, 50, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for player := 0; player < 2; player++ {
				w := advancedTestWorld()
				w.Rules.WalkDeath = 8
				for i := range w.Alt {
					w.Alt[i] = 2
				}
				w.Peeps = []Peep{
					{Player: byte(player), Population: tc.ownDry, Flags: InTown, AtPos: 10 + 10*MapWidth},
					{Player: byte(player), Population: tc.ownWet, Flags: InTown, AtPos: 20 + 20*MapWidth},
					{Player: byte(player ^ 1), Population: tc.enemyDry, Flags: InTown, AtPos: 40 + 40*MapWidth},
					{Player: byte(player ^ 1), Population: tc.enemyWet, Flags: InTown, AtPos: 50 + 50*MapWidth},
				}
				for _, index := range []int{1, 3} {
					p := w.Peeps[index].AtPos
					base := p%MapWidth + p/MapWidth*EndWidth
					for _, corner := range [...]int{0, 1, EndWidth, EndWidth + 1} {
						w.Alt[base+corner] = 1
					}
				}
				w.makeMap(0, 0, MapWidth-1, MapHeight-1)
				w.Computer[player].Mode |= computerFlood
				w.Magnets[player].Mana = ManaFloodCost + 1000
				before, beforeRNG := w.StateHash(), w.rng
				if got := w.advancedReadyFlood(player); got != tc.want {
					t.Fatalf("cast=%v want=%v", got, tc.want)
				}
				if !tc.want && w.StateHash() != before {
					t.Fatal("rejected flood modified the world")
				}
				if tc.want && (w.Magnets[player].Mana != 1000 || w.MapBlk[w.Peeps[3].AtPos] != WaterBlock || w.rng != beforeRNG) {
					t.Fatal("flood bypassed normal terrain, mana or RNG behaviour")
				}
			}
		})
	}
}

func TestAdvancedPolicyRalliesBeforeLaunchingKnight(t *testing.T) {
	w := advancedTestWorld()
	w.Computer[0].NoCastles = 3
	w.Computer[0].Mode |= computerKnight
	w.Magnets[0].Mana, w.Magnets[0].Carried = 10000, 2
	w.Peeps = []Peep{{Player: GodPlayer, Flags: InTown, Population: 3000, AtPos: 40 + 40*MapWidth}, {Player: GodPlayer, Flags: OnMove, Population: 500, AtPos: 20 + 20*MapWidth}, {Player: DevilPlayer, Flags: InTown, Population: 1000, AtPos: 50 + 50*MapWidth}}
	before := w.Magnets[0].Mana
	if !w.advancedPolicy(0) || w.Magnets[0].Flags != MagnetMode || w.Magnets[0].GoTo != w.Peeps[1].AtPos || w.Magnets[0].Mana != before-ManaMagnetCost {
		t.Fatal("rally did not use the existing leader and ordinary mana")
	}
	if w.Magnets[0].Carried != 2 || w.Peeps[1].Status == KnightStatus {
		t.Fatal("policy converted an undersized leader without ordinary recruitment")
	}
}

func TestAdvancedPolicyLetsHighAttritionCarrierGrowInTown(t *testing.T) {
	base := advancedTestWorld()
	base.Rules.WalkDeath = 8
	base.Computer[GodPlayer].NoCastles = 3
	base.Computer[GodPlayer].Mode |= computerKnight
	base.Magnets[GodPlayer].Mana = 10000
	base.Magnets[GodPlayer].Carried = 1
	base.Peeps = []Peep{
		{Player: GodPlayer, Flags: InTown, Population: 2600, AtPos: 20 + 20*MapWidth},
		{Player: DevilPlayer, Flags: InTown, Population: 1000, AtPos: 50 + 50*MapWidth},
	}
	base.MapWho[base.Peeps[0].AtPos], base.MapWho[base.Peeps[1].AtPos] = 1, 2
	before := base.StateHash()
	if base.advancedPolicy(GodPlayer) || base.StateHash() != before {
		t.Fatal("high-attrition policy woke a growing carrier and spent rally mana")
	}

	lowAttrition := *base
	lowAttrition.Rules.WalkDeath = 1
	if !lowAttrition.advancedPolicy(GodPlayer) || lowAttrition.Peeps[0].Flags != OnMove || lowAttrition.Magnets[GodPlayer].Mana != 10000-ManaMagnetCost {
		t.Fatal("low-attrition policy did not retain ordinary rally behaviour")
	}
}

func TestAdvancedNoBuildPolicyPreservesEconomyAndActionRules(t *testing.T) {
	for _, mode := range []byte{0, GameOnlyRaise, GameRaiseTown, GameNoBuild, GameNoBuild | GameOnlyRaise | GameRaiseTown} {
		for _, towns := range []int{3, 4} {
			t.Run(fmt.Sprintf("mode%d_towns%d", mode, towns), func(t *testing.T) {
				w := advancedTestWorld()
				w.Level.GameMode = mode
				w.Computer[0].NoTowns = towns
				for i := 0; i < towns; i++ {
					pos := 10 + i*8 + 10*MapWidth
					w.Peeps = append(w.Peeps, Peep{Player: GodPlayer, Flags: InTown, Population: 200, AtPos: pos})
					w.MapWho[pos] = byte(i + 1)
				}
				beforeAlt, beforeMana, beforeRNG := w.Alt, w.Magnets[0].Mana, w.rng
				w.runAdvancedComputerPlayer(0)
				want := SettleMode
				if mode&GameNoBuild != 0 && towns >= 4 {
					want = FightMode
				}
				if w.Magnets[0].Flags != want {
					t.Fatalf("mode=%d want=%d", w.Magnets[0].Flags, want)
				}
				if w.Alt != beforeAlt || w.Magnets[0].Mana != beforeMana || w.rng != beforeRNG || len(w.SoundEvents) != 0 {
					t.Fatal("military mode change modified terrain, mana, random stream or powers")
				}
				for _, p := range w.Peeps {
					if p.Flags != InTown || p.Population != 200 {
						t.Fatal("military mode evacuated a productive town")
					}
				}
				if want == FightMode {
					if w.Computer[0].DoneTurn != w.GameTurn {
						t.Fatal("mode change bypassed action cadence")
					}
					before := w.StateHash()
					w.runAdvancedComputerPlayer(0)
					if w.StateHash() != before {
						t.Fatal("AI acted twice in one slot")
					}
				}
			})
		}
	}
}

func TestAdvancedNoBuildCampaignRulesAndSnapshot(t *testing.T) {
	f, err := os.Open("../../assets/amiga/level.dat")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	levels, err := LoadLevels(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../assets/amiga/land0")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeTerrainRules(data)
	if err != nil {
		t.Fatal(err)
	}
	// These two worlds used to be wins with the pre-parity engine/maps. That
	// performance observation is not a game-rule invariant: restoring the
	// historical map, mana and opponent changes their outcomes. Keep the hard
	// cases as full-length rule/determinism checks; ai-audit reports wins/losses.
	for _, tc := range []struct{ level, player int }{{163, DevilPlayer}, {304, GodPlayer}} {
		t.Run(fmt.Sprintf("world%d_side%d", tc.level, tc.player), func(t *testing.T) {
			level := levels[tc.level]
			if level.GameMode&GameNoBuild == 0 || level.Terrain != 0 {
				t.Fatal("fixture no longer has the expected campaign restrictions")
			}
			w := GenerateWorldWithRules(level, rules)
			restored := WorldFromSnapshot(w.Snapshot(), rules)
			for w.GameTurn < 20*60*8 && campaignOutcome(w, tc.player) == "ongoing" {
				w.TickWithAdvancedComputer(tc.player)
				restored.TickWithAdvancedComputer(tc.player)
				w.DrainSoundEvents()
				restored.DrainSoundEvents()
				if w.Level != level || w.Rules != rules {
					t.Fatal("strategic AI changed campaign restrictions or terrain rules")
				}
				if w.GameTurn%512 == 0 {
					if w.StateHash() != restored.StateHash() {
						t.Fatalf("snapshot diverged at tick %d", w.GameTurn)
					}
					restored = WorldFromSnapshot(restored.Snapshot(), rules)
				}
			}
			if w.StateHash() != restored.StateHash() {
				t.Fatal("restored campaign ended in a different state")
			}
			t.Logf("corrected-engine observation: outcome=%s, ticks=%d, living populations=%v", campaignOutcome(w, tc.player), w.GameTurn, w.PlayerPopulations())
		})
	}
}

// This bounded, opt-in campaign panel measures real elimination, not a proxy
// such as population. The complete audit command covers all original levels.
func TestAdvancedTacticsCampaignPanel(t *testing.T) {
	if os.Getenv("POPULOUS_TACTICS_PANEL") == "" {
		t.Skip("set POPULOUS_TACTICS_PANEL=1 to measure the 19-profile strategy panel")
	}
	f, err := os.Open("../../assets/amiga/level.dat")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	levels, err := LoadLevels(f)
	if err != nil {
		t.Fatal(err)
	}
	var rules [4]TerrainRules
	for i := range rules {
		b, err := os.ReadFile(fmt.Sprintf("../../assets/amiga/land%d", i))
		if err != nil {
			t.Fatal(err)
		}
		rules[i], err = DecodeTerrainRules(b)
		if err != nil {
			t.Fatal(err)
		}
	}
	counts := map[string]int{}
	for _, n := range []int{0, 5, 25, 80, 100, 200, 250, 400, 450, 10, 35, 70, 130, 160, 220, 300, 350, 475, 490} {
		l := levels[n]
		w := GenerateWorldWithRules(l, rules[l.Terrain])
		outcome := func() string {
			pop := [2]int{}
			for _, p := range w.Peeps {
				if p.Population > 0 && p.Flags&InRuin == 0 {
					pop[p.Player] += p.Population
				}
			}
			if pop[0] == 0 && pop[1] == 0 {
				return "draw"
			}
			if pop[0] == 0 {
				return "loss"
			}
			if pop[1] == 0 {
				return "win"
			}
			return "ongoing"
		}
		for w.GameTurn < 9600 && outcome() == "ongoing" {
			w.TickWithAdvancedComputer(GodPlayer)
			w.DrainSoundEvents()
		}
		counts[outcome()]++
		t.Logf("level=%d outcome=%s seconds=%.3f pop=%v castles=%d/%d mana=%d/%d war=%v", n, outcome(), float64(w.GameTurn)/8, w.PlayerPopulations(), w.Computer[0].NoCastles, w.Computer[1].NoCastles, w.Magnets[0].Mana, w.Magnets[1].Mana, w.War)
	}
	t.Logf("FINAL %v", counts)
}
