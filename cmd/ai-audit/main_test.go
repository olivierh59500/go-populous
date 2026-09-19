package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"go-populous/internal/assets"
	"go-populous/internal/populous"
)

func TestWorldSelection(t *testing.T) {
	for _, tc := range []struct {
		selector string
		want     []int
	}{
		{"all", []int{0, 1, 2, 3, 4}},
		{"4,1-3,2,0", []int{0, 1, 2, 3, 4}},
		{" 1, 3-4 ", []int{1, 3, 4}},
		{"0", []int{0}},
	} {
		got, err := selectWorlds(tc.selector, 5)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("selectWorlds(%q) = %v, %v; want %v", tc.selector, got, err, tc.want)
		}
	}
	for _, selector := range []string{"", "-1", "5", "2-1", "0-5", "0-1-2", "1,", "all,1", "one"} {
		if got, err := selectWorlds(selector, 5); err == nil {
			t.Errorf("selectWorlds(%q) accepted invalid selector: %v", selector, got)
		}
	}
}

func TestOptionsValidateDurationAndExecution(t *testing.T) {
	good, err := parseOptions([]string{"-output", "result.csv", "-side", "both", "-duration", "125ms", "-workers", "2"}, io.Discard)
	if err != nil || good.duration != tickDuration || good.side != "both" || good.workers != 2 {
		t.Fatalf("valid options = %+v, %v", good, err)
	}
	for _, args := range [][]string{
		{}, {"-output", "x", "-side", "2"}, {"-output", "x", "-workers", "0"},
		{"-output", "x", "-workers", "65"}, {"-output", "x", "-duration", "0s"},
		{"-output", "x", "-duration", "-1s"}, {"-output", "x", "-duration", "10ms"},
		{"-output", "x", "-duration", "25h"}, {"-output", "x", "unexpected"},
	} {
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Errorf("accepted invalid options: %v", args)
		}
	}
	defaults, err := parseOptions([]string{"-output", "result.csv"}, io.Discard)
	if err != nil || defaults.duration != 20*time.Minute || defaults.side != "0" {
		t.Fatalf("defaults = %+v, %v", defaults, err)
	}
}

func TestEliminationExcludesRuinsAndPopulationAdvantages(t *testing.T) {
	w := &populous.World{Peeps: []populous.Peep{{Player: 0, Population: 32000}, {Player: 1, Population: 1}}}
	if got := outcome(w, 0); got != "ongoing" {
		t.Fatalf("population advantage is not victory: %s", got)
	}
	w.Peeps[1].Flags = populous.InRuin
	if got := outcome(w, 0); got != "win" {
		t.Fatalf("a ruin cannot keep its camp alive: %s", got)
	}
	if got := outcome(w, 1); got != "loss" {
		t.Fatalf("opposite viewpoint should lose: %s", got)
	}
	w.Peeps[0].Population = 0
	if got := outcome(w, 0); got != "draw" {
		t.Fatalf("simultaneous elimination should draw: %s", got)
	}
}

