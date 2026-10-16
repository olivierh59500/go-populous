package populous

// A conventional expedition places ordinary, paid magnet waypoints along safe
// land. It never orders an individual step or changes a follower. QuakeCount
// stores the signed deadline/cooldown and NoQuakes the current target ID.
func (w *World) advancedMarch(player int) (handled, acted bool) {
	stats := &w.Computer[player]
	if w.War || stats.Mode&(computerWar|computerKnight) != 0 {
		return false, false
	}
	if stats.QuakeCount < 0 && stats.NoSwamps != advancedMarchPlan {
		return false, false
	}
	pop := w.PlayerPopulations()
	carrier := w.carriedPeepIndex(player)
	if stats.QuakeCount >= 0 {
		// A low-attrition economy retains its conservative consolidation phase.
		// Waiting as long on harsh terrain wastes short-lived mobile forces.
		if w.walkDeath() < 8 && (w.GameTurn < 5*60*8 || pop[player] < 10000 || pop[player] <= pop[player^1]*2) {
			return false, false
		}
		if w.GameTurn < 3*60*8 || w.GameTurn < stats.QuakeCount || pop[player] < 5000 || pop[player]*4 < pop[player^1]*5 || w.Magnets[player].Mana < 1000 {
			return false, false
		}
		anchor := carrier
		if anchor < 0 || w.Peeps[anchor].Population < 1500 {
			if anchor >= 0 {
				return false, false
			}
			biggest := 1500
			for i, p := range w.Peeps {
				if p.Population > biggest && int(p.Player) == player && p.Flags == OnMove && !isHeadedPeep(p) && inMap(p.AtPos) {
					anchor, biggest = i, p.Population
				}
			}
		}
		if anchor < 0 {
			return false, false
		}
		target, _, _ := w.advancedMarchRoute(anchor)
		if target < 0 {
			return false, false
		}
		pos := w.Peeps[anchor].AtPos
		if !w.SetMagnetTo(player, pos) {
			return false, false
		}
		stats.QuakeCount = -(w.GameTurn + 180*8)
		stats.NoQuakes = target + 1
		stats.NoSwamps = advancedMarchPlan
		return true, true
	}
	if w.GameTurn > -stats.QuakeCount || pop[player] < pop[player^1] || carrier >= 0 && w.Peeps[carrier].Population < 200 {
		stats.QuakeCount = w.GameTurn + 30*8
		if w.Magnets[player].Flags != SettleMode {
			return true, w.SetMagnetMode(player, SettleMode)
		}
		return false, false
	}
	if carrier < 0 {
		return true, false
	}
	p := w.Peeps[carrier]
	if p.Flags&InBattle != 0 {
		return true, false
	}
	goTo := w.Magnets[player].GoTo
	if inMap(goTo) && max(abs(p.AtPos%MapWidth-goTo%MapWidth), abs(p.AtPos/MapWidth-goTo/MapWidth)) > 1 && w.MapBlk[goTo] != WaterBlock && w.MapBlk[goTo] != SwampBlock && w.MapBlk[goTo] != RockBlock {
		if p.Flags == InTown {
			return true, w.SetMagnetTo(player, goTo)
		}
		return true, false
	}
	target, waypoint, _ := w.advancedMarchRoute(carrier)
	if target < 0 {
		stats.QuakeCount = w.GameTurn + 30*8
		return true, w.SetMagnetMode(player, SettleMode)
	}
	stats.NoQuakes = target + 1
	if waypoint != goTo || w.Magnets[player].Flags != MagnetMode {
		return true, w.SetMagnetTo(player, waypoint)
	}
	return true, false
}

func (w *World) advancedMarchRoute(index int) (target, waypoint, steps int) {
	if !w.validPeep(index) || w.Peeps[index].Player > DevilPlayer {
		return -1, 0, 0
	}
	const cells = MapWidth * MapHeight
	var dist, parent [cells]int16
	for i := range dist {
		dist[i] = -1
	}
	var queue [cells]int
	start := w.Peeps[index].AtPos
	dist[start] = 0
	queue[0] = start
	for head, tail := 0, 1; head < tail; head++ {
		pos := queue[head]
		for _, delta := range toOffset {
			next := pos + delta
			if !inMap(next) || dist[next] >= 0 || w.validMove(pos, delta) != 0 || w.MapBlk[next] == SwampBlock {
				continue
			}
			dist[next] = dist[pos] + 1
			parent[next] = int16(pos)
			queue[tail] = next
			tail++
		}
	}
	target, best := -1, -1<<30
	p := w.Peeps[index]
	for i, q := range w.Peeps {
		if q.Population <= 0 || q.Player == p.Player || q.Flags&InRuin != 0 || !inMap(q.AtPos) || dist[q.AtPos] < 0 {
			continue
		}
		d := int(dist[q.AtPos])
		remaining := p.Population - d*w.walkDeath()*2
		if remaining*max(1, p.Weapons) < q.Population*max(1, q.Weapons)*3/2 {
			continue
		}
		score := 10000 - d*100 - q.Population
		if q.Flags == InTown {
			score += 2000
		}
		if score > best {
			target, best = i, score
		}
	}
	if target < 0 {
		return -1, 0, 0
	}
	waypoint = w.Peeps[target].AtPos
	steps = int(dist[waypoint])
	for dist[waypoint] > 8 {
		waypoint = int(parent[waypoint])
	}
	for waypoint != start && !w.advancedStraightMarch(start, waypoint) {
		waypoint = int(parent[waypoint])
	}
	return target, waypoint, steps
}

func (w *World) advancedStraightMarch(start, target int) bool {
	if !inMap(start) || !inMap(target) {
		return false
	}
	for start != target {
		dx, dy := sign(target%MapWidth-start%MapWidth), sign(target/MapWidth-start/MapWidth)
		delta := dx + dy*MapWidth
		if w.validMove(start, delta) != 0 || w.MapBlk[start+delta] == SwampBlock {
			return false
		}
		start += delta
	}
	return true
}
