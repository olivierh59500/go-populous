package populous

import (
	"errors"
	"testing"
)

func TestApplyCommandRejectsMalformedCommandWithoutMutation(t *testing.T) {
	world := GenerateWorld(Level{
		Number:             3,
		SeedOffset:         1234,
		PlayerPopulation:   2,
		EnemyPopulation:    2,
		PlayerPowers:       0x3f,
		EnemyPowers:        0x3f,
		EnemyRating:        3,
		EnemyReactionSpeed: 3,
	})
	before := world.StateHash()

	applied, err := world.ApplyCommand(Command{
		Kind:   CommandSetMagnet,
		Player: GodPlayer,
		X:      MapWidth,
		Y:      0,
	})
	if applied {
		t.Fatal("malformed command was applied")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("ApplyCommand error = %v, want ErrInvalidCommand", err)
	}
	if after := world.StateHash(); after != before {
		t.Fatalf("malformed command mutated world: before=%x after=%x", before, after)
	}
}

func TestApplyCommandCoversPlayerMutations(t *testing.T) {
	world := GenerateWorld(Level{
		Number:             8,
		SeedOffset:         9876,
		PlayerPopulation:   4,
		EnemyPopulation:    4,
		PlayerPowers:       0x3f,
		EnemyPowers:        0x3f,
		EnemyRating:        3,
		EnemyReactionSpeed: 3,
	})
	world.Magnets[GodPlayer].Mana = 500000

	commands := []Command{
		{Kind: CommandRaise, Player: GodPlayer, X: 32, Y: 32},
		{Kind: CommandLower, Player: GodPlayer, X: 32, Y: 32},
		{Kind: CommandPaintRaise, Player: GodPlayer, X: 20, Y: 20},
		{Kind: CommandPaintLower, Player: GodPlayer, X: 20, Y: 20},
		{Kind: CommandSetMagnet, Player: GodPlayer, X: 30, Y: 30},
		{Kind: CommandSetTendency, Player: GodPlayer, Value: JoinMode},
		{Kind: CommandSwamp, Player: GodPlayer, X: 25, Y: 25},
		{Kind: CommandQuake, Player: GodPlayer, X: 10, Y: 10},
		{Kind: CommandVolcano, Player: GodPlayer, X: 40, Y: 40},
		{Kind: CommandFlood, Player: GodPlayer},
		{Kind: CommandKnight, Player: GodPlayer},
		{Kind: CommandArmageddon, Player: GodPlayer},
	}

	for _, command := range commands {
		t.Run(command.Kind.String(), func(t *testing.T) {
			if _, err := world.ApplyCommand(command); err != nil {
				t.Fatalf("ApplyCommand(%s) returned validation error: %v", command.Kind, err)
			}
		})
	}
}

func TestStateHashIsCanonicalAndIgnoresLocalScore(t *testing.T) {
	level := Level{
		Number:             12,
		Code:               "HASHME",
		SeedOffset:         4321,
		PlayerPopulation:   3,
		EnemyPopulation:    3,
		PlayerPowers:       0x3f,
		EnemyPowers:        0x3f,
		EnemyRating:        4,
		EnemyReactionSpeed: 5,
	}
	left := GenerateWorld(level)
	right := GenerateWorld(level)
	if left.StateHash() != right.StateHash() {
		t.Fatal("identically generated worlds have different state hashes")
	}

	left.Score = 1000
	left.ScorePlayer = GodPlayer
	right.Score = 9000
	right.ScorePlayer = DevilPlayer
	if left.StateHash() != right.StateHash() {
		t.Fatal("local Score/ScorePlayer changed shared simulation hash")
	}

	right.Scores[DevilPlayer]++
	if left.StateHash() == right.StateHash() {
		t.Fatal("canonical score mutation did not change state hash")
	}
	right.Scores[DevilPlayer]--
	right.Magnets[DevilPlayer].Mana++
	if left.StateHash() == right.StateHash() {
		t.Fatal("shared simulation mutation did not change state hash")
	}
}

func TestSnapshotRoundTripKeepsCanonicalStateHash(t *testing.T) {
	rules := DefaultTerrainRules()
	world := GenerateWorldWithRules(Level{
		Number:             5,
		SeedOffset:         2468,
		PlayerPopulation:   2,
		EnemyPopulation:    2,
		EnemyRating:        3,
		EnemyReactionSpeed: 4,
	}, rules)
	for range 6 {
		world.TickWithComputer([2]bool{})
	}
	restored := WorldFromSnapshot(world.Snapshot(), rules)
	if got, want := restored.StateHash(), world.StateHash(); got != want {
		t.Fatalf("snapshot hash = %x, want %x", got, want)
	}
}
