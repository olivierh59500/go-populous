// ai-audit runs the original campaign without graphics and records actual
// eliminations. It never changes resources, rules, powers, or action cadence.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-populous/internal/assets"
	"go-populous/internal/populous"
)

const tickDuration = time.Second / 8

type options struct {
	output, worlds, side string
	workers              int
	duration             time.Duration
}

type match struct {
	level populous.Level
	side  int
}

type result struct {
	match
	outcome                                               string
	ticks, warTick, firstOffenseTick, lastOffenseTick     int
	population, livingPopulation, mana, initialPopulation [2]int
	initialMana, castles, peakCastles, livingSlots        [2]int
	towns, actionMarkers                                  [2]int
	deadSlots, ruinSlots, slots                           int
	// These audible events combine both camps, not just the advanced computer.
	quakes, volcanoes, floods, knights, wars int
	hash                                     [32]byte
}

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "ai-audit:", err)
			os.Exit(1)
		}
	}
}

func parseOptions(args []string, diagnostics io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("ai-audit", flag.ContinueOnError)
	fs.SetOutput(diagnostics)
	fs.StringVar(&o.output, "output", "", "new CSV file (required; existing files are never overwritten)")
	fs.StringVar(&o.worlds, "worlds", "all", "original world numbers: all, 0,5,25, or 0-19,200")
	fs.StringVar(&o.side, "side", "0", "strategic side: 0 (God/blue), 1 (Devil/red), or both")
	fs.IntVar(&o.workers, "workers", min(4, runtime.GOMAXPROCS(0)), "parallel matches (1-64)")
	fs.DurationVar(&o.duration, "duration", 20*time.Minute, "simulation time limit, a positive multiple of 125ms")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if o.output == "" {
		return o, errors.New("-output is required")
	}
	if o.workers < 1 || o.workers > 64 {
		return o, errors.New("-workers must be between 1 and 64")
	}
	if o.duration <= 0 || o.duration%tickDuration != 0 || o.duration > 24*time.Hour {
		return o, errors.New("-duration must be a positive multiple of 125ms, no more than 24h")
	}
	if o.side != "0" && o.side != "1" && o.side != "both" {
		return o, errors.New("-side must be 0, 1, or both")
	}
	return o, nil
}

func selectWorlds(selector string, count int) ([]int, error) {
	selected := make(map[int]bool)
	selector = strings.TrimSpace(selector)
	if selector == "all" {
		for n := 0; n < count; n++ {
			selected[n] = true
		}
	} else {
		for _, field := range strings.Split(selector, ",") {
			parts := strings.Split(strings.TrimSpace(field), "-")
			first, err := strconv.Atoi(parts[0])
			if err != nil || len(parts) > 2 {
				return nil, fmt.Errorf("invalid world selection %q", field)
			}
			last := first
			if len(parts) == 2 {
				last, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
			if err != nil || first < 0 || last < first || last >= count {
				return nil, fmt.Errorf("world selection %q outside 0-%d or reversed", field, count-1)
			}
			for n := first; n <= last; n++ {
				selected[n] = true
			}
		}
	}
	worlds := make([]int, 0, len(selected))
	for n := range selected {
		worlds = append(worlds, n)
	}
	sort.Ints(worlds)
	return worlds, nil
}

func run(args []string, diagnostics io.Writer) error {
	o, err := parseOptions(args, diagnostics)
	if err != nil {
		return err
	}
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		return err
	}
	if len(bundle.Warnings) != 0 {
		return fmt.Errorf("embedded asset warnings: %s", strings.Join(bundle.Warnings, "; "))
	}
	worlds, err := selectWorlds(o.worlds, len(bundle.Levels))
	if err != nil {
		return err
	}
	sides := []int{0}
	if o.side == "1" {
		sides = []int{1}
	} else if o.side == "both" {
		sides = []int{0, 1}
	}
	matches := make([]match, 0, len(worlds)*len(sides))
	for _, n := range worlds {
		level := bundle.Levels[n]
		if int(level.Terrain) >= len(bundle.TerrainRules) {
			return fmt.Errorf("world %d has missing terrain %d", n, level.Terrain)
		}
		for _, side := range sides {
			matches = append(matches, match{level: level, side: side})
		}
	}
	file, err := os.OpenFile(o.output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now()
	counts := make(map[string]int)
	fmt.Fprintf(diagnostics, "matches=%d workers=%d limit=%s strategic_side=%s\n", len(matches), o.workers, o.duration, o.side)
	results := audit(ctx, matches, bundle.TerrainRules, int(o.duration/tickDuration), o.workers, func(r result, completed int) {
		counts[r.outcome]++
		if completed%10 == 0 || completed == len(matches) {
			fmt.Fprintf(diagnostics, "completed=%d/%d win=%d loss=%d draw=%d ongoing=%d elapsed=%s\n", completed, len(matches), counts["win"], counts["loss"], counts["draw"], counts["ongoing"], time.Since(started).Round(time.Second))
		}
	})
	if err := writeCSV(file, results); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w; %d complete matches saved to %s", err, len(results), o.output)
	}
	fmt.Fprintf(diagnostics, "saved=%s matches=%d win=%d loss=%d draw=%d ongoing=%d elapsed=%s\n", o.output, len(results), counts["win"], counts["loss"], counts["draw"], counts["ongoing"], time.Since(started).Round(time.Second))
	return nil
}

