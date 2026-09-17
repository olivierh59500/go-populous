package populous

import (
	"reflect"
	"testing"
)

func TestPlayerPopulationsScansBothSides(t *testing.T) {
	world := &World{Peeps: []Peep{
		{Player: GodPlayer, Population: 30},
		{Player: DevilPlayer, Population: 45},
		{Player: GodPlayer, Population: 12},
		{Player: DevilPlayer, Population: 0},
		{Player: 7, Population: 99},
	}}

	populations := world.PlayerPopulations()
	if populations != [2]int{42, 45} {
		t.Fatalf("PlayerPopulations = %v, want [42 45]", populations)
	}
	for player, want := range populations {
		if got := world.PlayerPopulation(player); got != want {
			t.Fatalf("PlayerPopulation(%d) = %d, want %d", player, got, want)
		}
	}
	if got := world.PlayerPopulation(7); got != 0 {
		t.Fatalf("invalid player population = %d, want 0", got)
	}
}

func TestHumanOnlyTicksKeepComputerStatsAndDeterministicSnapshot(t *testing.T) {
	level := Level{
		Number:             31,
		Terrain:            2,
		SeedOffset:         0x5432,
		PlayerPopulation:   12,
		EnemyPopulation:    12,
		PlayerPowers:       0x3f,
		EnemyPowers:        0x3f,
		EnemyRating:        5,
		EnemyReactionSpeed: 4,
	}
	left := GenerateWorld(level)
	left.ComputerControlled = [2]bool{}
	right := WorldFromSnapshot(left.Snapshot(), left.Rules)
	computerBefore := left.Computer

	for tick := 0; tick < 256; tick++ {
		left.TickWithComputer([2]bool{})
		right.TickWithComputer([2]bool{})
		if left.StateHash() != right.StateHash() {
			t.Fatalf("human-only worlds diverged at tick %d", tick+1)
		}
	}
	if left.Computer != computerBefore || right.Computer != computerBefore {
		t.Fatalf("human-only ticks changed computer stats: left=%+v right=%+v", left.Computer, right.Computer)
	}
	if leftSnapshot, rightSnapshot := left.Snapshot(), right.Snapshot(); !reflect.DeepEqual(leftSnapshot, rightSnapshot) {
		t.Fatal("deterministic human-only worlds produced different snapshots")
	}
}

func TestControlledPlayerStillRefreshesComputerStats(t *testing.T) {
	world := GenerateWorld(Level{
		Terrain:            0,
		SeedOffset:         0x1234,
		PlayerPopulation:   3,
		EnemyPopulation:    3,
		EnemyRating:        3,
		EnemyReactionSpeed: 4,
	})
	world.Computer[DevilPlayer].NoTowns = 999
	world.Computer[DevilPlayer].Best1 = 999
	world.TickWithComputer([2]bool{false, true})
	if world.Computer[DevilPlayer].NoTowns == 999 || world.Computer[DevilPlayer].Best1 == 999 {
		t.Fatalf("controlled player stats were not refreshed: %+v", world.Computer[DevilPlayer])
	}
}

func TestMakeMapRowMajorMatchesColumnMajor(t *testing.T) {
	base := GenerateWorld(Level{Terrain: 0, SeedOffset: 0x3456, PlayerPopulation: 2, EnemyPopulation: 2})
	for y := 0; y < EndWidth; y++ {
		for x := 0; x < EndWidth; x++ {
			base.Alt[x+y*EndWidth] = (x*3 + y*5 + x*y) % 9
		}
	}
	for pos := 0; pos < MapWidth*MapHeight; pos++ {
		if pos%11 == 0 {
			base.MapBlk[pos] = RockBlock
		}
		base.MapBk2[pos] = byte((pos % 7) + 1)
		base.MapSteps[pos] = uint16(pos + 1)
	}
	baseline := base.Snapshot()

	tests := []struct {
		name           string
		x1, y1, x2, y2 int
	}{
		{name: "full", x1: 0, y1: 0, x2: MapWidth - 1, y2: MapHeight - 1},
		{name: "middle", x1: 9, y1: 13, x2: 37, y2: 42},
		{name: "corner", x1: 55, y1: 55, x2: MapWidth - 1, y2: MapHeight - 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := WorldFromSnapshot(baseline, base.Rules)
			want := WorldFromSnapshot(baseline, base.Rules)
			got.makeMap(test.x1, test.y1, test.x2, test.y2)
			makeMapColumnMajorReference(want, test.x1, test.y1, test.x2, test.y2)

			if got.MapAlt != want.MapAlt || got.MapBlk != want.MapBlk || got.MapBk2 != want.MapBk2 || got.MapSteps != want.MapSteps {
				t.Fatal("row-major makeMap differs from the original column-major result")
			}
			if got.StateHash() != want.StateHash() {
				t.Fatal("row-major makeMap changed the canonical world hash")
			}
		})
	}
}

