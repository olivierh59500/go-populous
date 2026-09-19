package populous

// Recruit a force by joining the carrier to a populated nearby castle. The
// magnet does the walking and the ordinary contact rules perform the merger.
// This avoids waiting in place, which costs eight lives per waiting tick on
// snow/desert instead of eight per walking animation.
func (w *World) advancedRallyTown(player int) (handled, acted bool) {
	stats := &w.Computer[player]
	if !w.advancedIntermediateTownStagesFlat() || w.War || stats.Mode&computerKnight == 0 && stats.Mode&computerWar != 0 {
		return false, false
	}
	knightPlan := stats.Mode&computerKnight != 0
	if stats.QuakeCount < 0 && stats.NoSwamps != advancedRallyPlan {
		return false, false
	}
	carrier := w.carriedPeepIndex(player)
	if stats.QuakeCount < 0 {
		deadline := -stats.QuakeCount
		target := stats.NoQuakes - 1
		if w.GameTurn > deadline || !w.validPeep(target) || int(w.Peeps[target].Player) != player || w.advancedActiveKnightCount(player) >= 3 {
			stats.QuakeCount = w.GameTurn + 30*8
			return false, false
		}
		if carrier < 0 {
			return true, false
		}
		p := w.Peeps[carrier]
		if p.Flags&InBattle != 0 {
			return true, false
		}
		if !knightPlan && p.Population >= 1500 {
			stats.QuakeCount = 0
			return false, false
		}
		if knightPlan && p.Population > DevilMakesKnight && w.Magnets[player].Mana >= ManaKnightCost {
			if w.Knight(player) {
				stats.QuakeCount = w.GameTurn + 30*8
				return true, true
			}
		}
		pos := w.Peeps[target].AtPos
		if carrier == target {
			stats.QuakeCount = 0
			return false, false
		}
		if w.Magnets[player].GoTo != pos || w.Magnets[player].Flags != MagnetMode || p.Flags == InTown {
			return true, w.SetMagnetTo(player, pos)
		}
		return true, false
	}
	reserve, required := 1000, 1800
	if knightPlan {
		reserve, required = ManaKnightCost+1000, 3500
	}
	if !knightPlan && carrier >= 0 && w.Peeps[carrier].Population >= 1500 {
		return false, false
	}
	if w.GameTurn < stats.QuakeCount || w.Magnets[player].Mana < reserve || w.advancedActiveKnightCount(player) >= 3 {
		return false, false
	}
	bestSource, bestTarget, bestDistance := -1, -1, 1<<30
	pop := w.PlayerPopulations()
	chain := stats.NoTowns+stats.NoCastles >= 8 && pop[player]*4 > pop[player^1]*5
	for i, p := range w.Peeps {
		if p.Population < 100 || int(p.Player) != player || p.Flags&(InBattle|InWater|InRuin) != 0 || isHeadedPeep(p) || !inMap(p.AtPos) {
			continue
		}
		if carrier >= 0 && carrier != i {
			continue
		}
		if carrier < 0 && p.Flags != OnMove {
			continue
		}
		for j, q := range w.Peeps {
			if j == i || q.Population < 1000 || int(q.Player) != player || q.Flags != InTown || !inMap(q.AtPos) {
				continue
			}
			d := max(abs(p.AtPos%MapWidth-q.AtPos%MapWidth), abs(p.AtPos/MapWidth-q.AtPos/MapWidth))
			if d > 12 || d >= bestDistance || (!chain && p.Population-d*w.walkDeath()*2+q.Population < required) || p.Population <= d*w.walkDeath()*2+50 || !w.advancedStraightMarch(p.AtPos, q.AtPos) {
				continue
			}
			bestSource, bestTarget, bestDistance = i, j, d
		}
	}
	if bestSource < 0 {
		return false, false
	}
	pos := w.Peeps[bestTarget].AtPos
	if carrier < 0 {
		pos = w.Peeps[bestSource].AtPos
	}
	if !w.SetMagnetTo(player, pos) {
		return false, false
	}
	stats.QuakeCount = -(w.GameTurn + 60*8)
	stats.NoQuakes = bestTarget + 1
	stats.NoSwamps = advancedRallyPlan
	return true, true
}
