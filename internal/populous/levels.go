package populous

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

const LevelRecordSize = 10

type Level struct {
	Number             int
	Code               string
	EnemyRating        byte
	EnemyReactionSpeed byte
	EnemyPowers        byte
	PlayerPowers       byte
	GameMode           byte
	Terrain            byte
	PlayerPopulation   byte
	EnemyPopulation    byte
	SeedOffset         uint16
}

var startWords = [32]string{
	"RING", "VERY", "KILL", "SHAD", "HURT", "WEAV", "MIN", "EOA",
	"COR", "JOS", "ALP", "HAM", "BUR", "BIN", "TIM", "BAD",
	"FUT", "MOR", "SAD", "CAL", "IMM", "SUZ", "NIM", "LOW",
	"SCO", "HOB", "DOU", "BIL", "QAZ", "SWA", "BUG", "SHI",
}

var midWords = [32]string{
	"OUT", "QAZ", "ING", "OGO", "QUE", "LOP", "SOD", "HIP",
	"KOP", "WIL", "IKE", "DIE", "IN", "AS", "MP", "DI",
	"OZ", "EA", "US", "GB", "CE", "ME", "DE", "PE",
	"OX", "A", "E", "I", "O", "U", "T", "Y",
}

var endWords = [32]string{
	"HILL", "TORY", "HOLE", "PERT", "MAR", "CON", "LOW", "DOR",
	"LIN", "ING", "HAM", "OLD", "PIL", "BAR", "MET", "END",
	"LAS", "OUT", "LUG", "ILL", "ICK", "PAL", "DON", "ORD",
	"OND", "BOY", "JOB", "ER", "ED", "ME", "AL", "T",
}

func LoadLevels(r io.Reader) ([]Level, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data)%LevelRecordSize != 0 {
		return nil, fmt.Errorf("level.dat size %d is not a multiple of %d", len(data), LevelRecordSize)
	}

	levels := make([]Level, 0, (len(data)/LevelRecordSize)*5)
	for record := 0; record < len(data)/LevelRecordSize; record++ {
		base := data[record*LevelRecordSize:]
		for n := 0; n < 5; n++ {
			number := record*5 + n
			levels = append(levels, Level{
				Number:             number,
				Code:               CodeForLevel(number),
				EnemyRating:        base[0],
				EnemyReactionSpeed: base[1],
				EnemyPowers:        base[2],
				PlayerPowers:       base[3],
				GameMode:           base[4],
				Terrain:            base[5],
				PlayerPopulation:   base[6],
				EnemyPopulation:    base[7],
				SeedOffset:         binary.BigEndian.Uint16(base[8:10]),
			})
		}
	}
	return levels, nil
}

func CodeForLevel(level int) string {
	if level < 0 {
		level = 0
	}
	code := nextRandom(uint16(level * 5))
	return CodeForWorldCode(code)
}

func CodeForWorldCode(code uint16) string {
	code &= 0x7fff
	start := int(code & 0x1f)
	mid := int((code >> 5) & 0x1f)
	end := int((code >> 10) & 0x1f)
	return startWords[start] + midWords[mid] + endWords[end]
}

func DecodeLevelCode(input string, maxLevel int) (int, bool) {
	clean := strings.ToUpper(strings.TrimSpace(input))
	if clean == "GENESIS" {
		return 0, true
	}
	if maxLevel < 0 {
		maxLevel = 1000
	}
	for level := 0; level <= maxLevel; level++ {
		if CodeForLevel(level) == clean {
			return level, true
		}
	}
	return 0, false
}

func nextRandom(seed uint16) uint16 {
	v := uint32(seed)*0x24a1 + 0x24df
	return uint16(v & 0x7fff)
}
