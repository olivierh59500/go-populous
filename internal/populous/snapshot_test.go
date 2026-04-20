package populous

import "testing"

func TestWorldSnapshotRestoresDynamicState(t *testing.T) {
	world := GenerateWorld(Level{
		Number:             7,
		Terrain:            1,
		SeedOffset:         0x1234,
		PlayerPopulation:   2,
		EnemyPopulation:    2,
		PlayerPowers:       0x3f,
		EnemyPowers:        0x3f,
		EnemyRating:        4,
		EnemyReactionSpeed: 5,
	})
	world.War = true
	world.ScorePlayer = DevilPlayer
	for i := 0; i < 3; i++ {
		world.Tick()
	}
	world.ComputerControlled = [2]bool{true, true}
	world.Magnets[GodPlayer].Mana = 12345
	snapshot := world.Snapshot()
	originalPopulation := snapshot.Peeps[0].Population
	world.Peeps[0].Population = 0

	restored := WorldFromSnapshot(snapshot, DefaultTerrainRules())

	if restored.GameTurn != snapshot.GameTurn {
		t.Fatalf("GameTurn = %d, want %d", restored.GameTurn, snapshot.GameTurn)
	}
	if restored.Terrain != snapshot.Terrain {
		t.Fatalf("Terrain = %d, want %d", restored.Terrain, snapshot.Terrain)
	}
	if restored.Peeps[0].Population != originalPopulation {
		t.Fatalf("restored peep population = %d, want %d", restored.Peeps[0].Population, originalPopulation)
	}
	if restored.Magnets[GodPlayer].Mana != 12345 {
		t.Fatalf("restored mana = %d, want 12345", restored.Magnets[GodPlayer].Mana)
	}
	if restored.War != snapshot.War || restored.rng != lcg(snapshot.RNG) {
		t.Fatalf("restored transient state mismatch: war=%v rng=%d", restored.War, restored.rng)
	}
	if restored.ScorePlayer != DevilPlayer || restored.ComputerControlled != [2]bool{true, true} {
		t.Fatalf("restored control state = score player %d computer %v", restored.ScorePlayer, restored.ComputerControlled)
	}
}
