package populous

// advancedKnightEscape is a bounded fallback for a strategic knight after the
// historical moveKnightPeep has already returned noMove. It deliberately does
// not decide whether the historical direction was good: the caller owns that
// decision and must invoke this only for noMove.
//
// The BFS ignores MagnetLastMove so a knight can backtrack out of a dead end.
// It otherwise accepts only the same safe land a knight can traverse: no water,
// hard rock or swamp. The current HeadFor target is retained whenever reachable;
// only an invalid or unreachable target may be replaced. No RNG or mana is
// consumed, and HeadFor is the only state changed on success.
func (w *World) advancedKnightEscape(index int) (move int, ok bool) {
	if w == nil || w.War || index < 0 || index >= len(w.Peeps) {
		return 0, false
	}
	knight := w.Peeps[index]
	if knight.Population <= 0 || !isHeadedPeep(knight) || !inMap(knight.AtPos) {
		return 0, false
	}

	const cells = MapWidth * MapHeight
	var distance [cells]int16
	var firstStep [cells]int16
	for pos := range distance {
		distance[pos] = -1
	}
	start := knight.AtPos
	var queue [cells]int
	queue[0] = start
	distance[start] = 0
	head, tail := 0, 1
	for head < tail {
		pos := queue[head]
		head++
		for _, delta := range toOffset {
			next := pos + delta
			if !advancedKnightEscapeStep(w, pos, next, delta) || distance[next] >= 0 {
				continue
			}
			distance[next] = distance[pos] + 1
			if pos == start {
				firstStep[next] = int16(delta)
			} else {
				firstStep[next] = firstStep[pos]
			}
			queue[tail] = next
			tail++
		}
	}

	target := knight.HeadFor - 1
	selected := -1
	if w.validKnightTarget(index, target) && distance[w.Peeps[target].AtPos] >= 0 {
		selected = target
	} else {
		bestDistance := int16(cells + 1)
		for candidate := range w.Peeps {
			if candidate == index || !w.validKnightTarget(index, candidate) {
				continue
			}
			candidatePos := w.Peeps[candidate].AtPos
			if distance[candidatePos] <= 0 || distance[candidatePos] >= bestDistance {
				continue
			}
			selected = candidate
			bestDistance = distance[candidatePos]
		}
	}
	if selected < 0 {
		return 0, false
	}

	move = int(firstStep[w.Peeps[selected].AtPos])
	if move == 0 {
		return 0, false
	}
	w.Peeps[index].HeadFor = selected + 1
	return move, true
}

func advancedKnightEscapeStep(w *World, from, to, delta int) bool {
	return w != nil && inMap(from) && inMap(to) && w.validMove(from, delta) == 0 && int(w.MapBlk[to]) != SwampBlock
}
