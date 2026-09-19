package populous

// The legacy computer's rally scratch fields are also included in snapshots.
// A tagged Arrived value stores an unfinished terrace's target altitude; its
// centre is LastBattle. No hidden state or extra simulation RNG is needed.
const (
	advancedPlateauPlan = 1000
	advancedRepairPlan  = 2000
)

// TickWithAdvancedComputer runs a spectator match between the original computer
// and a deterministic strategic computer. Both use the level's normal resources,
// powers and action cadence. The strategy is chosen by the caller, never stored
// in world snapshots or multiplayer state. An invalid side uses two original AIs.
func (w *World) TickWithAdvancedComputer(advancedPlayer int) {
	w.tickWithComputerStrategy([2]bool{true, true}, advancedPlayer)
}

func (w *World) runAdvancedComputerPlayer(player int) {
	if !w.computerActionReady(player) {
		return
	}
	if w.advancedFrontierLand(player) || w.advancedPower(player) || w.advancedPolicy(player) || w.advancedLand(player) {
		w.markComputerAction(player)
	}
}

// Growing independent settlements is more productive than randomly switching
// to joining/fighting while the economy is still small. Knights can attack
// independently; settlers keep supplying them with population and mana.
func (w *World) advancedPolicy(player int) bool {
	stats := w.Computer[player]
	// Worlds that prohibit sculpting cannot build the castle economy used by
	// the expedition policy below. Once several ordinary towns sustain the
	// population, let emigrants engage nearby enemies through the normal
	// combat mode. Towns keep producing; no mass evacuation is required.
	if w.Level.GameMode&GameNoBuild != 0 {
		mode := SettleMode
		if stats.NoTowns >= 4 {
			mode = FightMode
		}
		if w.Magnets[player].Flags != mode {
			return w.SetMagnetMode(player, mode)
		}
		return false
	}
	population := w.PlayerPopulations()
	carried := w.carriedPeepIndex(player)
	mode := SettleMode
	// A mature economy supplies expeditions while the towns keep producing.
	// Rally around the leader first; a knight then leaves the settlers free to
	// resume expansion. The ordinary magnet command wakes a settled leader.
	if stats.NoCastles >= 3 && carried >= 0 && population[player] > 2500 {
		if stats.Mode&computerKnight != 0 && w.Magnets[player].Mana >= ManaKnightCost+200 {
			if w.Peeps[carried].Population >= 1000 {
				return w.Knight(player)
			}
			pos := w.Peeps[carried].AtPos
			if w.Magnets[player].Flags != MagnetMode || w.Magnets[player].GoTo != pos {
				return w.SetMagnetTo(player, pos)
			}
			return false
		}
		// Even when a level disables knights, an accumulated leader can lead
		// an expedition. New emigrants join it through the ordinary magnet;
		// settled towns remain the economy behind the front.
		if w.Peeps[carried].Population >= 1500 && population[player] > population[player^1]*3/2 {
			if enemy := w.closestEnemy(carried); enemy >= 0 {
				target := w.Peeps[enemy].AtPos
				magnet := w.Magnets[player]
				if magnet.Flags != MagnetMode || magnet.GoTo != target {
					return w.SetMagnetTo(player, target)
				}
				return false
			}
		}
	}
	if w.Magnets[player].Flags != mode {
		return w.SetMagnetMode(player, mode)
	}
	return false
}

