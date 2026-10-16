package populous

// The old computer has a privileged early-emigration rule. The strategic
// player instead lowers one peripheral vertex, waits for a normal town update,
// then restores it with another paid command. Negative Arrived values encode
// the restoration deadline and altitude; existing snapshot fields preserve the
// project without introducing state outside the simulation.
func (w *World) advancedReleaseColonists(player int) bool {
	if w.Level.GameMode&(GameNoBuild|GameOnlyRaise) != 0 || w.War || w.Computer[player].Mode&computerLand == 0 || w.Computer[player].Arrived >= advancedRepairPlan || w.Magnets[player].Mana < 100 {
		return false
	}
	settlements, walkers, living := 0, 0, 0
	for _, p := range w.Peeps {
		if p.Population <= 0 || p.Flags&InRuin != 0 {
			continue
		}
		living++
		if int(p.Player) != player {
			continue
		}
		if p.Flags == InTown {
			settlements++
		} else if !isHeadedPeep(p) {
			walkers++
		}
	}
	if living >= MaxPeeps-4 || walkers >= max(1, settlements/3) {
		return false
	}
	minimum := MaxFood + w.walkDeath()*12
	for index, p := range w.Peeps {
		if p.Population < minimum || p.Population > 2000 || p.Flags != InTown || int(p.Player) != player || !inMap(p.AtPos) || w.checkLife(player, p.AtPos) < CityFood {
			continue
		}
		// Release settlers only where there is nearby useful land to colonize.
		x, y := p.AtPos%MapWidth, p.AtPos/MapWidth
		room := false
		for yy := max(0, y-5); yy <= min(MapHeight-1, y+5) && !room; yy++ {
			for xx := max(0, x-5); xx <= min(MapWidth-1, x+5); xx++ {
				pos := xx + yy*MapWidth
				if w.MapBlk[pos] == FlatBlock && w.MapWho[pos] == 0 && !w.nearExistingTown(pos) && w.checkLife(player, pos) > 0 {
					room = true
					break
				}
			}
		}
		if !room {
			continue
		}
		bestPoint, bestCost := -1, 1<<30
		var seen [EndWidth * EndWidth]bool
		for _, offset := range offsetVector[9:17] {
			if w.validMove(p.AtPos, offset) == 1 {
				continue
			}
			pos := p.AtPos + offset
			for _, corner := range [...]int{0, 1, EndWidth, EndWidth + 1} {
				point := pos%MapWidth + pos/MapWidth*EndWidth + corner
				if seen[point] {
					continue
				}
				seen[point] = true
				trial := *w
				if !trial.advancedSculpt(player, point%EndWidth, point/EndWidth, false) {
					continue
				}
				life := trial.checkLife(player, p.AtPos)
				if life < 200 || life >= CityFood || !w.advancedReleaseSafe(&trial, player, index) {
					continue
				}
				if !trial.advancedSculpt(player, point%EndWidth, point/EndWidth, true) || trial.Magnets[player].Mana < 0 || trial.checkLife(player, p.AtPos) < CityFood || w.advancedDamagesTown(&trial, player) {
					continue
				}
				cost := w.Magnets[player].Mana - trial.Magnets[player].Mana
				if cost < bestCost && cost <= 56 {
					bestPoint, bestCost = point, cost
				}
			}
		}
		if bestPoint < 0 {
			continue
		}
		alt := w.Alt[bestPoint]
		if !w.advancedSculpt(player, bestPoint%EndWidth, bestPoint/EndWidth, false) {
			continue
		}
		deadline := ((w.GameTurn + 7) / 8) * 8
		w.Computer[player].Arrived = -((deadline << 4) | alt)
		w.Computer[player].LastBattle = bestPoint
		return true
	}
	return false
}

func (w *World) advancedReleaseSafe(trial *World, player, source int) bool {
	for i, p := range w.Peeps {
		if p.Population <= 0 || int(p.Player) != player || p.Flags&InRuin != 0 || !inMap(p.AtPos) {
			continue
		}
		if trial.MapBlk[p.AtPos] == WaterBlock && w.MapBlk[p.AtPos] != WaterBlock {
			return false
		}
		if i != source && p.Flags&InTown != 0 && trial.checkLife(player, p.AtPos) < w.checkLife(player, p.AtPos) {
			return false
		}
	}
	return true
}

func (w *World) advancedRestoreColonistLand(player int) bool {
	stats := &w.Computer[player]
	if stats.Arrived >= 0 {
		return false
	}
	encoded := -stats.Arrived
	deadline, alt := encoded>>4, encoded&15
	if w.GameTurn <= deadline {
		return false
	}
	point := stats.LastBattle
	if point < 0 || point >= len(w.Alt) || alt < 1 || alt > 8 || w.Alt[point] >= alt {
		stats.Arrived = 0
		return false
	}
	trial := *w
	if !trial.advancedSculpt(player, point%EndWidth, point/EndWidth, true) || trial.Magnets[player].Mana < 0 {
		stats.Arrived = 0
		return false
	}
	changed := w.advancedSculpt(player, point%EndWidth, point/EndWidth, true)
	if w.Alt[point] >= alt {
		stats.Arrived = 0
	}
	return changed
}
