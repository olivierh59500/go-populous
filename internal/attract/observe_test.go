package attract

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"go-populous/internal/assets"
	"go-populous/internal/populous"
)

// An opt-in observational run helps choose a complete match to export without
// changing its original powers, resources, or outcome.
func TestObserveCampaignDemos(t *testing.T) {
	selection := os.Getenv("POPULOUS_DEMO_OBSERVE")
	if selection == "" {
		t.Skip("set POPULOUS_DEMO_OBSERVE to comma-separated world indices")
	}
	seconds := 15 * 60
	if value := os.Getenv("POPULOUS_DEMO_MAX_SECONDS"); value != "" {
		var err error
		seconds, err = strconv.Atoi(value)
		if err != nil || seconds <= 0 {
			t.Fatalf("invalid POPULOUS_DEMO_MAX_SECONDS %q", value)
		}
	}
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range strings.Split(selection, ",") {
		index, err := strconv.Atoi(item)
		if err != nil || index < 0 || index >= len(bundle.Levels) {
			t.Fatalf("invalid world %q", item)
		}
		t.Run(item, func(t *testing.T) {
			t.Parallel()
			level := bundle.Levels[index]
			session := NewLevelSession(level, bundle.TerrainRules[level.Terrain], 0, populous.GodPlayer)
			var peaks [2]int
			var events [128]int
			for !session.Finished && session.ElapsedTicks < seconds*TicksPerSecond {
				session.Tick()
				for player := range peaks {
					peaks[player] = max(peaks[player], session.World.Computer[player].NoCastles)
				}
				for _, event := range session.World.DrainSoundEvents() {
					if event >= 0 && event < len(events) {
						events[event]++
					}
				}
			}
			world := session.World
			t.Logf("world=%d code=%s terrain=%d powers=%02x/%02x speed=%d finished=%v winner=%d seconds=%.3f pop=%v peak_castles=%v mana=%d/%d quake=%d swamp=%d knight=%d volcano=%d flood=%d war=%d", level.Number, level.Code, level.Terrain, level.PlayerPowers, level.EnemyPowers, level.EnemyReactionSpeed, session.Finished, session.Winner, float64(session.ElapsedTicks)/TicksPerSecond, world.PlayerPopulations(), peaks, world.Magnets[0].Mana, world.Magnets[1].Mana, events[populous.TuneQuake], events[populous.TuneSwamp], events[populous.TuneKnighted], events[populous.TuneVolcano], events[populous.TuneFlood], events[populous.TuneWar])
		})
	}
}