func TestAuditDeterministicAcrossWorkers(t *testing.T) {
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	matches := []match{{bundle.Levels[200], 1}, {bundle.Levels[0], 0}, {bundle.Levels[200], 0}, {bundle.Levels[10], 0}}
	one := audit(context.Background(), matches, bundle.TerrainRules, 256, 1, nil)
	many := audit(context.Background(), matches, bundle.TerrainRules, 256, 4, nil)
	if !reflect.DeepEqual(one, many) {
		t.Fatal("worker count changed simulation results or result order")
	}
	if len(one) != len(matches) || one[0].level.Number != 0 || one[2].side != 0 || one[3].side != 1 {
		t.Fatalf("results are not ordered by world then side: %+v", one)
	}
	for _, r := range one {
		if r.ticks != 256 || r.outcome != "ongoing" {
			t.Fatalf("short audit did not honor simulation limit: ticks=%d result=%s", r.ticks, r.outcome)
		}
		world := populous.GenerateWorldWithRules(r.level, bundle.TerrainRules[r.level.Terrain])
		if r.initialPopulation != world.PlayerPopulations() || r.initialMana != [2]int{world.Magnets[0].Mana, world.Magnets[1].Mana} {
			t.Fatal("audit changed original initial resources")
		}
		if r.livingSlots[0]+r.livingSlots[1]+r.deadSlots+r.ruinSlots != r.slots {
			t.Fatal("slot counters do not partition the world")
		}
		for player := range r.actualSummary {
			if r.actualSummary[player].Population != r.population[player] {
				t.Fatalf("actual summary population=%d, terminal population=%d", r.actualSummary[player].Population, r.population[player])
			}
			if r.peakActualSummary[player].Population < r.actualSummary[player].Population ||
				r.peakActualSummary[player].Towns < r.actualSummary[player].Towns ||
				r.peakActualSummary[player].Castles < r.actualSummary[player].Castles ||
				r.peakActualSummary[player].Knights < r.actualSummary[player].Knights {
				t.Fatal("actual summary peak is below the terminal summary")
			}
			if r.unclassifiedPowerScore[player] != 0 {
				t.Fatalf("unexpected power score delta for player %d: %d", player, r.unclassifiedPowerScore[player])
			}
		}
	}
	var first, second bytes.Buffer
	if err := writeCSV(&first, one); err != nil {
		t.Fatal(err)
	}
	if err := writeCSV(&second, many); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("CSV changed with worker count")
	}
	rows, err := csv.NewReader(&first).ReadAll()
	if err != nil || len(rows) != len(matches)+1 {
		t.Fatalf("CSV shape invalid: rows=%d error=%v", len(rows), err)
	}
	if len(rows[0]) <= 43 || rows[0][42] != "state_hash" || rows[0][43] != "blue_exact_quakes" {
		t.Fatalf("legacy CSV prefix changed or exact metrics were not appended: %v", rows[0])
	}
}

func TestScoreDeltasAttributeEveryPowerToEachPlayer(t *testing.T) {
	var got result
	deltas := []int{
		populous.ScoreQuake,
		populous.ScoreSwamp,
		populous.ScoreKnight,
		populous.ScoreVolcano,
		populous.ScoreFlood,
		populous.ScoreWar,
	}
	for player := 0; player < 2; player++ {
		for _, delta := range deltas {
			before := [2]int{1000, 2000}
			after := before
			after[player] += delta
			got.observeScoreDeltas(before, after)
		}
	}
	for player, counts := range got.spells {
		if counts != (spellCounts{quakes: 1, swamps: 1, knights: 1, volcanoes: 1, floods: 1, wars: 1}) {
			t.Fatalf("player %d spell counts = %+v", player, counts)
		}
	}

	before := [2]int{10, 20}
	got.observeScoreDeltas(before, [2]int{85, 20})
	got.observeScoreDeltas(before, before)
	if got.unclassifiedPowerScore != [2]int{75, 0} {
		t.Fatalf("unclassified score deltas = %v, want [75 0]", got.unclassifiedPowerScore)
	}
}