func (w *World) advancedPower(player int) bool {
	mode := w.Computer[player].Mode
	mana := w.Magnets[player].Mana
	populations := w.PlayerPopulations()
	if mode&computerWar != 0 && mana >= ManaWarCost+1000 && populations[player]*4 > populations[player^1]*5 {
		return w.WarPower(player)
	}
	// On passive opening levels, productive land is a better investment than
	// immediately spending the first castle's income on one earthquake.
	if w.Computer[player^1].Mode&(computerQuake|computerSwamp|computerKnight|computerVolcano|computerFlood|computerWar) == 0 && w.Computer[player].NoCastles < 4 && w.GameTurn < 4*60*8 && mana < 12000 {
		return false
	}
	// Flood only when the opponent has substantially more people exposed to
	// drowning after the actual deterministic sea-level change. Low slopes
	// are not necessarily submerged; altitude alone overestimates the damage.
	if mode&computerFlood != 0 && mana >= ManaFloodCost+1000 {
		vulnerable := w.advancedFloodExposure()
		if vulnerable[player^1] > 100 && vulnerable[player^1] > vulnerable[player]*3 && vulnerable[player]*4 < populations[player] {
			return w.Flood(player)
		}
	}
	// Do not spend every new handful of mana on harassment forever. Once the
	// economy can win an Armageddon, bank the decisive spell's real cost.
	if mode&computerWar != 0 && populations[player]*4 > populations[player^1]*5 && populations[player] > 10000 && w.Computer[player].NoCastles >= 2 {
		return false
	}
	carried := w.carriedPeepIndex(player)
	if mode&computerKnight != 0 && mana >= ManaKnightCost+500 && carried >= 0 && w.Peeps[carried].Population >= 1500 && w.Computer[player].NoTowns+w.Computer[player].NoCastles >= 3 {
		return w.Knight(player)
	}
	if mana < ManaQuakeCost+500 {
		return false
	}
	// Aim at valuable clusters, with a heavy penalty for friendly settlements.
	// No random lookahead is used: examining a candidate must not consume the
	// simulation's RNG or affect the original computer's future decisions.
	target, value := -1, 0
	for i, p := range w.Peeps {
		if p.Population <= 0 || int(p.Player) == player || p.Flags&InRuin != 0 || !inMap(p.AtPos) {
			continue
		}
		x := clamp(p.AtPos%MapWidth-4, 0, MapWidth-9)
		y := clamp(p.AtPos/MapWidth-4, 0, MapHeight-9)
		score := 0
		for _, other := range w.Peeps {
			if other.Population <= 0 || other.Flags&InRuin != 0 || !inMap(other.AtPos) || other.AtPos%MapWidth < x || other.AtPos%MapWidth > x+8 || other.AtPos/MapWidth < y || other.AtPos/MapWidth > y+8 {
				continue
			}
			worth := other.Population
			if other.Flags == InTown {
				worth += w.checkLife(int(other.Player), other.AtPos) * 2
			}
			if int(other.Player) == player {
				score -= worth * 3
			} else {
				score += worth
			}
		}
		if score > value {
			target, value = i, score
		}
	}
	if target < 0 {
		return false
	}
	p := w.Peeps[target]
	x, y := clamp(p.AtPos%MapWidth-4, 0, MapWidth-9), clamp(p.AtPos/MapWidth-4, 0, MapHeight-9)
	if mode&computerVolcano != 0 && mana >= ManaVolcanoCost+500 && value >= 800 {
		return w.VolcanoAt(player, x, y)
	}
	if mode&computerQuake != 0 && value > 0 && (p.Flags == InTown || w.MapAlt[p.AtPos] < 2) {
		return w.QuakeAt(player, x, y)
	}
	if mode&computerSwamp != 0 && mana >= ManaSwampCost+500 && value > 0 && p.Flags != InTown {
		return w.SwampAt(player, p.AtPos%MapWidth, p.AtPos/MapWidth)
	}
	return false
}