// Workers own independent worlds. Sorting makes the data independent of worker
// completion order; only the progress log contains nondeterministic wall time.
func audit(ctx context.Context, matches []match, rules []populous.TerrainRules, limit, workers int, progress func(result, int)) []result {
	jobs := make(chan match, len(matches))
	completed := make(chan result, workers)
	for _, m := range matches {
		jobs <- m
	}
	close(jobs)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range jobs {
				r, ok := simulate(ctx, m, rules[m.level.Terrain], limit)
				if !ok {
					return
				}
				completed <- r
			}
		}()
	}
	go func() {
		wg.Wait()
		close(completed)
	}()
	results := make([]result, 0, len(matches))
	for r := range completed {
		results = append(results, r)
		if progress != nil {
			progress(r, len(results))
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].level.Number == results[j].level.Number {
			return results[i].side < results[j].side
		}
		return results[i].level.Number < results[j].level.Number
	})
	return results
}

func outcome(w *populous.World, side int) string {
	var alive [2]bool
	for _, p := range w.Peeps {
		if p.Population > 0 && p.Flags&populous.InRuin == 0 && p.Player < 2 {
			alive[p.Player] = true
		}
	}
	if alive[0] && alive[1] {
		return "ongoing"
	}
	if !alive[0] && !alive[1] {
		return "draw"
	}
	if alive[side] {
		return "win"
	}
	return "loss"
}

func simulate(ctx context.Context, m match, rules populous.TerrainRules, limit int) (result, bool) {
	w := populous.GenerateWorldWithRules(m.level, rules)
	r := result{match: m, initialPopulation: w.PlayerPopulations(), initialMana: [2]int{w.Magnets[0].Mana, w.Magnets[1].Mana}}
	for w.GameTurn < limit && outcome(w, m.side) == "ongoing" {
		if w.GameTurn%64 == 0 && ctx.Err() != nil {
			return r, false
		}
		w.TickWithAdvancedComputer(m.side)
		for player := range w.Computer {
			if w.Computer[player].DoneTurn == w.GameTurn {
				r.actionMarkers[player]++
			}
			r.peakCastles[player] = max(r.peakCastles[player], w.Computer[player].NoCastles)
		}
		if w.War && r.warTick == 0 {
			r.warTick = w.GameTurn
		}
		for _, sound := range w.DrainSoundEvents() {
			switch sound {
			case populous.TuneQuake:
				r.quakes++
			case populous.TuneVolcano:
				r.volcanoes++
			case populous.TuneFlood:
				r.floods++
			case populous.TuneKnighted:
				r.knights++
			case populous.TuneWar:
				r.wars++
			default:
				continue
			}
			if r.firstOffenseTick == 0 {
				r.firstOffenseTick = w.GameTurn
			}
			r.lastOffenseTick = w.GameTurn
		}
	}
	r.outcome = outcome(w, m.side)
	r.ticks = w.GameTurn
	r.population = w.PlayerPopulations()
	r.slots = len(w.Peeps)
	for player := range w.Computer {
		r.mana[player] = w.Magnets[player].Mana
		r.castles[player] = w.Computer[player].NoCastles
		r.towns[player] = w.Computer[player].NoTowns
	}
	for _, p := range w.Peeps {
		if p.Population <= 0 {
			r.deadSlots++
			continue
		}
		if p.Flags&populous.InRuin != 0 {
			r.ruinSlots++
			continue
		}
		if p.Player < 2 {
			r.livingSlots[p.Player]++
			r.livingPopulation[p.Player] += p.Population
		}
	}
	r.hash = w.StateHash()
	return r, true
}

func writeCSV(out io.Writer, results []result) error {
	w := csv.NewWriter(out)
	if err := w.Write([]string{"level", "code", "terrain", "game_mode", "player_powers", "enemy_powers", "speed", "strategic_side", "outcome", "ticks", "seconds", "blue_population", "red_population", "blue_living_population", "red_living_population", "blue_mana", "red_mana", "blue_castles", "red_castles", "blue_peak_castles", "red_peak_castles", "blue_towns", "red_towns", "blue_action_markers", "red_action_markers", "blue_alive_slots", "red_alive_slots", "dead_slots", "ruins", "slots", "war_tick", "first_offense_tick", "last_offense_tick", "both_quakes", "both_volcanoes", "both_floods", "both_knights", "both_wars", "blue_initial_population", "red_initial_population", "blue_initial_mana", "red_initial_mana", "state_hash"}); err != nil {
		return err
	}
	for _, r := range results {
		row := []string{strconv.Itoa(r.level.Number), r.level.Code, strconv.Itoa(int(r.level.Terrain)), strconv.Itoa(int(r.level.GameMode)), strconv.Itoa(int(r.level.PlayerPowers)), strconv.Itoa(int(r.level.EnemyPowers)), strconv.Itoa(int(r.level.EnemyReactionSpeed)), strconv.Itoa(r.side), r.outcome, strconv.Itoa(r.ticks), strconv.FormatFloat(float64(r.ticks)/8, 'f', 3, 64)}
		for _, n := range []int{r.population[0], r.population[1], r.livingPopulation[0], r.livingPopulation[1], r.mana[0], r.mana[1], r.castles[0], r.castles[1], r.peakCastles[0], r.peakCastles[1], r.towns[0], r.towns[1], r.actionMarkers[0], r.actionMarkers[1], r.livingSlots[0], r.livingSlots[1], r.deadSlots, r.ruinSlots, r.slots, r.warTick, r.firstOffenseTick, r.lastOffenseTick, r.quakes, r.volcanoes, r.floods, r.knights, r.wars, r.initialPopulation[0], r.initialPopulation[1], r.initialMana[0], r.initialMana[1]} {
			row = append(row, strconv.Itoa(n))
		}
		row = append(row, fmt.Sprintf("%x", r.hash))
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
