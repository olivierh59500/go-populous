package populous

type WorldSnapshot struct {
	Level              Level
	Terrain            int
	GameTurn           int
	Alt                [EndWidth * EndWidth]int
	MapAlt             [MapWidth * MapHeight]byte
	MapBlk             [MapWidth * MapHeight]byte
	MapBk2             [MapWidth * MapHeight]byte
	MapWho             [MapWidth * MapHeight]byte
	MapSteps           [MapWidth * MapHeight]uint16
	Peeps              []Peep
	Magnets            [2]Magnet
	Computer           [2]ComputerStats
	ComputerControlled [2]bool
	BattleWon          [2]int
	War                bool
	Scores             [2]int
	Score              int
	ScorePlayer        int
	RNG                uint16
}

func (w *World) Snapshot() WorldSnapshot {
	if w == nil {
		return WorldSnapshot{}
	}
	w.commitLocalScoreView()
	snapshot := WorldSnapshot{
		Level:              w.Level,
		Terrain:            w.Terrain,
		GameTurn:           w.GameTurn,
		Alt:                w.Alt,
		MapAlt:             w.MapAlt,
		MapBlk:             w.MapBlk,
		MapBk2:             w.MapBk2,
		MapWho:             w.MapWho,
		MapSteps:           w.MapSteps,
		Magnets:            w.Magnets,
		Computer:           w.Computer,
		ComputerControlled: w.ComputerControlled,
		BattleWon:          w.BattleWon,
		War:                w.War,
		Scores:             w.Scores,
		Score:              w.Score,
		ScorePlayer:        w.ScorePlayer,
		RNG:                uint16(w.rng),
	}
	snapshot.Peeps = append([]Peep(nil), w.Peeps...)
	return snapshot
}

func WorldFromSnapshot(snapshot WorldSnapshot, rules TerrainRules) *World {
	if rules == (TerrainRules{}) {
		rules = DefaultTerrainRules()
	}
	terrain := snapshot.Terrain
	if terrain < 0 || terrain > 3 {
		terrain = int(snapshot.Level.Terrain)
	}
	if terrain < 0 || terrain > 3 {
		terrain = 0
	}
	world := &World{
		Level:              snapshot.Level,
		Rules:              rules,
		Terrain:            terrain,
		GameTurn:           snapshot.GameTurn,
		Alt:                snapshot.Alt,
		MapAlt:             snapshot.MapAlt,
		MapBlk:             snapshot.MapBlk,
		MapBk2:             snapshot.MapBk2,
		MapWho:             snapshot.MapWho,
		MapSteps:           snapshot.MapSteps,
		Peeps:              append([]Peep(nil), snapshot.Peeps...),
		Magnets:            snapshot.Magnets,
		Computer:           snapshot.Computer,
		ComputerControlled: snapshot.ComputerControlled,
		BattleWon:          snapshot.BattleWon,
		War:                snapshot.War,
		Scores:             snapshot.Scores,
		Score:              snapshot.Score,
		ScorePlayer:        snapshot.ScorePlayer,
		rng:                lcg(snapshot.RNG),
	}
	if world.ScorePlayer < GodPlayer || world.ScorePlayer > DevilPlayer {
		world.ScorePlayer = GodPlayer
	}
	// Scores was added after the original save/snapshot format. A decoded old
	// snapshot has a zero array, so recover its selected player's accumulated
	// score and initialize the other side from the level.
	legacyScores := world.Scores == [2]int{}
	for player := range world.Scores {
		if world.Scores[player] == 0 {
			world.Scores[player] = world.initialScoreFor(player)
		}
	}
	if legacyScores && snapshot.Score != 0 {
		world.Scores[world.ScorePlayer] = snapshot.Score
	}
	world.Score = world.Scores[world.ScorePlayer]
	return world
}