// advancedLand spends a single legal terrain action where it most improves a
// town's food supply. Working on the actual seventeen food tiles (rather than
// the outer edge of a 9x9 square) brings castles online sooner. A small local
// lookahead rejects edits that would destroy an existing friendly settlement.
func (w *World) advancedLand(player int) bool {
	if w.Computer[player].Mode&computerLand == 0 || w.Level.GameMode&GameNoBuild != 0 || w.Magnets[player].Mana < ManaPointCost {
		return false
	}
	if w.Computer[player].Arrived >= advancedRepairPlan {
		return w.advancedFinishRepair(player)
	}
	if w.Computer[player].Arrived < advancedPlateauPlan && w.advancedRepair(player) {
		return true
	}
	if w.advancedRaisedPlateau(player) {
		return true
	}
	towns := [4]int{-1, -1, -1, -1}
	priorities := [4]int{}
	for i, p := range w.Peeps {
		if p.Population <= 0 || int(p.Player) != player || p.Flags != InTown || !inMap(p.AtPos) {
			continue
		}
		life := w.checkLife(player, p.AtPos)
		if life <= 0 {
			continue
		}
		priority := life
		if life >= CityFood {
			priority = 80 // Keep widening completed plateaus for new towns.
		}
		if w.Alt[p.AtPos%MapWidth+p.AtPos/MapWidth*EndWidth] >= 2 {
			priority += 40
		}
		for rank := range towns {
			if towns[rank] < 0 || priority > priorities[rank] {
				for shift := len(towns) - 1; shift > rank; shift-- {
					towns[shift], priorities[shift] = towns[shift-1], priorities[shift-1]
				}
				towns[rank], priorities[rank] = i, priority
				break
			}
		}
	}
	bestScore, bestX, bestY, bestRaise := 0, 0, 0, false
	for _, index := range towns {
		if index < 0 {
			continue
		}
		p := w.Peeps[index]
		x, y := p.AtPos%MapWidth, p.AtPos/MapWidth
		alt := w.Alt[x+y*EndWidth]
		if alt <= 0 {
			continue
		}
		oldLife := w.checkLife(player, p.AtPos)
		var examined [EndWidth * EndWidth]bool
		for _, offset := range offsetVector[:17] {
			if w.validMove(p.AtPos, offset) == 1 {
				continue
			}
			pos := p.AtPos + offset
			for _, corner := range [...]int{0, 1, EndWidth, EndWidth + 1} {
				point := pos%MapWidth + pos/MapWidth*EndWidth + corner
				if examined[point] || w.Alt[point] == alt {
					continue
				}
				examined[point] = true
				xx, yy := point%EndWidth, point/EndWidth
				raise := w.Alt[point] < alt
				if !raise && w.Level.GameMode&GameOnlyRaise != 0 {
					continue
				}
				trial := *w // Altitude edits touch value fields only, not Peeps.
				if !trial.advancedSculpt(player, xx, yy, raise) || trial.Magnets[player].Mana < 0 {
					continue
				}
				life := trial.checkLife(player, p.AtPos)
				if life < oldLife || w.advancedDamagesTown(&trial, player) {
					continue
				}
				// Even an edit that does not finish a flat tile progresses
				// toward the town's plateau. Completing a castle dominates.
				score := 1 + oldLife/10 + (life-oldLife)*20
				score -= abs(xx-x) + abs(yy-y) + (w.Magnets[player].Mana-trial.Magnets[player].Mana)/4
				if score > bestScore {
					bestScore, bestX, bestY, bestRaise = score, xx, yy, raise
				}
			}
		}
		// The food footprint alone produces isolated castles. Extend the same
		// level surface outwards so their emigrants have room for whole new
		// settlements. When flood is available, favour a raised extension: a
		// plateau at altitude two survives one sea-level rise.
		plateauAlt := alt
		if w.Computer[player^1].Mode&computerFlood != 0 && plateauAlt < 2 {
			plateauAlt = 2
		}
		for dy := -5; dy <= 6; dy++ {
			for dx := -5; dx <= 6; dx++ {
				xx, yy := x+dx, y+dy
				if xx < 0 || xx > MapWidth || yy < 0 || yy > MapHeight || abs(dx)+abs(dy) < 3 {
					continue
				}
				point := xx + yy*EndWidth
				if w.Alt[point] == plateauAlt || examined[point] {
					continue
				}
				raise := w.Alt[point] < plateauAlt
				if !raise && w.Level.GameMode&GameOnlyRaise != 0 {
					continue
				}
				trial := *w
				if !trial.advancedSculpt(player, xx, yy, raise) || trial.Magnets[player].Mana < 0 || w.advancedDamagesTown(&trial, player) {
					continue
				}
				score := 16 - abs(dx) - abs(dy) - (w.Magnets[player].Mana-trial.Magnets[player].Mana)/4
				for cy := max(0, yy-1); cy <= min(MapHeight-1, yy); cy++ {
					for cx := max(0, xx-1); cx <= min(MapWidth-1, xx); cx++ {
						pos := cx + cy*MapWidth
						if trial.MapBlk[pos] == FlatBlock && w.MapBlk[pos] != FlatBlock && int(w.MapBlk[pos]) != FarmBlock+player {
							score += 15
						}
					}
				}
				if score > bestScore {
					bestScore, bestX, bestY, bestRaise = score, xx, yy, raise
				}
			}
		}
	}
	if bestScore > 0 {
		return w.advancedSculpt(player, bestX, bestY, bestRaise)
	}
	// A walker on a slope needs a flat founding tile before there is a town
	// to improve. Reuse the original's inexpensive one-corner operation.
	for _, p := range w.Peeps {
		if p.Population <= 0 || int(p.Player) != player || p.Flags != OnMove || isHeadedPeep(p) || !inMap(p.AtPos) {
			continue
		}
		// advancedFoundingTile checks each actual corner, including the
		// original sea-level exception to town-only construction.
		if w.advancedFoundingTile(player, p.AtPos) {
			return true
		}
	}
	return false
}

