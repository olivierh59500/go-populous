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
