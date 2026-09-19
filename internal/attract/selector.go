package attract

import (
	"math/rand/v2"

	"go-populous/internal/populous"
)

// Selector visits a shuffled deck of original campaign levels. Its private RNG
// makes a recording reproducible without consuming the simulation's RNG or
// changing a level's original landscape seed and opponent profile.
type Selector struct {
	levels   []populous.Level
	rng      *rand.Rand
	next     int
	previous landscape
	hasLast  bool
}

type landscape struct {
	seed    uint16
	terrain byte
}

func landscapeFor(level populous.Level) landscape {
	terrain := level.Terrain
	if terrain > 3 {
		terrain = 0
	}
	return landscape{uint16(level.SeedOffset) + uint16((level.Number*5)&7), terrain}
}

// NewSelector copies levels, so shuffling never changes the campaign menu. Seed
// zero is valid; the application chooses a fresh random seed unless requested
// otherwise. An empty catalog falls back to the original tutorial level.
func NewSelector(levels []populous.Level, seed uint64) *Selector {
	if len(levels) == 0 {
		levels = []populous.Level{populous.TutorialLevel()}
	}
	return &Selector{
		levels: append([]populous.Level(nil), levels...),
		rng:    rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		next:   len(levels),
	}
}

// Next preserves each level verbatim. The strategic AI always takes the human's
// original side, leaving the historical Devil AI's profile intact. Levels are
// not repeated until the deck is exhausted; consecutive identical landscapes
// are avoided whenever another landscape remains in the deck.
func (selector *Selector) Next() (populous.Level, int) {
	if selector.next == len(selector.levels) {
		selector.rng.Shuffle(len(selector.levels), func(i, j int) {
			selector.levels[i], selector.levels[j] = selector.levels[j], selector.levels[i]
		})
		selector.next = 0
	}
	index := selector.next
	if selector.hasLast && landscapeFor(selector.levels[index]) == selector.previous {
		for candidate := index + 1; candidate < len(selector.levels); candidate++ {
			if landscapeFor(selector.levels[candidate]) != selector.previous {
				selector.levels[index], selector.levels[candidate] = selector.levels[candidate], selector.levels[index]
				break
			}
		}
	}
	level := selector.levels[index]
	selector.next++
	selector.previous = landscapeFor(level)
	selector.hasLast = true
	return level, populous.GodPlayer
}
