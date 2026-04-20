package populous

import (
	"encoding/binary"
	"fmt"
)

const LandHeaderSize = 114

type TerrainRules struct {
	WalkDeath     int
	PopulationAdd [11]int
	ManaAdd       [11]int
	WeaponsAdd    [11]int
	BattleAdd1    [11]int
	BattleAdd2    [3]int
	MapColor      [16]byte
	SpriteSet     int
}

func DefaultTerrainRules() TerrainRules {
	return TerrainRules{
		WalkDeath:     1,
		PopulationAdd: [11]int{0, 1, 1, 2, 2, 3, 3, 3, 4, 4, 5},
		ManaAdd:       [11]int{0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 20},
		WeaponsAdd:    [11]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 20},
		BattleAdd1:    [11]int{50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 2000},
		BattleAdd2:    [3]int{100, 1000, 3000},
		SpriteSet:     0,
	}
}

func DecodeTerrainRules(data []byte) (TerrainRules, error) {
	if len(data) < LandHeaderSize {
		return TerrainRules{}, fmt.Errorf("land header too short: got %d, need %d", len(data), LandHeaderSize)
	}

	var rules TerrainRules
	offset := 0
	readShort := func() int {
		value := int(int16(binary.BigEndian.Uint16(data[offset : offset+2])))
		offset += 2
		return value
	}

	rules.WalkDeath = readShort()
	for i := range rules.PopulationAdd {
		rules.PopulationAdd[i] = readShort()
	}
	for i := range rules.ManaAdd {
		rules.ManaAdd[i] = readShort()
	}
	for i := range rules.WeaponsAdd {
		rules.WeaponsAdd[i] = readShort()
	}
	for i := range rules.BattleAdd1 {
		rules.BattleAdd1[i] = readShort()
	}
	for i := range rules.BattleAdd2 {
		rules.BattleAdd2[i] = readShort()
	}
	copy(rules.MapColor[:], data[offset:offset+len(rules.MapColor)])
	offset += len(rules.MapColor)
	rules.SpriteSet = readShort()
	if rules.SpriteSet < 0 || rules.SpriteSet > 3 {
		rules.SpriteSet = 0
	}
	return rules, nil
}