// Renovate one low settlement's whole food footprint before starting another.
// Outer points are raised first, retaining the productive town for as long as
// possible. The four founding corners are completed last; settlers can then
// build a castle on a contiguous plateau above the next flood. This intentionally
// accepts a brief relocation, which a greedy "never reduce food" rule cannot do.
func (w *World) advancedRaisedPlateau(player int) bool {
	stats := &w.Computer[player]
	if stats.Arrived < advancedPlateauPlan {
		if w.Computer[player^1].Mode&computerFlood == 0 || w.Magnets[player].Mana < 500 {
			return false
		}
		best, value, safe := -1, 0, 0
		for i, p := range w.Peeps {
			if p.Population <= 0 || int(p.Player) != player || p.Flags != InTown || !inMap(p.AtPos) {
				continue
			}
			alt := w.Alt[p.AtPos%MapWidth+p.AtPos/MapWidth*EndWidth]
			if alt >= 2 {
				safe++
				continue
			}
			if alt != 1 || p.Population > 1500 {
				continue
			}
			life := w.checkLife(player, p.AtPos)
			if life > value && life < CityFood {
				best, value = i, life
			}
		}
		if best < 0 || safe >= max(2, (stats.NoTowns+stats.NoCastles)/2) {
			return false
		}
		stats.LastBattle = w.Peeps[best].AtPos
		stats.Arrived = advancedPlateauPlan + 2
	}

	pos, target := stats.LastBattle, stats.Arrived-advancedPlateauPlan
	if !inMap(pos) || target < 2 || target > 8 {
		stats.Arrived = 0
		return false
	}
	x, y := pos%MapWidth, pos/MapWidth
	if !w.HasBuildPresence(player, x-4, y-4, 9, 9) {
		stats.Arrived = 0
		return false
	}
	for radius := 3; radius >= 0; radius-- {
		for yy := max(0, y-radius); yy <= min(MapHeight, y+radius+1); yy++ {
			for xx := max(0, x-radius); xx <= min(MapWidth, x+radius+1); xx++ {
				if radius > 0 && xx > x-radius && xx < x+radius+1 && yy > y-radius && yy < y+radius+1 {
					continue
				}
				alt := w.Alt[xx+yy*EndWidth]
				if alt >= target {
					continue
				}
				trial := *w
				if trial.advancedSculpt(player, xx, yy, true) && trial.Magnets[player].Mana >= 0 {
					return w.advancedSculpt(player, xx, yy, true)
				}
			}
		}
	}
	stats.Arrived = 0
	return false
}

