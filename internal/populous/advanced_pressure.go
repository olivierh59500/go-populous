package populous

// A temporary loss of peripheral farmland can be cheaper than letting a
// hostile knight raze the town. With a threefold population advantage the same
// trade can break a remaining enemy position. Evaluate the full terrain cost
// and preserve all occupied friendly tiles and town centres before committing
// one legal edit. This does not grant remote construction near an enemy.
// Restrict the trade to high attrition: the full campaign A/B retained better
// low-attrition results with the existing strict farmland protection.
func (w *World) advancedPressureLand(player int) bool {
	if w.walkDeath() < 8 || w.War || w.Level.GameMode&(GameOnlyRaise|GameNoBuild) != 0 || w.Computer[player].Mode&computerLand == 0 || w.Magnets[player].Mana < 50 {
		return false
	}
	bestScore, bestPoint := 0, -1
	pop := w.PlayerPopulations()
	cleanup := pop[player] > 10000 && pop[player] > pop[player^1]*3
	var seen [EndWidth * EndWidth]bool
	for _, enemy := range w.Peeps {
		if enemy.Population <= 0 || int(enemy.Player) == player || enemy.Flags&(InRuin|InWater|InBattle) != 0 || (!cleanup && (enemy.Population < 300 || !isHeadedPeep(enemy))) || !inMap(enemy.AtPos) {
			continue
		}
		base := enemy.AtPos%MapWidth + enemy.AtPos/MapWidth*EndWidth
		for _, corner := range [...]int{0, 1, EndWidth, EndWidth + 1} {
			point := base + corner
			if seen[point] || w.Alt[point] == 0 {
				continue
			}
			seen[point] = true
			trial := *w
			if !trial.advancedSculpt(player, point%EndWidth, point/EndWidth, false) || trial.Magnets[player].Mana < 0 {
				continue
			}
			cost := w.Magnets[player].Mana - trial.Magnets[player].Mana
			if cost > advancedMaxRoutineLandCost {
				continue
			}
			score := enemy.Population/4 - cost*2
			if trial.MapBlk[enemy.AtPos] == WaterBlock {
				score += enemy.Population*2 + 1000
			}
			if cleanup && enemy.Flags == InTown {
				score += max(0, min(MaxFood, w.checkLife(player^1, enemy.AtPos))-min(MaxFood, trial.checkLife(player^1, enemy.AtPos))) * 10
			}
			unsafe := false
			for _, p := range w.Peeps {
				if p.Population <= 0 || int(p.Player) != player || p.Flags&InRuin != 0 || !inMap(p.AtPos) {
					continue
				}
				if trial.MapBlk[p.AtPos] == WaterBlock && w.MapBlk[p.AtPos] != WaterBlock {
					unsafe = true
					break
				}
				if p.Flags&InTown != 0 {
					old, now := w.checkLife(player, p.AtPos), trial.checkLife(player, p.AtPos)
					if now <= 0 && old > 0 {
						unsafe = true
						break
					}
					if now < old {
						score -= 100 + (min(old, MaxFood)-min(now, MaxFood))*4
					}
				}
			}
			if !unsafe && score > bestScore {
				bestPoint, bestScore = point, score
			}
		}
	}
	return bestPoint >= 0 && w.advancedSculpt(player, bestPoint%EndWidth, bestPoint/EndWidth, false)
}
