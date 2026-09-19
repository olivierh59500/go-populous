package attract

import (
	"testing"

	embeddedassets "go-populous/assets"
	"go-populous/internal/populous"
)

func campaignLevels(t *testing.T) []populous.Level {
	t.Helper()
	file, err := embeddedassets.Files.Open("amiga/level.dat")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	levels, err := populous.LoadLevels(file)
	if err != nil {
		t.Fatal(err)
	}
	return levels
}

func TestSelectorUsesWholeOriginalCampaignWithoutConsecutiveWorlds(t *testing.T) {
	levels := campaignLevels(t)
	original := append([]populous.Level(nil), levels...)
	selector := NewSelector(levels, 20260918)
	var previous populous.Level
	profiles := make(map[[3]byte]bool)
	terrains := make(map[byte]bool)
	for deck := range 3 {
		seen := make(map[populous.Level]bool)
		for index := range len(levels) {
			level, player := selector.Next()
			if player != populous.GodPlayer {
				t.Fatal("historical enemy was moved to the human side")
			}
			if seen[level] {
				t.Fatalf("level %d repeated before the deck was exhausted", level.Number)
			}
			if deck+index > 0 && landscapeFor(level) == landscapeFor(previous) {
				t.Fatalf("consecutive demos reused landscape: previous=%+v current=%+v", previous, level)
			}
			seen[level] = true
			profiles[[3]byte{level.EnemyRating, level.EnemyReactionSpeed, level.EnemyPowers}] = true
			terrains[level.Terrain] = true
			previous = level
		}
		for _, level := range original {
			if !seen[level] {
				t.Fatalf("original level %d missing or altered", level.Number)
			}
		}
	}
	if len(profiles) < 2 || len(terrains) != 4 {
		t.Fatalf("campaign variation lost: %d enemy profiles, %d terrains", len(profiles), len(terrains))
	}
	for i := range levels {
		if levels[i] != original[i] {
			t.Fatal("selector changed the campaign catalog")
		}
	}
}

func TestSelectorSeedReproducesRounds(t *testing.T) {
	levels := campaignLevels(t)
	left, right := NewSelector(levels, 77), NewSelector(levels, 77)
	different := NewSelector(levels, 78)
	diverged := false
	for range 2 * len(levels) {
		a, ap := left.Next()
		b, bp := right.Next()
		c, _ := different.Next()
		if a != b || ap != bp {
			t.Fatal("same seed did not reproduce demo sequence")
		}
		diverged = diverged || a != c
	}
	if !diverged {
		t.Fatal("different seeds always select the same sequence")
	}
}

func TestSelectorEmptyAndSingleCatalogRemainPlayable(t *testing.T) {
	for _, levels := range [][]populous.Level{nil, {populous.TutorialLevel()}} {
		selector := NewSelector(levels, 0)
		for range 3 {
			level, player := selector.Next()
			if level != populous.TutorialLevel() || player != populous.GodPlayer {
				t.Fatalf("invalid single-world fallback: %+v player=%d", level, player)
			}
		}
	}
}