// Soft rocks, swamps and scorched ground disappear after an ordinary terrain
// edit. Forecast both edits of a short repair, including their full propagated
// mana cost. Hard RockBlock survives sculpting; sinking an entire high plateau
// to remove it is deliberately not considered.
func (w *World) advancedRepair(player int) bool {
	if w.Level.GameMode&GameOnlyRaise != 0 || w.Magnets[player].Mana < 100 {
		return false
	}
	bestPoint, bestScore, bestRaise := -1, 0, false
	for _, p := range w.Peeps {
		if p.Population <= 0 || int(p.Player) != player || p.Flags != InTown || !inMap(p.AtPos) {
			continue
		}
		oldLife := w.checkLife(player, p.AtPos)
		if oldLife <= 0 || oldLife >= CityFood {
			continue
		}
		for _, offset := range offsetVector[:17] {
			if w.validMove(p.AtPos, offset) == 1 {
				continue
			}
			pos := p.AtPos + offset
			block := int(w.MapBlk[pos])
			if block != RockBlock+1 && block != RockBlock+2 && block != SwampBlock && block != BadLand {
				continue
			}
			for _, corner := range [...]int{0, 1, EndWidth, EndWidth + 1} {
				point := pos%MapWidth + pos/MapWidth*EndWidth + corner
				x, y := point%EndWidth, point/EndWidth
				for _, raise := range [...]bool{false, true} {
					trial := *w
					if !trial.advancedSculpt(player, x, y, raise) || w.advancedDisplacesTown(&trial, player) || !trial.advancedSculpt(player, x, y, !raise) {
						continue
					}
					cost := w.Magnets[player].Mana - trial.Magnets[player].Mana
					gain := trial.checkLife(player, p.AtPos) - oldLife
					if gain <= 0 || cost > 84 || trial.Magnets[player].Mana < 0 || w.advancedDamagesTown(&trial, player) {
						continue
					}
					score := gain*20 - cost
					if score > bestScore {
						bestScore, bestPoint, bestRaise = score, point, raise
					}
				}
			}
		}
	}
	if bestPoint < 0 {
		return false
	}
	w.Computer[player].LastBattle = bestPoint
	w.Computer[player].Arrived = advancedRepairPlan + w.Alt[bestPoint]
	return w.advancedSculpt(player, bestPoint%EndWidth, bestPoint/EndWidth, bestRaise)
}

func (w *World) advancedFinishRepair(player int) bool {
	stats := &w.Computer[player]
	point, target := stats.LastBattle, stats.Arrived-advancedRepairPlan
	if point < 0 || point >= len(w.Alt) || target < 0 || target > 8 || w.Alt[point] == target {
		stats.Arrived = 0
		return false
	}
	raise := w.Alt[point] < target
	if !w.HasBuildPresenceAt(player, point%EndWidth-3, point/EndWidth-3, 8, 8, point%EndWidth, point/EndWidth) {
		stats.Arrived = 0
		return false
	}
	if !raise && w.Level.GameMode&GameOnlyRaise != 0 {
		stats.Arrived = 0
		return false
	}
	trial := *w
	if !trial.advancedSculpt(player, point%EndWidth, point/EndWidth, raise) || trial.Magnets[player].Mana < 0 {
		return false
	}
	changed := w.advancedSculpt(player, point%EndWidth, point/EndWidth, raise)
	if w.Alt[point] == target {
		stats.Arrived = 0
	}
	return changed
}

func (w *World) advancedDamagesTown(trial *World, player int) bool {
	for _, p := range w.Peeps {
		if p.Population <= 0 || int(p.Player) != player || p.Flags != InTown || !inMap(p.AtPos) {
			continue
		}
		if trial.checkLife(player, p.AtPos) < w.checkLife(player, p.AtPos) {
			return true
		}
	}
	return false
}

func (w *World) advancedDisplacesTown(trial *World, player int) bool {
	for _, p := range w.Peeps {
		if p.Population > 0 && int(p.Player) == player && p.Flags == InTown && inMap(p.AtPos) && w.checkLife(player, p.AtPos) > 0 && trial.checkLife(player, p.AtPos) <= 0 {
			return true
		}
	}
	return false
}