func TestActualSummariesTrackTerminalValuesAndPeaks(t *testing.T) {
	w := &populous.World{
		BattleWon: [2]int{1, 2},
		Peeps: []populous.Peep{
			{Player: 0, Population: 100, Flags: populous.InTown, Frame: populous.FirstTown},
			{Player: 0, Population: 200, Flags: populous.InTown, Frame: populous.LastTown},
			{Player: 0, Population: 300, Flags: populous.OnMove, HeadFor: 2},
			{Player: 1, Population: 400, Flags: populous.InTown, Frame: populous.FirstTown},
		},
	}
	var got result
	got.observeSummaries(w)
	if got.actualSummary[0] != (populous.PlayerSummary{Population: 600, BattlesWon: 1, Knights: 1, Towns: 1, Castles: 1}) {
		t.Fatalf("initial blue summary = %+v", got.actualSummary[0])
	}
	if got.actualSummary[1] != (populous.PlayerSummary{Population: 400, BattlesWon: 2, Towns: 1}) {
		t.Fatalf("initial red summary = %+v", got.actualSummary[1])
	}

	w.Peeps[0].Population = 0
	w.Peeps[1].Flags = populous.OnMove
	w.Peeps[2].Population = 50
	w.BattleWon[0] = 3
	got.observeSummaries(w)
	if got.actualSummary[0] != (populous.PlayerSummary{Population: 250, BattlesWon: 3, Knights: 1}) {
		t.Fatalf("terminal blue summary = %+v", got.actualSummary[0])
	}
	if got.peakActualSummary[0] != (populous.PlayerSummary{Population: 600, BattlesWon: 3, Knights: 1, Towns: 1, Castles: 1}) {
		t.Fatalf("blue summary peaks = %+v", got.peakActualSummary[0])
	}
}

func TestCSVWritesAppendedExactMetrics(t *testing.T) {
	r := result{}
	r.spells[0] = spellCounts{quakes: 1, swamps: 2, knights: 3, volcanoes: 4, floods: 5, wars: 6}
	r.spells[1] = spellCounts{quakes: 7, swamps: 8, knights: 9, volcanoes: 10, floods: 11, wars: 12}
	r.unclassifiedPowerScore = [2]int{13, 14}
	r.actualSummary = [2]populous.PlayerSummary{
		{Towns: 15, Castles: 16, Knights: 17, BattlesWon: 18},
		{Towns: 19, Castles: 20, Knights: 21, BattlesWon: 22},
	}
	r.peakActualSummary = [2]populous.PlayerSummary{
		{Population: 23, Towns: 25, Castles: 27, Knights: 29, BattlesWon: 31},
		{Population: 24, Towns: 26, Castles: 28, Knights: 30, BattlesWon: 32},
	}
	var output bytes.Buffer
	if err := writeCSV(&output, []result{r}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&output).ReadAll()
	if err != nil || len(rows) != 2 || len(rows[0]) != len(rows[1]) {
		t.Fatalf("CSV shape rows=%d header=%d data=%d err=%v", len(rows), len(rows[0]), len(rows[1]), err)
	}
	columns := make(map[string]string, len(rows[0]))
	for i, name := range rows[0] {
		columns[name] = rows[1][i]
	}
	for name, want := range map[string]string{
		"blue_exact_quakes":             "1",
		"red_exact_swamps":              "8",
		"blue_exact_wars":               "6",
		"red_exact_wars":                "12",
		"blue_unclassified_power_score": "13",
		"red_actual_castles":            "20",
		"blue_actual_battles_won":       "18",
		"red_peak_actual_population":    "24",
		"blue_peak_actual_towns":        "25",
		"red_peak_actual_knights":       "30",
		"red_peak_actual_battles_won":   "32",
	} {
		if columns[name] != want {
			t.Errorf("%s=%q, want %q", name, columns[name], want)
		}
	}
}

func TestAuditCancellationDoesNotCountIncompleteWorlds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	matches := []match{{level: populous.Level{PlayerPopulation: 1, EnemyPopulation: 1}}}
	results := audit(ctx, matches, []populous.TerrainRules{populous.DefaultTerrainRules()}, 9600, 2, nil)
	if len(results) != 0 {
		t.Fatal("cancelled match was recorded as an outcome")
	}
}

func TestExistingOutputIsPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.csv")
	const content = "previous audit\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-output", path, "-worlds", "0", "-duration", "125ms"}, io.Discard); err == nil {
		t.Fatal("audit accepted an existing output file")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != content {
		t.Fatalf("existing file modified: %q, %v", got, err)
	}
}
