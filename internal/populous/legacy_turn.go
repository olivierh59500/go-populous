package populous

// The original send[] buffer has one replaceable order per side. Legacy AI
// decisions are made while people move, but their effects happen after everyone
// has been processed. Public commands retain the modern input/lockstep API.
type legacyTurnState struct {
	active                                         bool
	advancedPlayer                                 int
	orders                                         [2]Command
	bestLife, oldest, youngest, castles, lastTowns [2]int
}

func (w *World) beginLegacyTurn() {
	w.legacyTurn = legacyTurnState{active: true, advancedPlayer: w.legacyTurn.advancedPlayer, youngest: [2]int{19999, 19999}}
	for player := range w.Computer {
		s := &w.Computer[player]
		w.legacyTurn.lastTowns[player] = s.NoTowns + s.NoCastles*3
		s.Best1, s.Best2, s.MyBest = -1, -1, -1
		w.Magnets[player].Population = 0
		w.Magnets[player].NoTowns = 0
		if w.GameTurn&1 == 0 {
			w.Magnets[player].Mana++
		}
	}
}

func (w *World) legacyOrder(player int, kind CommandKind, x, y, value int) bool {
	if player < 0 || player > 1 {
		return false
	}
	command := Command{Player: player, Kind: kind, X: x, Y: y, Value: value}
	if w.legacyTurn.active {
		w.legacyTurn.orders[player] = command
		return true
	}
	return w.applyLegacyOrder(command)
}

func (w *World) applyLegacyOrder(command Command) bool {
	p := command.Player
	switch command.Kind {
	case CommandInvalid:
		return false
	case CommandSetMagnet:
		// Unlike the intentional player convenience API, do_magnet only
		// changes the destination, and requires a carrier at execution time.
		if w.War || w.carriedPeepIndex(p) < 0 || w.Magnets[p].Mana < ManaMagnetCost {
			return false
		}
		w.Magnets[p].Mana -= ManaMagnetCost
		w.Magnets[p].GoTo = clamp(command.X, 0, MapWidth-1) + clamp(command.Y, 0, MapHeight-1)*MapWidth
		w.queueSound(TuneMagnet)
		return true
	case CommandRaise:
		if w.War {
			return w.forceRaiseAt(command.X, command.Y)
		}
	}
	ok, _ := w.ApplyCommand(command)
	return ok
}

func (w *World) finishLegacyTurn() {
	orders := w.legacyTurn.orders
	for p := range w.Computer {
		w.Computer[p].NoTowns = w.Magnets[p].NoTowns
		w.Computer[p].NoCastles = w.legacyTurn.castles[p]
	}
	// Reset before execution, as get_message does. No order survives the tick
	// boundary, including a rejected order or one superseded by a swimmer.
	w.resetComputerActionSlots()
	w.legacyTurn = legacyTurnState{}
	for _, command := range orders {
		w.applyLegacyOrder(command)
	}
}

func (w *World) collectLegacyTown(index, life int) {
	if !w.legacyTurn.active {
		return
	}
	p := &w.Peeps[index]
	side := int(p.Player)
	if p.Frame == LastTown {
		w.legacyTurn.castles[side]++
	}
	if life > w.legacyTurn.bestLife[side] {
		w.legacyTurn.bestLife[side] = life
		w.Computer[side^1].Best1 = index
	}
	age := w.GameTurn - p.BattlePopulation
	if age >= w.legacyTurn.oldest[side] {
		w.legacyTurn.oldest[side] = age
		w.Computer[side^1].Best2 = index
	}
	if age < w.legacyTurn.youngest[side] {
		w.legacyTurn.youngest[side] = age
		w.Computer[side].MyBest = index
	}
}

func (w *World) signalWaitingOccupant(index int) {
	p := &w.Peeps[index]
	if p.Population <= 0 || !inMap(p.AtPos) {
		return
	}
	other := int(w.MapWho[p.AtPos]) - 1
	if other == index || !w.validPeep(other) {
		return
	}
	target := &w.Peeps[other]
	if target.Frame == 0 && target.Flags&InWater == 0 {
		target.Flags |= WaitForMe
		target.Frame = FirstWaitSprite
		target.BattlePopulation = 0
	}
}
