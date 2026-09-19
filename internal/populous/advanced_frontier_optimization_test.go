package populous

import (
	"fmt"
	"testing"
)

// Preserve the pre-optimization evaluator as an independent oracle: the
// production version skips/caches candidates but must select the same action.
func referenceFrontierLand(w *World, player int) bool {
	if w.Computer[player].Mode&computerLand == 0 || w.Level.GameMode&GameNoBuild != 0 || w.Magnets[player].Mana < 100 || w.War {
		return false
	}
	bestScore, bestX, bestY, bestRaise := 0, 0, 0, false
	consider := func(x, y int, raise bool, base int) {
		if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
			return
		}
		trial := *w // Terrain and mana are value fields; never simulate RNG.
		if !trial.advancedSculpt(player, x, y, raise) || trial.Magnets[player].Mana < 0 || w.advancedDamagesTown(&trial, player) {
			return
		}
		score := base - (w.Magnets[player].Mana - trial.Magnets[player].Mana)
		for _, p := range w.Peeps {
			if p.Population <= 0 || p.Flags&InRuin != 0 || !inMap(p.AtPos) {
				continue
			}
			if int(p.Player) == player {
				if trial.MapBlk[p.AtPos] == WaterBlock && w.MapBlk[p.AtPos] != WaterBlock {
					return
				}
			} else {
				if trial.MapBlk[p.AtPos] == WaterBlock && w.MapBlk[p.AtPos] != WaterBlock {
					score += p.Population*2 + 1000
				}
				if p.Flags == InTown {
					score += max(0, w.checkLife(player^1, p.AtPos)-trial.checkLife(player^1, p.AtPos))
				}
			}
		}
		if score > bestScore {
			bestScore, bestX, bestY, bestRaise = score, x, y, raise
		}
	}
	for i, p := range w.Peeps {
		if p.Population <= 0 || p.Flags&InRuin != 0 || !inMap(p.AtPos) {
			continue
		}
		x, y := p.AtPos%MapWidth, p.AtPos/MapWidth
		if int(p.Player) != player && p.Flags&InWater == 0 {
			for _, d := range [...][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				consider(x+d[0], y+d[1], false, 0)
			}
		} else if int(p.Player) == player && (isHeadedPeep(p) || (w.Magnets[player].Flags == MagnetMode && w.Magnets[player].Carried == i+1)) {
			target := w.closestEnemy(i)
			if target < 0 {
				continue
			}
			nx, ny := x+sign(w.Peeps[target].AtPos%MapWidth-x), y+sign(w.Peeps[target].AtPos/MapWidth-y)
			next := nx + ny*MapWidth
			if inMap(next) && w.MapBlk[next] == WaterBlock {
				for _, d := range [...][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
					consider(nx+d[0], ny+d[1], true, 500)
				}
			}
		}
	}
	if bestScore > 0 {
		return w.advancedSculpt(player, bestX, bestY, bestRaise)
	}
	return false
}

func TestAdvancedFrontierOptimizationPreservesCandidateSelection(t *testing.T) {
	committed := 0
	for _, crowded := range []bool{false, true} {
		for _, gameMode := range []byte{0, GameOnlyRaise, GameRaiseTown, GameNoBuild} {
			for player := 0; player < 2; player++ {
				t.Run(fmt.Sprintf("crowded%t/mode%d/player%d", crowded, gameMode, player), func(t *testing.T) {
					base := advancedTestWorld()
					base.Level.GameMode = gameMode
					// A water channel gives the same state both offensive lowering
					// candidates and causeway raising candidates.
					for y := 0; y <= MapHeight; y++ {
						base.Alt[32+y*EndWidth] = 0
						base.Alt[33+y*EndWidth] = 0
					}
					base.makeMap(0, 0, MapWidth-1, MapHeight-1)
					count := 16
					if crowded {
						count = MaxPeeps
					}
					for i := 0; i < count; i++ {
						// Adjacent and overlapping units deliberately revisit point
						// candidates, including rejected and equal-scoring ones.
						x, y := 31+(i%2)*3, 20+(i/2)%4
						pos := x + y*MapWidth
						p := Peep{Player: byte(i % 2), Flags: InTown, Population: 200 + i, AtPos: pos}
						if int(p.Player) == player {
							p.Flags = OnMove
						}
						if i%3 == 0 {
							p.Flags, p.HeadFor, p.Status = OnMove, 1, KnightStatus
						}
						base.Peeps = append(base.Peeps, p)
						if base.MapWho[pos] == 0 {
							base.MapWho[pos] = byte(i + 1)
						}
					}
					base.Magnets[0].Mana, base.Magnets[1].Mana = 10000, 10000
					base.Magnets[0].Flags, base.Magnets[1].Flags = MagnetMode, MagnetMode
					base.Magnets[0].Carried, base.Magnets[1].Carried = 1, 2
					for attempt := 0; attempt < 12; attempt++ {
						optimized, reference := *base, *base
						before := base.StateHash()
						got, want := optimized.advancedFrontierLand(player), referenceFrontierLand(&reference, player)
						if got != want || optimized.StateHash() != reference.StateHash() {
							t.Fatalf("candidate selection differs on action %d: changed=%t/%t", attempt, got, want)
						}
						if base.StateHash() != before {
							t.Fatal("candidate evaluation changed shared state")
						}
						if !got {
							break
						}
						committed++
						*base = optimized
					}
				})
			}
		}
	}
	if committed < 12 {
		t.Fatalf("fixture exercised only %d successful actions", committed)
	}
}
