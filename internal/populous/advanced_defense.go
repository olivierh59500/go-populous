package populous

// advancedKnightDefense uses one ordinary Swamp power against the most urgent
// enemy knight that is already heading for a living friendly target. Planning
// is deterministic; only the committed SwampAt call consumes the world's RNG.
func (w *World) advancedKnightDefense(player int) bool {
	x, y, ok := w.advancedKnightDefenseTarget(player)
	if !ok {
		return false
	}
	return w.SwampAt(player, x, y)
}

func (w *World) advancedKnightDefenseTarget(player int) (int, int, bool) {
	defenseEconomy := w.Rules.WalkDeath == 4 || w.advancedIntermediateTownStagesFlat()
	if !defenseEconomy || player < 0 || player >= len(w.Computer) || w.War || w.Computer[player].Mode&computerSwamp == 0 || w.Magnets[player].Mana < ManaSwampCost {
		return 0, 0, false
	}
	bestScore, bestKnight, bestX, bestY := 0, len(w.Peeps), 0, 0
	for index, knight := range w.Peeps {
		if knight.Population <= 0 || int(knight.Player) == player || knight.Player > DevilPlayer || knight.Flags&OnMove == 0 || knight.Flags&(InBattle|InWater|InRuin) != 0 || knight.HeadFor == 0 || !inMap(knight.AtPos) {
			continue
		}
		targetIndex := knight.HeadFor - 1
		if targetIndex < 0 || targetIndex >= len(w.Peeps) {
			continue
		}
		target := w.Peeps[targetIndex]
		if target.Population <= 0 || int(target.Player) != player || target.Flags&InRuin != 0 || !inMap(target.AtPos) {
			continue
		}
		next, ok := w.advancedKnightNextStep(knight, target.AtPos)
		if !ok {
			continue
		}
		x, y := next%MapWidth, next/MapWidth
		if !w.advancedDefensiveSwampSafe(player, x, y) {
			continue
		}
		distance := abs(knight.AtPos%MapWidth-target.AtPos%MapWidth) + abs(knight.AtPos/MapWidth-target.AtPos/MapWidth)
		score := w.advancedKnightThreatScore(player, knight, targetIndex, distance)
		// Relate the minimum useful threat to the spell's normal price. A weak,
		// distant remnant should not consume the same mana as a real town raid.
		if score < ManaSwampCost/5 {
			continue
		}
		point := x + y*MapWidth
		bestPoint := bestX + bestY*MapWidth
		if score > bestScore || score == bestScore && (index < bestKnight || index == bestKnight && point < bestPoint) {
			bestScore, bestKnight, bestX, bestY = score, index, x, y
		}
	}
	return bestX, bestY, bestScore > 0
}

// advancedKnightNextStep mirrors the non-random direction order used by
// moveToward without mutating MagnetLastMove or queuing a terrain order.
func (w *World) advancedKnightNextStep(knight Peep, target int) (int, bool) {
	if !inMap(knight.AtPos) || !inMap(target) || knight.AtPos == target {
		return 0, false
	}
	dx := sign(target%MapWidth - knight.AtPos%MapWidth)
	dy := sign(target/MapWidth - knight.AtPos/MapWidth)
	direction := toDelta[(dx+1)*3+dy+1]
	if next, ok := w.advancedKnightStepInDirection(knight, direction); ok {
		return next, true
	}
	for candidate, tries := direction-1, 0; tries < len(toOffset); candidate, tries = candidate+1, tries+1 {
		if candidate < 0 {
			candidate = len(toOffset) - 1
		}
		if candidate >= len(toOffset) {
			candidate = 0
		}
		if toOffset[candidate] == knight.MagnetLastMove {
			continue
		}
		if next, ok := w.advancedKnightStepInDirection(knight, candidate); ok {
			return next, true
		}
	}
	return 0, false
}

func (w *World) advancedKnightStepInDirection(knight Peep, direction int) (int, bool) {
	if direction < 0 || direction >= len(toOffset) {
		return 0, false
	}
	delta := toOffset[direction]
	next := knight.AtPos + delta
	if !inMap(next) || w.validMove(knight.AtPos, delta) != 0 || int(w.MapBlk[next]) == SwampBlock {
		return 0, false
	}
	return next, true
}

func (w *World) advancedKnightThreatScore(player int, knight Peep, targetIndex, distance int) int {
	target := w.Peeps[targetIndex]
	knightValue := knight.Population * max(1, knight.Weapons)
	targetValue := target.Population * max(1, target.Weapons)
	if target.Flags&InTown != 0 {
		life := w.checkLife(player, target.AtPos)
		stage := 0
		if life >= CityFood {
			stage = LastTown - FirstTown
		} else if life > 0 {
			stage = clamp((life*10)/MaxFood, 0, LastTown-FirstTown-1)
		}
		targetValue += life + w.Rules.ManaAdd[stage]*100 + w.Rules.PopulationAdd[stage]*50
	}
	if w.Magnets[player].Carried == targetIndex+1 {
		targetValue += target.Population + ManaKnightCost/2
	}
	return (knightValue + targetValue) / max(1, distance)
}

// SwampAt can affect any eligible unoccupied tile in this clipped 7x7 square.
// Reject obvious friendly collateral and any existing swamp in that whole
// footprint, so repeated action slots cannot carpet the same defensive zone.
func (w *World) advancedDefensiveSwampSafe(player, x, y int) bool {
	placeable := 0
	x1, y1 := max(0, x-3), max(0, y-3)
	x2, y2 := min(MapWidth-1, x+3), min(MapHeight-1, y+3)
	for yy := y1; yy <= y2; yy++ {
		for xx := x1; xx <= x2; xx++ {
			pos := xx + yy*MapWidth
			if int(w.MapBlk[pos]) == SwampBlock || int(w.MapBlk[pos]) == FarmBlock+player {
				return false
			}
			if occupant := int(w.MapWho[pos]) - 1; occupant >= 0 && occupant < len(w.Peeps) {
				peep := w.Peeps[occupant]
				if peep.Population > 0 && int(peep.Player) == player && peep.Flags&InRuin == 0 {
					return false
				}
			}
			block := int(w.MapBlk[pos])
			if w.MapWho[pos] == 0 && (block == FlatBlock || block == FarmBlock+GodPlayer || block == FarmBlock+DevilPlayer || block == BadLand) {
				placeable++
			}
		}
	}
	for _, peep := range w.Peeps {
		if peep.Population <= 0 || int(peep.Player) != player || peep.Flags&InTown == 0 || peep.Flags&InRuin != 0 || !inMap(peep.AtPos) {
			continue
		}
		for _, offset := range offsetVector[:17] {
			if w.validMove(peep.AtPos, offset) == 1 {
				continue
			}
			pos := peep.AtPos + offset
			px, py := pos%MapWidth, pos/MapWidth
			if px >= x1 && px <= x2 && py >= y1 && py <= y2 {
				return false
			}
		}
	}
	if inMap(w.Magnets[player].GoTo) {
		mx, my := w.Magnets[player].GoTo%MapWidth, w.Magnets[player].GoTo/MapWidth
		if mx >= x1 && mx <= x2 && my >= y1 && my <= y2 {
			return false
		}
	}
	return placeable >= 4
}
