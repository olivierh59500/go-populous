package populous

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"
	"testing"
)

// The fingerprint was obtained by compiling the unmodified C++ newrand,
// raise_point, make_alt, make_thing, make_map and make_woods_rocks bodies.
// clear_map's four initial RNG draws precede those routines; setup_display
// increments seed afterwards. No original sources or C++ compiler are needed
// to run this regression test, including when previous/ is not distributed.
func TestLegacyTerrainCampaignMatchesOriginalCppFingerprint(t *testing.T) {
	f, err := os.Open("../../assets/amiga/level.dat")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	levels, err := LoadLevels(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 495 {
		t.Fatalf("campaign has %d worlds, want 495", len(levels))
	}
	digest := sha256.New()
	for _, level := range levels {
		w := GenerateWorld(level)
		fmt.Fprintln(digest, legacyTerrainSignature(w))
	}
	const want = "dc3b775e4bd3f749a84d8143faf9d7b78346503d50cd12bbd325db57044fac6d"
	if got := fmt.Sprintf("%x", digest.Sum(nil)); got != want {
		t.Fatalf("495 original C++ landscapes/RNG fingerprint = %s, want %s", got, want)
	}
}

func legacyTerrainSignature(w *World) string {
	hash := func(data []byte) uint64 {
		h := fnv.New64a()
		_, _ = h.Write(data)
		return h.Sum64()
	}
	alt := make([]byte, len(w.Alt)*2)
	for i, value := range w.Alt {
		binary.LittleEndian.PutUint16(alt[i*2:], uint16(value))
	}
	return fmt.Sprintf("%d %d %d %d %d", uint16(w.rng), hash(alt), hash(w.MapAlt[:]), hash(w.MapBlk[:]), hash(w.MapBk2[:]))
}

func TestLegacyTerrainConquestManaThreshold(t *testing.T) {
	for _, number := range []int{0, 247, 248, 494} {
		w := GenerateWorld(Level{Number: number, SeedOffset: 25, PlayerPopulation: 1, EnemyPopulation: 1})
		want := 399
		if number >= 248 {
			want = ManaQuakeCost + 200
		}
		if w.Magnets[GodPlayer].Mana != 399 || w.Magnets[DevilPlayer].Mana != want {
			t.Fatalf("world %d initial mana=%d/%d, want 399/%d", number, w.Magnets[0].Mana, w.Magnets[1].Mana, want)
		}
	}
}

func TestLegacyTerrainInitialMagnetTargetsRemainAtMapCentre(t *testing.T) {
	w := GenerateWorld(Level{SeedOffset: 25, PlayerPopulation: 3, EnemyPopulation: 3})
	centre := MapWidth/2 + MapWidth*(MapHeight/2)
	for player, magnet := range w.Magnets {
		if magnet.GoTo != centre || magnet.Flags != SettleMode || magnet.Carried == 0 {
			t.Fatalf("player %d initial magnet=%+v, want carried/settle with target %d", player, magnet, centre)
		}
		if w.Peeps[magnet.Carried-1].AtPos == centre {
			t.Fatal("fixture does not distinguish initial target from carrier position")
		}
	}
}

func TestLegacyTerrainSculptDebtIsNotBattleFloor(t *testing.T) {
	w := &World{Rules: DefaultTerrainRules()}
	for y := 0; y < EndWidth; y++ {
		for x := 0; x < EndWidth; x++ {
			w.Alt[x+y*EndWidth] = max(7-max(abs(x-32), abs(y-32)), 0)
		}
	}
	w.makeMap(0, 0, 63, 63)
	w.Magnets[0].Mana = ManaPointCost
	if !w.RaiseAt(0, 32, 32) {
		t.Fatal("original affordable sculpt command was rejected")
	}
	if w.Alt[32+32*EndWidth] != 8 || w.Magnets[0].Mana != -900 {
		t.Fatalf("225-vertex sculpt: altitude=%d mana=%d, want 8/-900", w.Alt[32+32*EndWidth], w.Magnets[0].Mana)
	}
}

func TestLegacyTerrainSculptLimitStillChargesAndRebuilds(t *testing.T) {
	for _, raise := range []bool{false, true} {
		w := &World{Rules: DefaultTerrainRules()}
		alt := 0
		if raise {
			alt = 8
		}
		for i := range w.Alt {
			w.Alt[i] = alt
		}
		w.makeMap(0, 0, 63, 63)
		w.Magnets[0].Mana = 100
		pos := 32 + 32*MapWidth
		w.MapSteps[pos] = 42
		before := w.Alt
		kind := CommandLower
		if raise {
			kind = CommandRaise
		}
		accepted, err := w.ApplyCommand(Command{Kind: kind, Player: 0, X: 32, Y: 32})
		if err != nil || !accepted || w.Alt != before || w.Magnets[0].Mana != 90 || w.MapSteps[pos] != 0 {
			t.Fatalf("raise=%v accepted=%v err=%v mana=%d steps=%d", raise, accepted, err, w.Magnets[0].Mana, w.MapSteps[pos])
		}
	}
}

func TestLegacyTerrainTownOnlyAllowsSeaLevelWithWalker(t *testing.T) {
	pos := 10 + 10*MapWidth
	w := &World{
		Level: Level{GameMode: GameRaiseTown},
		Peeps: []Peep{{AtPos: pos, Population: 45, Flags: OnMove, Player: GodPlayer}},
	}
	w.MapWho[pos] = 1
	if w.HasBuildPresence(0, 8, 8, 8, 8) {
		t.Fatal("generic town-required presence changed")
	}
	if !w.HasBuildPresenceAt(0, 8, 8, 8, 8, 10, 10) {
		t.Fatal("sea-level construction rejected despite a visible living walker")
	}
	if w.HasBuildPresenceAt(0, 20, 20, 8, 8, 10, 10) || w.HasBuildPresenceAt(1, 8, 8, 8, 8, 10, 10) || w.HasBuildPresenceAt(0, 8, 8, 8, 8, 65, 10) {
		t.Fatal("sea-level exception bypassed area, ownership or coordinate bounds")
	}
	w.Alt[10+10*EndWidth] = 1
	if w.HasBuildPresenceAt(0, 8, 8, 8, 8, 10, 10) {
		t.Fatal("above-sea construction accepted without a town")
	}
	w.Peeps[0].Flags = InTown
	if !w.HasBuildPresenceAt(0, 8, 8, 8, 8, 10, 10) {
		t.Fatal("above-sea construction rejected with a town")
	}
	w.Peeps[0].Population = 0
	if w.HasBuildPresenceAt(0, 8, 8, 8, 8, 10, 10) {
		t.Fatal("dead town authorized construction")
	}
}

func TestLegacyTerrainAdvancedSculptUsesSameSeaException(t *testing.T) {
	w := &World{
		Level: Level{GameMode: GameRaiseTown},
		Peeps: []Peep{{AtPos: 10 + 10*MapWidth, Population: 45, Flags: OnMove}},
	}
	w.MapWho[w.Peeps[0].AtPos] = 1
	w.Magnets[0].Mana = 100
	w.Computer[0].Mode = computerLand
	if !w.advancedSculpt(0, 10, 10, true) || w.Alt[10+10*EndWidth] != 1 || w.Magnets[0].Mana != 86 {
		t.Fatal("strategic AI did not use the same paid sea-level exception")
	}
	if w.advancedSculpt(0, 10, 10, true) || w.Magnets[0].Mana != 86 {
		t.Fatal("strategic AI bypassed town restriction above sea level")
	}
}

func TestLegacyTerrainQuakeOriginalColumnOrder(t *testing.T) {
	for _, seed := range []lcg{0, 2, 1234, 32767} {
		w := &World{rng: seed}
		for i := range w.Alt {
			w.Alt[i] = 2
		}
		w.makeMap(0, 0, 63, 63)
		w.Magnets[0].Mana = ManaQuakeCost
		want := *w
		// populous_effect.cpp do_quake: two passes, X outermost, Y innermost.
		bounds := newAltBounds(20, 20)
		for pass := 0; pass < 2; pass++ {
			for x := 20; x < 29; x++ {
				for y := 20; y < 29; y++ {
					if want.Alt[x+y*EndWidth] == 0 {
						continue
					}
					switch want.rng.next() % 5 {
					case 1:
						want.raisePointTracked(x, y, bounds)
					case 2, 3, 4:
						want.lowerPointTracked(x, y, bounds)
					}
				}
			}
		}
		want.rebuildAltitudeBounds(bounds)
		if !w.QuakeAt(0, 20, 20) || legacyTerrainSignature(w) != legacyTerrainSignature(&want) {
			t.Fatalf("quake seed %d differs from original column order", seed)
		}
	}
}

func TestLegacyTerrainVolcanoOriginalRockColumnOrder(t *testing.T) {
	for _, seed := range []lcg{0, 2, 1234, 32767} {
		w := &World{rng: seed}
		for i := range w.Alt {
			w.Alt[i] = 2
		}
		w.makeMap(0, 0, 63, 63)
		w.Magnets[0].Mana = ManaVolcanoCost
		want := *w
		bounds := newAltBounds(20, 20)
		for ring := 0; ring <= 4; ring++ {
			for x := ring; x < 9-ring; x++ {
				for y := ring; y < 9-ring; y++ {
					switch want.rng.next() % 5 {
					case 1, 2, 4:
						want.raisePointTracked(20+x, 20+y, bounds)
					}
				}
			}
		}
		// populous_effect.cpp do_volcano: rock draws also visit X then Y.
		// Both protected magnets are outside this interior fixture.
		for x := 20; x < 28; x++ {
			for y := 20; y < 28; y++ {
				if want.rng.next()%5 == 0 {
					want.MapBlk[x+y*MapWidth] = RockBlock
					want.MapBk2[x+y*MapWidth] = 0
				}
			}
		}
		want.rebuildAltitudeBounds(bounds)
		if !w.VolcanoAt(0, 20, 20) || legacyTerrainSignature(w) != legacyTerrainSignature(&want) {
			t.Fatalf("volcano seed %d differs from original column order", seed)
		}
	}
}

func TestLegacyTerrainVolcanoStillProtectsBothCarriersAndMagnets(t *testing.T) {
	w := &World{rng: 1234}
	for i := range w.Alt {
		w.Alt[i] = 2
	}
	w.makeMap(0, 0, 63, 63)
	w.Magnets[0].Mana = ManaVolcanoCost
	unprotected := *w
	unprotected.VolcanoAt(0, 20, 20)
	var rocks []int
	for pos, block := range unprotected.MapBlk {
		if block == RockBlock {
			rocks = append(rocks, pos)
		}
	}
	if len(rocks) < 4 {
		t.Fatal("volcano fixture needs four rock positions")
	}
	w.Peeps = []Peep{
		{AtPos: rocks[0], Population: 45, Player: GodPlayer, Flags: OnMove},
		{AtPos: rocks[1], Population: 45, Player: DevilPlayer, Flags: OnMove},
	}
	for player := range w.Magnets {
		w.Magnets[player].Carried = player + 1
		w.Magnets[player].GoTo = rocks[player+2]
		w.MapWho[rocks[player]] = byte(player + 1)
	}
	if !w.VolcanoAt(0, 20, 20) {
		t.Fatal("volcano rejected")
	}
	for _, pos := range rocks[:4] {
		if w.MapBlk[pos] == RockBlock {
			t.Fatalf("protected carrier/magnet at %d buried by rock", pos)
		}
	}
}
