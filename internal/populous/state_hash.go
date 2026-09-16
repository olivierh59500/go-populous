package populous

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

// StateHashVersion changes whenever the canonical state representation
// changes. Multiplayer peers must agree on this value during their handshake.
const StateHashVersion uint16 = 2

// StateHash returns a platform-independent SHA-256 digest of every value that
// can affect future shared simulation. Presentation-only queued sound events
// and the local Score/ScorePlayer view are intentionally excluded. Scores,
// which contains the authoritative totals for both players, is included.
func (w *World) StateHash() [sha256.Size]byte {
	if w == nil {
		return sha256.Sum256([]byte("go-populous:nil-world"))
	}

	digest := sha256.New()
	digest.Write([]byte("go-populous:world-state\x00"))
	hashUint16(digest, StateHashVersion)

	hashLevel(digest, w.Level)
	hashTerrainRules(digest, w.Rules)
	hashInt(digest, w.Terrain)
	hashInt(digest, w.GameTurn)
	for _, value := range w.Alt {
		hashInt(digest, value)
	}
	digest.Write(w.MapAlt[:])
	digest.Write(w.MapBlk[:])
	digest.Write(w.MapBk2[:])
	digest.Write(w.MapWho[:])
	for _, value := range w.MapSteps {
		hashUint16(digest, value)
	}

	hashUint32(digest, uint32(len(w.Peeps)))
	for _, peep := range w.Peeps {
		digest.Write([]byte{peep.Flags, peep.Player})
		hashInt(digest, peep.IQ)
		hashInt(digest, peep.Weapons)
		hashInt(digest, peep.Population)
		hashInt(digest, peep.BattlePopulation)
		hashInt(digest, peep.AtPos)
		hashInt(digest, peep.Direction)
		hashInt(digest, peep.Frame)
		hashInt(digest, peep.HeadFor)
		hashInt(digest, peep.InOut)
		hashInt(digest, peep.Status)
		hashBool(digest, peep.LandComplete)
		hashInt(digest, peep.MagnetLastMove)
	}

	for _, magnet := range w.Magnets {
		hashInt(digest, magnet.Carried)
		hashInt(digest, magnet.GoTo)
		hashInt(digest, magnet.Flags)
		hashInt(digest, magnet.NoTowns)
		hashInt(digest, magnet.Population)
		hashInt(digest, magnet.Mana)
	}
	for _, computer := range w.Computer {
		hashInt(digest, computer.Mode)
		hashInt(digest, computer.Skill)
		hashInt(digest, computer.Speed)
		hashInt(digest, computer.QuakeCount)
		hashInt(digest, computer.NoQuakes)
		hashInt(digest, computer.NoSwamps)
		hashInt(digest, computer.NoTowns)
		hashInt(digest, computer.NoCastles)
		hashInt(digest, computer.Arrived)
		hashInt(digest, computer.LastBattle)
		hashInt(digest, computer.Best1)
		hashInt(digest, computer.Best2)
		hashInt(digest, computer.MyBest)
		hashInt(digest, computer.DoneTurn)
	}
	for _, controlled := range w.ComputerControlled {
		hashBool(digest, controlled)
	}
	for _, won := range w.BattleWon {
		hashInt(digest, won)
	}
	hashBool(digest, w.War)
	for player, score := range w.Scores {
		if score == 0 {
			score = w.initialScoreFor(player)
		}
		hashInt(digest, score)
	}
	hashUint16(digest, uint16(w.rng))

	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func hashLevel(digest hash.Hash, level Level) {
	hashInt(digest, level.Number)
	hashString(digest, level.Code)
	digest.Write([]byte{
		level.EnemyRating,
		level.EnemyReactionSpeed,
		level.EnemyPowers,
		level.PlayerPowers,
		level.GameMode,
		level.Terrain,
		level.PlayerPopulation,
		level.EnemyPopulation,
	})
	hashUint16(digest, level.SeedOffset)
}

func hashTerrainRules(digest hash.Hash, rules TerrainRules) {
	hashInt(digest, rules.WalkDeath)
	for _, value := range rules.PopulationAdd {
		hashInt(digest, value)
	}
	for _, value := range rules.ManaAdd {
		hashInt(digest, value)
	}
	for _, value := range rules.WeaponsAdd {
		hashInt(digest, value)
	}
	for _, value := range rules.BattleAdd1 {
		hashInt(digest, value)
	}
	for _, value := range rules.BattleAdd2 {
		hashInt(digest, value)
	}
	digest.Write(rules.MapColor[:])
	hashInt(digest, rules.SpriteSet)
}

func hashString(digest hash.Hash, value string) {
	hashUint32(digest, uint32(len(value)))
	digest.Write([]byte(value))
}

func hashBool(digest hash.Hash, value bool) {
	if value {
		digest.Write([]byte{1})
		return
	}
	digest.Write([]byte{0})
}

func hashInt(digest hash.Hash, value int) {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], uint64(int64(value)))
	digest.Write(data[:])
}

func hashUint16(digest hash.Hash, value uint16) {
	var data [2]byte
	binary.LittleEndian.PutUint16(data[:], value)
	digest.Write(data[:])
}

func hashUint32(digest hash.Hash, value uint32) {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], value)
	digest.Write(data[:])
}