func TestWorldPeepsArePreallocatedWithoutChangingSnapshotIndices(t *testing.T) {
	world := GenerateWorld(Level{Terrain: 0, SeedOffset: 0x2222, PlayerPopulation: 4, EnemyPopulation: 5})
	if cap(world.Peeps) != MaxPeeps {
		t.Fatalf("generated peep capacity = %d, want %d", cap(world.Peeps), MaxPeeps)
	}
	snapshot := world.Snapshot()
	restored := WorldFromSnapshot(snapshot, world.Rules)
	if cap(restored.Peeps) != MaxPeeps {
		t.Fatalf("restored peep capacity = %d, want %d", cap(restored.Peeps), MaxPeeps)
	}
	if restored.StateHash() != world.StateHash() {
		t.Fatal("peep preallocation changed snapshot indices or canonical state")
	}
	if len(restored.Peeps) > 0 {
		restored.Peeps[0].Population++
		if restored.Peeps[0].Population == snapshot.Peeps[0].Population {
			t.Fatal("restored peeps alias the snapshot backing array")
		}
	}
}

// makeMapColumnMajorReference preserves the traversal used before makeMap was
// changed to row-major order, so the test compares results rather than merely
// comparing two calls to the optimized implementation.
func makeMapColumnMajorReference(world *World, x1, y1, x2, y2 int) {
	for x := x1; x <= x2; x++ {
		for y := y1; y <= y2; y++ {
			pos := x + y*MapWidth
			altPos := x + EndWidth*y
			avg := (world.Alt[altPos] + world.Alt[altPos+1] + world.Alt[altPos+EndWidth] + world.Alt[altPos+EndWidth+1]) >> 2
			keepRock := int(world.MapBlk[pos]) == RockBlock
			block := 0
			if world.Alt[altPos] > avg {
				block++
			}
			if world.Alt[altPos+1] > avg {
				block += 2
			}
			if world.Alt[altPos+EndWidth+1] > avg {
				block += 4
			}
			if world.Alt[altPos+EndWidth] > avg {
				block += 8
			}
			if keepRock && !(block == 0 && avg == 0) {
				block = RockBlock
			}
			if avg != 0 && block == 0 {
				avg--
				block = FlatBlock
			}
			if avg == 0 && block != FlatBlock && block != WaterBlock {
				block += 16
			}
			world.MapAlt[pos] = byte(avg)
			if keepRock && !(block == WaterBlock && avg == 0) {
				block = RockBlock
			}
			world.MapBlk[pos] = byte(block)
			if block == WaterBlock {
				world.MapBk2[pos] = 0
			}
			world.MapSteps[pos] = 0
		}
	}
}

var benchmarkPopulationResult [2]int
var benchmarkSnapshotResult WorldSnapshot

func BenchmarkPlayerPopulationsMaxPeeps(b *testing.B) {
	world := benchmarkMaxPeepWorld()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkPopulationResult = world.PlayerPopulations()
	}
}

func BenchmarkTickWithComputerHumanHumanMaxPeeps(b *testing.B) {
	world := benchmarkMaxPeepWorld()
	world.TickWithComputer([2]bool{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		world.TickWithComputer([2]bool{})
		world.SoundEvents = world.SoundEvents[:0]
	}
}

func BenchmarkUpdateComputerStatsMaxPeeps(b *testing.B) {
	world := benchmarkMaxPeepWorld()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		world.updateComputerStats()
	}
}

func BenchmarkMakeMapFullRowMajor(b *testing.B) {
	world := benchmarkMaxPeepWorld()
	for i := range world.Alt {
		world.Alt[i] = (i*3 + i/EndWidth) % 9
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		world.makeMap(0, 0, MapWidth-1, MapHeight-1)
	}
}

func BenchmarkMakeMapFullColumnMajorReference(b *testing.B) {
	world := benchmarkMaxPeepWorld()
	for i := range world.Alt {
		world.Alt[i] = (i*3 + i/EndWidth) % 9
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		makeMapColumnMajorReference(world, 0, 0, MapWidth-1, MapHeight-1)
	}
}

func BenchmarkSnapshotMaxPeeps(b *testing.B) {
	world := benchmarkMaxPeepWorld()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSnapshotResult = world.Snapshot()
	}
}

func benchmarkMaxPeepWorld() *World {
	world := &World{
		Level:       Level{PlayerPopulation: 30, EnemyPopulation: 30},
		Rules:       DefaultTerrainRules(),
		Peeps:       make([]Peep, MaxPeeps, MaxPeeps),
		ScorePlayer: GodPlayer,
	}
	for pos := range world.MapBlk {
		world.MapBlk[pos] = FlatBlock
	}
	for i := range world.Peeps {
		world.Peeps[i] = Peep{
			Flags:            InRuin,
			Player:           byte(i & 1),
			Population:       i + 1,
			BattlePopulation: 1 << 30,
			AtPos:            i,
		}
	}
	return world
}
