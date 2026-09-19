package populous

// Forecast only the deterministic terrain operation. Calling Flood on a copy
// would also append shared audio and update score; no spell, RNG or entity
// simulation is needed to see which occupied tiles will be underwater.
func (w *World) advancedFloodExposure() [2]int {
	trial := *w
	for i, alt := range trial.Alt {
		if alt > 0 {
			trial.Alt[i]--
		}
	}
	trial.makeMap(0, 0, MapWidth-1, MapHeight-1)
	exposed := [2]int{}
	for _, p := range w.Peeps {
		if p.Population > 0 && p.Flags&InRuin == 0 && p.Player <= DevilPlayer && inMap(p.AtPos) && trial.MapBlk[p.AtPos] == WaterBlock {
			exposed[p.Player] += p.Population
		}
	}
	return exposed
}

// advancedSculpt is the strategic computer's ordinary terrain command. The
// engine primitive checks mana and level restrictions, but the player's view
// also requires a living construction presence. Apply that same requirement
// to every candidate, committed action and resumed multi-action project.
func (w *World) advancedSculpt(player, x, y int, raise bool) bool {
	if player < 0 || player >= len(w.Computer) || w.Computer[player].Mode&computerLand == 0 || !w.HasBuildPresenceAt(player, x-3, y-3, 8, 8, x, y) {
		return false
	}
	return w.changeAltitude(player, x, y, raise, false)
}

func (w *World) advancedFoundingTile(player, pos int) bool {
	if !inMap(pos) || w.Magnets[player].Mana < 20 || w.Magnets[player].NoTowns > 50 {
		return false
	}
	x, y := pos%MapWidth, pos/MapWidth
	total := w.Alt[x+y*EndWidth] + w.Alt[x+1+y*EndWidth] + w.Alt[x+(y+1)*EndWidth] + w.Alt[x+1+(y+1)*EndWidth]
	if total == 1 {
		return false
	}
	for xx := x; xx <= x+1; xx++ {
		for yy := y; yy <= y+1; yy++ {
			alt := w.Alt[xx+yy*EndWidth]
			raise := total%4 == 3 && alt == total/4
			lower := total%4 == 1 && alt > total/4
			if !raise && !lower {
				continue
			}
			trial := *w
			if trial.advancedSculpt(player, xx, yy, raise) && trial.Magnets[player].Mana >= 0 {
				return w.advancedSculpt(player, xx, yy, raise)
			}
		}
	}
	return false
}

// advancedFrontierLand accompanies real troops, using one paid terrain edit.
// A nearby enemy can be deprived of food or drowned; an expedition blocked by
// water can build a causeway. Neither operation grants remote construction or
// walks a unit through terrain: ordinary movement and combat still decide it.
func (w *World) advancedFrontierLand(player int) bool {
	if w.Computer[player].Mode&computerLand == 0 || w.Level.GameMode&GameNoBuild != 0 || w.Magnets[player].Mana < 100 || w.War {
		return false
	}
	// The original world's food values stay constant during candidate scoring.
	// Index by tile rather than peep so overlapping groups share the lookup.
	var enemyFood [MapWidth * MapHeight]int
	for _, p := range w.Peeps {
		if p.Population > 0 && int(p.Player) != player && p.Flags == InTown && inMap(p.AtPos) {
			enemyFood[p.AtPos] = w.checkLife(player^1, p.AtPos)
		}
	}
	// A point reached from several neighbouring people has the same score.
	// Lowering always has base zero and raising always has base 500 below.
	// Keep the first visit, preserving the existing tie-breaking order.
	var examined [EndWidth * EndWidth]uint8
	bestScore, bestX, bestY, bestRaise := 0, 0, 0, false
	consider := func(x, y int, raise bool, base int) {
		if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
			return
		}
		point, mask := x+y*EndWidth, uint8(1)
		if raise {
			mask = 2
		}
		if examined[point]&mask != 0 {
			return
		}
		examined[point] |= mask
		if !w.HasBuildPresenceAt(player, x-3, y-3, 8, 8, x, y) {
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
					score += max(0, enemyFood[p.AtPos]-trial.checkLife(player^1, p.AtPos))
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
