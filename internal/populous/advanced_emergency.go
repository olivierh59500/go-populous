package populous

// advancedEmergencyLand uses one ordinary paid terrain action to rescue a
// strategic-computer follower. It never moves a person directly: the normal
// movement loop observes the rebuilt terrain later in the tick.
//
// Swimmers are considered first. Ordinary settlers are helped only when they
// have no safe adjacent land tile and cannot found a settlement where they
// stand. This deliberately conservative definition avoids spending mana on a
// shortcut when the normal movement code can route around an obstacle.
func (w *World) advancedEmergencyLand(player int) bool {
	if player < 0 || player >= len(w.Computer) || w.War || w.Level.GameMode&GameNoBuild != 0 || w.Computer[player].Mode&computerLand == 0 || w.Magnets[player].Mana < ManaPointCost {
		return false
	}
	if x, y, ok := w.advancedSwimmerRescue(player); ok {
		return w.advancedSculpt(player, x, y, true)
	}
	if x, y, ok := w.advancedBlockedSettlerPassage(player); ok {
		return w.advancedSculpt(player, x, y, true)
	}
	return false
}

type advancedEmergencyCandidate struct {
	x, y    int
	benefit int
	cost    int
	valid   bool
}

func (candidate *advancedEmergencyCandidate) consider(x, y, benefit, cost int) {
	point := x + y*EndWidth
	bestPoint := candidate.x + candidate.y*EndWidth
	if !candidate.valid || benefit > candidate.benefit ||
		(benefit == candidate.benefit && (cost < candidate.cost || cost == candidate.cost && point < bestPoint)) {
		*candidate = advancedEmergencyCandidate{x: x, y: y, benefit: benefit, cost: cost, valid: true}
	}
}

func (w *World) advancedSwimmerRescue(player int) (int, int, bool) {
	var examined [EndWidth * EndWidth]bool
	best := advancedEmergencyCandidate{}
	for _, peep := range w.Peeps {
		if peep.Population <= 0 || int(peep.Player) != player || peep.Flags&InRuin != 0 || !inMap(peep.AtPos) || w.MapBlk[peep.AtPos] != WaterBlock || !w.advancedWaterOccupantCanBeSaved(peep) {
			continue
		}
		x, y := peep.AtPos%MapWidth, peep.AtPos/MapWidth
		for _, corner := range [...]struct{ x, y int }{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}} {
			point := corner.x + corner.y*EndWidth
			if examined[point] {
				continue
			}
			examined[point] = true
			trial := *w
			if !trial.advancedSculpt(player, corner.x, corner.y, true) || trial.Magnets[player].Mana < 0 || !w.advancedEmergencyFriendlySafe(&trial, player) {
				continue
			}
			benefit := 0
			for _, other := range w.Peeps {
				if other.Population > 0 && int(other.Player) == player && other.Flags&InRuin == 0 && inMap(other.AtPos) && w.MapBlk[other.AtPos] == WaterBlock && trial.MapBlk[other.AtPos] != WaterBlock && w.advancedWaterOccupantCanBeSaved(other) {
					benefit += other.Population
				}
			}
			if benefit > 0 {
				best.consider(corner.x, corner.y, benefit, w.Magnets[player].Mana-trial.Magnets[player].Mana)
			}
		}
	}
	return best.x, best.y, best.valid
}

func (w *World) advancedWaterOccupantCanBeSaved(peep Peep) bool {
	// In fatal-water worlds, processTown kills a follower whose InWater flag was
	// already set before inspecting the rebuilt tile. A group flooded this tick
	// has not acquired that flag yet and can still be saved pre-emptively.
	return w.Level.GameMode&GameWaterFatal == 0 || peep.Flags&InWater == 0
}

func (w *World) advancedBlockedSettlerPassage(player int) (int, int, bool) {
	var blocked [MaxPeeps]int
	blockedCount := 0
	for i, peep := range w.Peeps {
		if int(peep.Player) == player && w.advancedSettlerClearlyBlocked(i) {
			if blockedCount < len(blocked) {
				blocked[blockedCount] = i
				blockedCount++
			}
		}
	}
	if blockedCount == 0 {
		return 0, 0, false
	}

	var examined [EndWidth * EndWidth]bool
	best := advancedEmergencyCandidate{}
	for _, index := range blocked[:blockedCount] {
		peep := w.Peeps[index]
		for _, delta := range toOffset {
			target := peep.AtPos + delta
			if !inMap(target) || w.validMove(peep.AtPos, delta) == 1 {
				continue
			}
			block := int(w.MapBlk[target])
			if block != WaterBlock && block != SwampBlock {
				continue
			}
			x, y := target%MapWidth, target/MapWidth
			for _, corner := range [...]struct{ x, y int }{{x, y}, {x + 1, y}, {x, y + 1}, {x + 1, y + 1}} {
				point := corner.x + corner.y*EndWidth
				if examined[point] {
					continue
				}
				examined[point] = true
				trial := *w
				if !trial.advancedSculpt(player, corner.x, corner.y, true) || trial.Magnets[player].Mana < 0 || !w.advancedEmergencyFriendlySafe(&trial, player) {
					continue
				}
				benefit := 0
				for _, blockedIndex := range blocked[:blockedCount] {
					if !trial.advancedSettlerClearlyBlocked(blockedIndex) {
						benefit += w.Peeps[blockedIndex].Population
					}
				}
				if benefit > 0 {
					best.consider(corner.x, corner.y, benefit, w.Magnets[player].Mana-trial.Magnets[player].Mana)
				}
			}
		}
	}
	return best.x, best.y, best.valid
}

func (w *World) advancedSettlerClearlyBlocked(index int) bool {
	if index < 0 || index >= len(w.Peeps) {
		return false
	}
	peep := w.Peeps[index]
	if peep.Population <= 0 || peep.Flags != OnMove || isHeadedPeep(peep) || !inMap(peep.AtPos) {
		return false
	}
	if int(w.MapBlk[peep.AtPos]) == FlatBlock && w.checkLife(int(peep.Player), peep.AtPos) > 0 {
		return false
	}
	hasWaterOrSwamp := false
	for _, delta := range toOffset {
		target := peep.AtPos + delta
		move := w.validMove(peep.AtPos, delta)
		if move == 0 && inMap(target) && int(w.MapBlk[target]) != SwampBlock {
			return false
		}
		if inMap(target) && (move == 3 || int(w.MapBlk[target]) == SwampBlock) {
			hasWaterOrSwamp = true
		}
	}
	return hasWaterOrSwamp
}

func (w *World) advancedEmergencyFriendlySafe(trial *World, player int) bool {
	if trial == nil {
		return false
	}
	for _, peep := range w.Peeps {
		if peep.Population <= 0 || int(peep.Player) != player || peep.Flags&InRuin != 0 || !inMap(peep.AtPos) {
			continue
		}
		// Include towns currently in battle. advancedDamagesTown intentionally
		// considers only the exact InTown state for ordinary economic planning,
		// while an emergency passage must not ruin contested friendly farmland.
		if peep.Flags&InTown != 0 && trial.checkLife(player, peep.AtPos) < w.checkLife(player, peep.AtPos) {
			return false
		}
		if w.MapBlk[peep.AtPos] != WaterBlock && trial.MapBlk[peep.AtPos] == WaterBlock {
			return false
		}
	}
	return true
}
