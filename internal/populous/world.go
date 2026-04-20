package populous

const (
	MapWidth  = 64
	MapHeight = 64
	EndWidth  = 65

	WaterBlock       = 0
	FlatBlock        = 15
	FarmBlock        = 31
	FirstTown        = 32
	LastTown         = 42
	CityTower        = 41
	CityCentre       = 42
	CityWall1        = 43
	CityWall2        = 44
	FirstRuinTown    = 54
	SwampBlock       = 53
	RockBlock        = 47
	TreeBlock        = 50
	BadLand          = 66
	BlocksPerLand    = 70
	FlagSprite       = 64
	ManaSprite       = 69
	GodsHandSprite   = 78
	DevilsHandSprite = 79
	CrosshairSprite  = 84
	SideWall1        = 77
	SideWall2        = 76

	GodPlayer   = 0
	DevilPlayer = 1

	InTown     = 0x01
	OnMove     = 0x02
	InEffect   = 0x04
	InBattle   = 0x08
	InWater    = 0x10
	WaitForMe  = 0x20
	IAmWaiting = 0x40
	InRuin     = 0x80

	MagnetMode = 0
	SettleMode = 1
	JoinMode   = 2
	FightMode  = 3

	ManaFloor       = -250
	ManaPointCost   = 10
	ManaMagnetCost  = 200
	ManaQuakeCost   = 2500
	ManaSwampCost   = 5000
	ManaKnightCost  = 7500
	ManaVolcanoCost = 10000
	ManaFloodCost   = 40000
	ManaWarCost     = 80000

	StartFood    = 50
	FlatLandFood = 15
	RockLandFood = -15
	MaxFood      = FlatLandFood*17 + StartFood
	CityFood     = MaxFood * 10
	MaxMensa     = 4
	MaxPeeps     = 208

	BadPeople    = 16
	KnightPeople = 32

	BattleFirstFrame  = 70
	BattleLastFrame   = 73
	VictorySprite     = 85
	LastVictorySprite = 88
	KnightWinSprite   = 117

	FirstWaterSprite = 93
	LastWaterSprite  = 96
	FirstWaitSprite  = 101
	LastWaitSprite   = 102
	FireSprite       = 105
	FirstKnightWater = 109
	LastKnightWater  = 112
	KnightWaitSprite = 125
	BlueVsPeepSprite = 130
	RedVsPeepSprite  = 134
	BlueVsRedSprite  = 138
	KnightStatus     = 1

	TuneSwamp    = 66
	TuneSword1   = 67
	TuneSword2   = 68
	TuneKnighted = 69
	TuneFire     = 70
	TuneHeart1   = 71
	TuneHeart2   = 72
	TuneWar      = 73
	TuneFlood    = 74
	TuneVolcano  = 75
	TuneQuake    = 76
	TuneMagnet   = 77

	noWoods = 22
	noTrees = 30

	ScoreQuake       = 25
	ScoreSwamp       = 50
	ScoreVolcano     = 100
	ScoreKnight      = 150
	ScoreFlood       = 250
	ScoreWar         = 5000
	ScoreBattle      = 5000
	ScoreYourOptions = 1000
	ScoreHisOptions  = 1000
	ScoreSpeed       = 15
	ScorePeople      = 10
	ScoreWon         = 10

	DevilMakesKnight = 3000

	GameWaterFatal  = 0x01
	GameSwampRemain = 0x02
	GameNoBuild     = 0x04
	GameOnlyRaise   = 0x08
	GameRaiseTown   = 0x10
)

const (
	computerLand = 1 << iota
	computerTown
	computerLeader
	computerQuake
	computerSwamp
	computerKnight
	computerVolcano
	computerFlood
	computerWar
)

const (
	moveToFlat = iota
	moveToBattle
	moveToEnemy
	moveToFriend
	moveToEmpty
)

const noMove = 999

const (
	ResultOngoing = iota
	ResultWon
	ResultLost
)

var offsetVector = [...]int{
	0, -64, 1, 64, -1, -63, 65, 63, -65,
	-128, 2, 128, -2, -126, 130, 126, -130,
	-127, -62, 66, 129, 127, 62, -66, -129,
}

var bigCity = [...]byte{
	CityCentre, CityWall2, CityWall1,
	CityWall2, CityWall1, CityTower,
	CityTower, CityTower, CityTower,
}

var computerFlatSearch = [...][2]int{
	{-4, -4}, {-4, -3}, {-4, -2}, {-4, -1}, {-4, 0}, {-4, 1}, {-4, 2}, {-4, 3}, {-4, 4},
	{-3, -4}, {-3, -3}, {-3, -2}, {-3, -1}, {-3, 0}, {-3, 1}, {-3, 2}, {-3, 3}, {-3, 4},
	{4, -4}, {4, -3}, {4, -2}, {4, -1}, {4, 0}, {4, 1}, {4, 2}, {4, 3}, {4, 4},
	{3, -4}, {3, -3}, {3, -2}, {3, -1}, {3, 0}, {3, 1}, {3, 2}, {3, 3}, {3, 4},
	{-2, -4}, {-2, -3}, {-2, -2}, {-2, -1}, {-2, 0}, {-2, 1}, {-2, 2}, {-2, 3}, {-2, 4},
	{2, -4}, {2, -3}, {2, -2}, {2, -1}, {2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4},
	{-1, -4}, {-1, -3}, {-1, -2}, {-1, -1}, {-1, 0}, {-1, 1}, {-1, 2}, {-1, 3}, {-1, 4},
	{1, -4}, {1, -3}, {1, -2}, {1, -1}, {1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4},
	{0, -4}, {0, -3}, {0, -2}, {0, -1}, {0, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4},
}

var toDelta = [...]int{7, 6, 5, 0, 0, 4, 1, 2, 3}
var toOffset = [...]int{-64, -63, 1, 65, 64, 63, -1, -65}
var opposite = [...]int{4, 5, 6, 7, 0, 1, 2, 3}

type World struct {
	Level              Level
	Rules              TerrainRules
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
	Score              int
	ScorePlayer        int
	SoundEvents        []int

	rng lcg
}

type Peep struct {
	Flags            byte
	Player           byte
	IQ               int
	Weapons          int
	Population       int
	BattlePopulation int
	AtPos            int
	Direction        int
	Frame            int
	HeadFor          int
	InOut            int
	Status           int
	LandComplete     bool
	MagnetLastMove   int
}

type Magnet struct {
	Carried    int
	GoTo       int
	Flags      int
	NoTowns    int
	Population int
	Mana       int
}

type ComputerStats struct {
	Mode       int
	Skill      int
	Speed      int
	QuakeCount int
	NoQuakes   int
	NoSwamps   int
	NoTowns    int
	NoCastles  int
	Arrived    int
	LastBattle int
	Best1      int
	Best2      int
	MyBest     int
	DoneTurn   int
}

type PlayerSummary struct {
	Population int
	BattlesWon int
	Knights    int
	Towns      int
	Castles    int
}

func GenerateWorld(level Level) *World {
	return GenerateWorldWithRules(level, DefaultTerrainRules())
}

func TutorialLevel() Level {
	return Level{
		Number:             0,
		Code:               "TUTORIAL",
		EnemyRating:        10,
		EnemyReactionSpeed: 10,
		PlayerPowers:       0x07,
		GameMode:           GameWaterFatal,
		Terrain:            1,
		PlayerPopulation:   15,
		EnemyPopulation:    3,
		SeedOffset:         27068,
	}
}

func GenerateWorldWithRules(level Level, rules TerrainRules) *World {
	if rules == (TerrainRules{}) {
		rules = DefaultTerrainRules()
	}
	terrain := int(level.Terrain)
	if terrain < 0 || terrain > 3 {
		terrain = 0
	}

	w := &World{
		Level:   level,
		Rules:   rules,
		Terrain: terrain,
		rng:     lcg(uint16(level.SeedOffset) + uint16((level.Number*5)&7)),
	}
	w.makeAlt()
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.makeWoodsRocks()
	w.initComputerStats()
	w.ComputerControlled = [2]bool{false, true}
	w.ScorePlayer = GodPlayer
	w.placeFirstPeople()
	return w
}

func (w *World) ConfigureTutorial() {
	if w == nil {
		return
	}
	w.Level = TutorialLevel()
	w.Computer[GodPlayer] = ComputerStats{
		Mode:     computerLand | computerTown | computerLeader | computerQuake | computerSwamp | computerKnight,
		Skill:    10,
		Speed:    10,
		Best1:    -1,
		Best2:    -1,
		MyBest:   -1,
		DoneTurn: 1,
	}
	w.Computer[DevilPlayer] = ComputerStats{
		Mode:     computerLand | computerTown,
		Skill:    10,
		Speed:    10,
		Best1:    -1,
		Best2:    -1,
		MyBest:   -1,
		DoneTurn: 1,
	}
	w.ComputerControlled = [2]bool{false, true}
	w.Magnets[GodPlayer].Mana = ManaVolcanoCost
}

func (w *World) initComputerStats() {
	center := MapWidth/2 + MapWidth*(MapHeight/2)
	for player := range w.Computer {
		powers := int(w.Level.PlayerPowers)
		if player == DevilPlayer {
			powers = int(w.Level.EnemyPowers)
		}
		noQuakes := w.rng.next() % 3
		w.Computer[player] = ComputerStats{
			Mode:       computerLand | computerTown | computerLeader | (powers << 3),
			Skill:      int(w.Level.EnemyRating),
			Speed:      int(w.Level.EnemyReactionSpeed),
			NoQuakes:   noQuakes,
			NoSwamps:   noQuakes + 1 + w.rng.next()%5,
			LastBattle: center,
			Best1:      -1,
			Best2:      -1,
			MyBest:     -1,
			DoneTurn:   1,
		}
		if w.Computer[player].Skill <= 0 {
			w.Computer[player].Skill = 1
		}
		if w.Computer[player].Speed <= 0 {
			w.Computer[player].Speed = 1
		}
	}
}

func (w *World) makeAlt() {
	w.makeThing(2, 4)
	w.makeThing(4, 2)
	w.makeThing(3, 3)
}

func (w *World) makeThing(gx, gy int) {
	x := w.rng.next() % MapWidth
	y := w.rng.next() % MapWidth
	for w.raisePoint(x, y) != 6 {
		x += -gx + w.rng.next()%(gx*2+1)
		y += -gy + w.rng.next()%(gy*2+1)
		if x < 0 {
			x = 0
		}
		if x > MapWidth {
			x = MapWidth
		}
		if y < 0 {
			y = 0
		}
		if y > MapHeight {
			y = MapHeight
		}
	}
}

func (w *World) raisePoint(x, y int) int {
	return w.raisePointTracked(x, y, nil)
}

func (w *World) raisePointTracked(x, y int, bounds *altBounds) int {
	if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
		return 0
	}
	pos := x + EndWidth*y
	if w.Alt[pos] >= 8 {
		return w.Alt[pos]
	}

	w.Alt[pos]++
	if bounds != nil {
		bounds.record(x, y)
	}
	for _, n := range [][2]int{
		{x + 1, y}, {x + 1, y + 1}, {x, y + 1}, {x - 1, y + 1},
		{x - 1, y}, {x - 1, y - 1}, {x, y - 1}, {x + 1, y - 1},
	} {
		if n[0] < 0 || n[0] > MapWidth || n[1] < 0 || n[1] > MapHeight {
			continue
		}
		if w.Alt[pos]-w.Alt[n[0]+EndWidth*n[1]] > 1 {
			w.raisePointTracked(n[0], n[1], bounds)
		}
	}
	return w.Alt[pos]
}

func (w *World) lowerPointTracked(x, y int, bounds *altBounds) int {
	if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
		return 0
	}
	pos := x + EndWidth*y
	if w.Alt[pos] == 0 {
		return 0
	}

	w.Alt[pos]--
	if bounds != nil {
		bounds.record(x, y)
	}
	for _, n := range [][2]int{
		{x + 1, y}, {x + 1, y + 1}, {x, y + 1}, {x - 1, y + 1},
		{x - 1, y}, {x - 1, y - 1}, {x, y - 1}, {x + 1, y - 1},
	} {
		if n[0] < 0 || n[0] > MapWidth || n[1] < 0 || n[1] > MapHeight {
			continue
		}
		if w.Alt[n[0]+EndWidth*n[1]]-w.Alt[pos] > 1 {
			w.lowerPointTracked(n[0], n[1], bounds)
		}
	}
	return w.Alt[pos]
}

func (w *World) RaiseAt(player, x, y int) bool {
	return w.changeAltitude(player, x, y, true, false)
}

func (w *World) LowerAt(player, x, y int) bool {
	return w.changeAltitude(player, x, y, false, false)
}

func (w *World) PaintRaiseAt(x, y int) bool {
	return w.changeAltitude(GodPlayer, x, y, true, true)
}

func (w *World) PaintLowerAt(x, y int) bool {
	return w.changeAltitude(GodPlayer, x, y, false, true)
}

func (w *World) forceRaiseAt(x, y int) bool {
	if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
		return false
	}
	bounds := newAltBounds(x, y)
	w.raisePointTracked(x, y, bounds)
	if bounds.changed == 0 {
		return false
	}
	w.rebuildAltitudeBounds(bounds)
	return true
}

func (w *World) SetMagnetToTile(player, x, y int) bool {
	x = clamp(x, 0, MapWidth-1)
	y = clamp(y, 0, MapHeight-1)
	return w.SetMagnetTo(player, x+y*MapWidth)
}

func (w *World) SetMagnetTo(player, pos int) bool {
	if player < 0 || player >= len(w.Magnets) || !inMap(pos) {
		return false
	}
	if w.War {
		return false
	}
	if w.Magnets[player].Mana < ManaMagnetCost {
		return false
	}
	w.Magnets[player].Mana -= ManaMagnetCost
	w.Magnets[player].GoTo = pos
	w.Magnets[player].Flags = MagnetMode
	w.wakeMagnetCarrier(player)
	w.queueSound(TuneMagnet)
	return true
}

func (w *World) SetMagnetMode(player, mode int) bool {
	if player < 0 || player >= len(w.Magnets) {
		return false
	}
	if mode < MagnetMode || mode > FightMode {
		mode = SettleMode
	}
	w.Magnets[player].Flags = mode
	return true
}

func (w *World) SwampAtTile(player, x, y int) bool {
	x = clamp(x, 0, MapWidth-1)
	y = clamp(y, 0, MapHeight-1)
	return w.SwampAt(player, x, y)
}

func (w *World) SwampAt(player, startX, startY int) bool {
	if !w.spendPowerMana(player, ManaSwampCost, computerSwamp) {
		return false
	}
	if player == w.ScorePlayer {
		w.Score += ScoreSwamp
	}
	for i := 0; i < noTrees; i++ {
		x := startX + w.rng.next()%7 - 3
		y := startY + w.rng.next()%7 - 3
		if x < 0 || x >= MapWidth || y < 0 || y >= MapHeight {
			continue
		}
		pos := x + y*MapWidth
		block := int(w.MapBlk[pos])
		if (block == FlatBlock || block == FarmBlock+GodPlayer || block == FarmBlock+DevilPlayer || block == BadLand) && w.MapWho[pos] == 0 {
			w.MapBlk[pos] = SwampBlock
			w.MapBk2[pos] = 0
		}
	}
	return true
}

func (w *World) QuakeAtTile(player, x, y int) bool {
	x = clamp(x, 0, MapWidth-1)
	y = clamp(y, 0, MapHeight-1)
	return w.QuakeAt(player, x, y)
}

func (w *World) QuakeAt(player, x, y int) bool {
	if !w.spendPowerMana(player, ManaQuakeCost, computerQuake) {
		return false
	}
	w.queueSound(TuneQuake)
	if player == w.ScorePlayer {
		w.Score += ScoreQuake
	}
	bounds := newAltBounds(x, y)
	for pass := 0; pass < 2; pass++ {
		for yy := y; yy < y+9; yy++ {
			for xx := x; xx < x+9; xx++ {
				if xx < 0 || xx > MapWidth || yy < 0 || yy > MapHeight {
					continue
				}
				if w.Alt[xx+yy*EndWidth] == 0 {
					continue
				}
				switch w.rng.next() % 5 {
				case 1:
					w.raisePointTracked(xx, yy, bounds)
				case 2, 3, 4:
					w.lowerPointTracked(xx, yy, bounds)
				}
			}
		}
	}
	if bounds.changed > 0 {
		w.rebuildAltitudeBounds(bounds)
	}
	return true
}

func (w *World) VolcanoAtTile(player, x, y int) bool {
	x = clamp(x, 0, MapWidth-1)
	y = clamp(y, 0, MapHeight-1)
	return w.VolcanoAt(player, x, y)
}

func (w *World) VolcanoAt(player, x, y int) bool {
	if !w.spendPowerMana(player, ManaVolcanoCost, computerVolcano) {
		return false
	}
	w.queueSound(TuneVolcano)
	if player == w.ScorePlayer {
		w.Score += ScoreVolcano
	}
	bounds := newAltBounds(x, y)
	for ring := 0; ring <= 4; ring++ {
		for xx := ring; xx < 9-ring; xx++ {
			for yy := ring; yy < 9-ring; yy++ {
				switch w.rng.next() % 5 {
				case 1, 2, 4:
					w.raisePointTracked(x+xx, y+yy, bounds)
				}
			}
		}
	}

	protected := w.protectedPowerPositions()
	for yy := y; yy < y+8; yy++ {
		for xx := x; xx < x+8; xx++ {
			if xx < 0 || xx >= MapWidth || yy < 0 || yy >= MapHeight || w.rng.next()%5 != 0 {
				continue
			}
			pos := xx + yy*MapWidth
			if protected[pos] {
				continue
			}
			w.MapBlk[pos] = RockBlock
			w.MapBk2[pos] = 0
		}
	}
	x1, y1 := x-1, y-1
	x2, y2 := x+8, y+8
	if bounds.changed > 0 {
		if bounds.minX-1 < x1 {
			x1 = bounds.minX - 1
		}
		if bounds.minY-1 < y1 {
			y1 = bounds.minY - 1
		}
		if bounds.maxX > x2 {
			x2 = bounds.maxX
		}
		if bounds.maxY > y2 {
			y2 = bounds.maxY
		}
	}
	w.makeMap(clamp(x1, 0, MapWidth-1), clamp(y1, 0, MapHeight-1), clamp(x2, 0, MapWidth-1), clamp(y2, 0, MapHeight-1))
	return true
}

func (w *World) Flood(player int) bool {
	if !w.spendPowerMana(player, ManaFloodCost, computerFlood) {
		return false
	}
	w.queueSound(TuneFlood)
	if player == w.ScorePlayer {
		w.Score += ScoreFlood
	}
	for pos := range w.Alt {
		if w.Alt[pos] > 0 {
			w.Alt[pos]--
		}
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	return true
}

func (w *World) Knight(player int) bool {
	if player < 0 || player >= len(w.Magnets) {
		return false
	}
	carried := w.Magnets[player].Carried - 1
	if carried < 0 || carried >= len(w.Peeps) || w.Peeps[carried].Population <= 0 {
		return false
	}
	if !w.spendPowerMana(player, ManaKnightCost, computerKnight) {
		return false
	}
	w.queueSound(TuneKnighted)
	if player == w.ScorePlayer {
		w.Score += ScoreKnight
	}
	if w.Peeps[carried].Flags&InTown != 0 {
		w.setTown(carried, true)
	}
	peep := &w.Peeps[carried]
	peep.Flags &^= InTown | WaitForMe | IAmWaiting
	peep.Flags |= OnMove
	peep.Status = KnightStatus
	peep.HeadFor = 0
	if target := w.closestEnemy(carried); target >= 0 {
		peep.HeadFor = target + 1
	}
	peep.Frame = 0
	peep.Direction = 0
	w.Magnets[player].GoTo = peep.AtPos
	w.Magnets[player].Carried = 0
	return true
}

func (w *World) WarPower(player int) bool {
	if !w.spendPowerMana(player, ManaWarCost, computerWar) {
		return false
	}
	w.queueSound(TuneWar)
	if player == w.ScorePlayer {
		w.Score += ScoreWar
	}
	w.War = true
	w.setWarMagnets()
	for i := range w.Peeps {
		w.Peeps[i].HeadFor = 0
		if w.Peeps[i].Population > 0 && w.Peeps[i].Flags&InRuin == 0 {
			w.Peeps[i].Status = KnightStatus
			if w.Peeps[i].Flags == InTown {
				w.setTown(i, true)
				w.Peeps[i].Flags = OnMove
				w.Peeps[i].Frame = 0
				w.Peeps[i].Direction = 0
			}
		}
	}
	return true
}

func (w *World) setWarMagnets() {
	center := MapWidth/2 + (MapHeight/2)*MapWidth
	for i := range w.Magnets {
		w.Magnets[i].GoTo = center
	}
}

func (w *World) spendMana(player, cost int) bool {
	if player < 0 || player >= len(w.Magnets) {
		return false
	}
	if w.Magnets[player].Mana < cost {
		return false
	}
	w.Magnets[player].Mana -= cost
	if w.Magnets[player].Mana < ManaFloor {
		w.Magnets[player].Mana = ManaFloor
	}
	return true
}

func (w *World) spendPowerMana(player, cost, power int) bool {
	if w.War {
		return false
	}
	if !w.powerAllowed(player, power) {
		return false
	}
	return w.spendMana(player, cost)
}

func (w *World) powerAllowed(player, power int) bool {
	if player < 0 || player >= len(w.Computer) {
		return false
	}
	mode := w.Computer[player].Mode
	if mode == 0 {
		return true
	}
	return mode&power != 0
}

func (w *World) protectedPowerPositions() map[int]bool {
	protected := map[int]bool{}
	for _, magnet := range w.Magnets {
		if inMap(magnet.GoTo) {
			protected[magnet.GoTo] = true
		}
		if carried := magnet.Carried - 1; carried >= 0 && carried < len(w.Peeps) && inMap(w.Peeps[carried].AtPos) {
			protected[w.Peeps[carried].AtPos] = true
		}
	}
	return protected
}

func (w *World) PlayerPopulation(player int) int {
	total := 0
	for _, peep := range w.Peeps {
		if peep.Population > 0 && int(peep.Player) == player {
			total += peep.Population
		}
	}
	return total
}

func (w *World) DrainSoundEvents() []int {
	if len(w.SoundEvents) == 0 {
		return nil
	}
	events := append([]int(nil), w.SoundEvents...)
	w.SoundEvents = w.SoundEvents[:0]
	return events
}

func (w *World) queueSound(sound int) {
	if sound <= 0 {
		return
	}
	w.SoundEvents = append(w.SoundEvents, sound)
}

func (w *World) SetScorePlayer(player int) bool {
	if player < 0 || player >= len(w.Magnets) {
		return false
	}
	oldBase := w.initialScoreFor(w.ScorePlayer)
	newBase := w.initialScoreFor(player)
	if w.Score == oldBase {
		w.Score = newBase
	}
	w.ScorePlayer = player
	return true
}

func (w *World) initialScoreFor(player int) int {
	count := int(w.Level.PlayerPopulation)
	if player == DevilPlayer {
		count = int(w.Level.EnemyPopulation)
	}
	if count <= 0 {
		count = 1
	}
	return count * ScorePeople
}

func (w *World) HasBuildPresence(player, xoff, yoff, width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	needTown := w.Level.GameMode&GameRaiseTown != 0
	for y := 0; y < height; y++ {
		mapY := yoff + y
		if mapY < 0 || mapY >= MapHeight {
			continue
		}
		for x := 0; x < width; x++ {
			mapX := xoff + x
			if mapX < 0 || mapX >= MapWidth {
				continue
			}
			index := int(w.MapWho[mapX+mapY*MapWidth]) - 1
			if index < 0 || index >= len(w.Peeps) {
				continue
			}
			peep := w.Peeps[index]
			if peep.Population <= 0 || int(peep.Player) != player {
				continue
			}
			if !needTown || peep.Flags&InTown != 0 {
				return true
			}
		}
	}
	return false
}

func (w *World) ResultFor(player int) int {
	if player < 0 || player >= len(w.Magnets) {
		return ResultOngoing
	}
	opponent := player ^ 1
	if w.PlayerPopulation(player) == 0 {
		return ResultLost
	}
	if w.PlayerPopulation(opponent) == 0 {
		return ResultWon
	}
	return ResultOngoing
}

func (w *World) SummaryFor(player int) PlayerSummary {
	summary := PlayerSummary{BattlesWon: w.BattleWon[player]}
	for _, peep := range w.Peeps {
		if peep.Population <= 0 || int(peep.Player) != player {
			continue
		}
		summary.Population += peep.Population
		if peep.HeadFor != 0 && peep.Flags&InRuin == 0 {
			summary.Knights++
		}
		if peep.Flags == InTown {
			if peep.Frame == LastTown {
				summary.Castles++
			} else {
				summary.Towns++
			}
		}
	}
	return summary
}

func (w *World) EndScore(player int, lost bool) int {
	if player < 0 || player >= len(w.Magnets) {
		return 0
	}
	score := w.Score
	opponent := player ^ 1
	if w.BattleWon[player] > w.BattleWon[opponent] {
		score += ScoreBattle
	}
	for power := computerQuake; power <= computerWar; power <<= 1 {
		if w.Computer[player].Mode&power == 0 {
			score += ScoreYourOptions
		}
		if w.Computer[opponent].Mode&power != 0 {
			score += ScoreHisOptions
		}
	}
	score += (10 - w.Computer[opponent].Speed) * ScoreSpeed
	if !lost {
		score *= ScoreWon
	}
	if score < 500 {
		score = 500
	}
	if score > 555555 {
		score = 515090
	}
	return score
}

func NextConquestLevelIndex(currentIndex, score int) int {
	if currentIndex < 0 {
		currentIndex = 0
	}
	const maxOriginalLevel = 2470
	originalLevel := currentIndex * 5
	nextLevel := originalLevel + score/5000 + 1
	if nextLevel%5 != 0 {
		nextLevel += 5 - nextLevel%5
	}
	if nextLevel > maxOriginalLevel {
		if originalLevel == maxOriginalLevel {
			return 0
		}
		nextLevel = maxOriginalLevel
	}
	return nextLevel / 5
}

func (w *World) NextConquestLevelIndex(score int) int {
	return NextConquestLevelIndex(w.Level.Number, score)
}

func (w *World) wakeMagnetCarrier(player int) {
	carried := w.Magnets[player].Carried - 1
	if carried < 0 || carried >= len(w.Peeps) || w.Peeps[carried].Population <= 0 {
		return
	}
	if w.Peeps[carried].Flags != InTown {
		return
	}
	w.setTown(carried, true)
	w.Peeps[carried].Flags = OnMove
	w.Peeps[carried].Frame = 0
	w.Peeps[carried].Direction = 0
}

func (w *World) changeAltitude(player, x, y int, raise, paint bool) bool {
	if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
		return false
	}
	if w.War {
		return false
	}
	if !paint {
		if player < 0 || player >= len(w.Magnets) {
			return false
		}
		if w.Level.GameMode&GameNoBuild != 0 {
			return false
		}
		if !raise && w.Level.GameMode&GameOnlyRaise != 0 {
			return false
		}
		if w.Magnets[player].Mana < ManaPointCost {
			return false
		}
	}

	bounds := newAltBounds(x, y)
	if raise {
		w.raisePointTracked(x, y, bounds)
	} else {
		w.lowerPointTracked(x, y, bounds)
	}
	if bounds.changed == 0 {
		return false
	}

	if !paint {
		w.Magnets[player].Mana -= bounds.changed*4 + ManaPointCost
		if w.Magnets[player].Mana < ManaFloor {
			w.Magnets[player].Mana = ManaFloor
		}
	}
	w.rebuildAltitudeBounds(bounds)
	return true
}

func (w *World) rebuildAltitudeBounds(bounds *altBounds) {
	x1 := clamp(bounds.minX-1, 0, MapWidth-1)
	y1 := clamp(bounds.minY-1, 0, MapHeight-1)
	x2 := clamp(bounds.maxX, 0, MapWidth-1)
	y2 := clamp(bounds.maxY, 0, MapHeight-1)
	w.makeMap(x1, y1, x2, y2)
}

type altBounds struct {
	minX    int
	minY    int
	maxX    int
	maxY    int
	changed int
}

func newAltBounds(x, y int) *altBounds {
	return &altBounds{minX: x, minY: y, maxX: x, maxY: y}
}

func (b *altBounds) record(x, y int) {
	if x < b.minX {
		b.minX = x
	}
	if y < b.minY {
		b.minY = y
	}
	if x > b.maxX {
		b.maxX = x
	}
	if y > b.maxY {
		b.maxY = y
	}
	b.changed++
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (w *World) makeMap(x1, y1, x2, y2 int) {
	for x := x1; x <= x2; x++ {
		for y := y1; y <= y2; y++ {
			pos := x + y*MapWidth
			altPos := x + EndWidth*y
			avg := (w.Alt[altPos] + w.Alt[altPos+1] + w.Alt[altPos+EndWidth] + w.Alt[altPos+EndWidth+1]) >> 2
			keepRock := int(w.MapBlk[pos]) == RockBlock
			block := 0
			if w.Alt[altPos] > avg {
				block++
			}
			if w.Alt[altPos+1] > avg {
				block += 2
			}
			if w.Alt[altPos+EndWidth+1] > avg {
				block += 4
			}
			if w.Alt[altPos+EndWidth] > avg {
				block += 8
			}
			if keepRock && !(block == 0 && avg == 0) {
				block = RockBlock
			}
			if avg != 0 && block == 0 {
				avg--
				block = FlatBlock
			}
			if avg == 0 && block != FlatBlock && block != WaterBlock {
				block += 16
			}
			w.MapAlt[pos] = byte(avg)
			if keepRock && !(block == WaterBlock && avg == 0) {
				block = RockBlock
			}
			w.MapBlk[pos] = byte(block)
			if block == WaterBlock {
				w.MapBk2[pos] = 0
			}
			w.MapSteps[pos] = 0
		}
	}
}

func (w *World) makeWoodsRocks() {
	for i := 0; i < noWoods; i++ {
		kind := TreeBlock
		if i < noWoods/3 {
			kind = RockBlock
		}
		startX := w.rng.next() % (MapWidth - 5)
		startY := w.rng.next() % (MapHeight - 5)
		for j := 0; j < noTrees; j++ {
			x := startX + w.rng.next()%9
			y := startY + w.rng.next()%9
			if x < 0 || x >= MapWidth || y < 0 || y >= MapHeight {
				continue
			}
			pos := x + y*MapWidth
			if w.MapBlk[pos] == WaterBlock || w.MapBlk[pos] == RockBlock {
				continue
			}
			if kind == RockBlock {
				w.MapBlk[pos] = byte(kind + w.rng.next()%3)
			} else {
				w.MapBk2[pos] = byte(kind + w.rng.next()%3)
			}
		}
	}
}

func (w *World) placeFirstPeople() {
	w.Peeps = w.Peeps[:0]
	for i := range w.MapWho {
		w.MapWho[i] = 0
		w.MapSteps[i] = 0
	}
	center := MapWidth/2 + MapWidth*(MapHeight/2)
	for i := range w.Magnets {
		w.Magnets[i] = Magnet{GoTo: center, Flags: SettleMode, Mana: 399}
	}

	godCount := int(w.Level.PlayerPopulation)
	if godCount <= 0 {
		godCount = 1
	}
	w.Score = w.initialScoreFor(w.ScorePlayer)
	w.placeInitialSide(GodPlayer, godCount, MapWidth*2, MapWidth*MapHeight, 1)

	devilCount := int(w.Level.EnemyPopulation)
	if devilCount <= 0 {
		devilCount = 1
	}
	start := (MapWidth-1)*(MapHeight-1) - 1
	w.placeInitialSide(DevilPlayer, devilCount, start, MapWidth*2-1, -1)
}

func (w *World) placeInitialSide(player, count, start, end, step int) {
	for pos := start; count > 0 && pos != end; pos += step {
		if pos < 0 || pos >= MapWidth*MapHeight {
			continue
		}
		if w.MapBlk[pos] == FlatBlock && w.MapWho[pos] == 0 {
			count--
			w.placePeople(player, pos, count == 0)
		}
	}
	if count == 0 {
		return
	}

	if step > 0 {
		for pos := 0; count > 0 && pos < MapWidth*MapHeight; pos++ {
			if w.MapBlk[pos] != 0 && w.MapWho[pos] == 0 {
				count--
				w.placePeople(player, pos, count == 0)
			}
		}
		return
	}
	for pos := MapWidth*MapHeight - 1; count > 0 && pos >= 0; pos-- {
		if w.MapBlk[pos] != 0 && w.MapWho[pos] == 0 {
			count--
			w.placePeople(player, pos, count == 0)
		}
	}
}

func (w *World) placePeople(player, pos int, leader bool) {
	if len(w.Peeps) >= MaxPeeps || pos < 0 || pos >= MapWidth*MapHeight {
		return
	}
	peep := Peep{
		Flags:      OnMove,
		Player:     byte(player),
		IQ:         1,
		Weapons:    1,
		Population: StartFood - 5,
		AtPos:      pos,
		Frame:      0,
		Direction:  0,
	}
	w.Peeps = append(w.Peeps, peep)
	index := len(w.Peeps)
	w.MapWho[pos] = byte(index)
	if leader {
		w.Magnets[player].Carried = index
		w.Magnets[player].GoTo = pos
	}
}

func (w *World) Tick() {
	computerControlled := w.ComputerControlled
	if !computerControlled[GodPlayer] && !computerControlled[DevilPlayer] {
		computerControlled[DevilPlayer] = true
	}
	w.TickWithComputer(computerControlled)
}

func (w *World) TickWithComputer(computerControlled [2]bool) {
	w.ComputerControlled = computerControlled
	w.GameTurn++
	if w.War {
		w.setWarMagnets()
	}
	w.updateComputerStats()
	for player, controlled := range w.ComputerControlled {
		if controlled {
			w.runComputerPlayer(player)
		}
	}
	for i := range w.Magnets {
		w.Magnets[i].Population = 0
		w.Magnets[i].NoTowns = 0
		if w.GameTurn&1 == 0 {
			w.Magnets[i].Mana++
		}
	}

	initialLen := len(w.Peeps)
	for i := 0; i < initialLen && i < len(w.Peeps); i++ {
		if w.Peeps[i].Population <= 0 {
			continue
		}
		player := int(w.Peeps[i].Player)
		if player < 0 || player >= len(w.Magnets) || !inMap(w.Peeps[i].AtPos) {
			w.zeroPopulation(i)
			continue
		}

		w.Magnets[player].Population += w.Peeps[i].Population
		block := int(w.MapBlk[w.Peeps[i].AtPos])
		if w.Peeps[i].Flags&InWater != 0 {
			if w.Level.GameMode&GameWaterFatal != 0 {
				w.zeroPopulation(i)
				continue
			}
			if w.War {
				w.forceRaiseAt(w.Peeps[i].AtPos%MapWidth, w.Peeps[i].AtPos/MapWidth)
			}
			if block != WaterBlock {
				w.Peeps[i].Flags &^= InWater
				w.setFrame(i)
				continue
			}
			w.setFrame(i)
			w.Peeps[i].Population -= w.walkDeath() << 1
			if w.Peeps[i].Population <= 0 {
				w.zeroPopulation(i)
			}
			continue
		}
		if block == WaterBlock {
			if w.Peeps[i].Flags&InTown != 0 {
				w.setTown(i, true)
			}
			w.Peeps[i].Flags = OnMove | InWater
			if w.Peeps[i].Frame >= 8 {
				w.Peeps[i].Frame = 0
			}
			continue
		}

		switch {
		case w.Peeps[i].Flags&InEffect != 0:
			if w.setFrame(i) {
				w.Peeps[i].Flags &^= InEffect
				w.setFrame(i)
			}
		case w.Peeps[i].Flags&InBattle != 0:
			if w.Peeps[i].Flags == InBattle {
				w.doBattle(i)
			}
		case w.Peeps[i].Flags&InRuin != 0:
			if w.MapWho[w.Peeps[i].AtPos] == 0 {
				w.MapWho[w.Peeps[i].AtPos] = byte(i + 1)
			}
			if w.Peeps[i].BattlePopulation <= 0 {
				w.Peeps[i].Population = 0
			} else {
				w.Peeps[i].BattlePopulation--
			}
		case w.Peeps[i].Flags == InTown:
			w.processTown(i)
		case w.Peeps[i].Flags == OnMove:
			if w.setFrame(i) {
				if int(w.MapBlk[w.Peeps[i].AtPos]) == SwampBlock {
					swampPos := w.Peeps[i].AtPos
					w.queueSound(TuneSwamp)
					w.zeroPopulation(i)
					if w.Level.GameMode&GameSwampRemain == 0 {
						w.MapBlk[swampPos] = FlatBlock
					}
					continue
				}
				if w.ComputerControlled[player] {
					w.computerOneBlockFlat(w.Peeps[i].AtPos, player)
				}
				w.moveExplorer(i)
				if i < len(w.Peeps) && w.Peeps[i].Population > 0 && w.Peeps[i].InOut != 0 {
					if w.Peeps[i].InOut != w.Peeps[i].AtPos-w.Peeps[i].Direction {
						w.Peeps[i].InOut = 0
					}
					w.Peeps[i].Population -= w.walkDeath()
				}
			}
			if i < len(w.Peeps) && w.Peeps[i].Population <= 0 {
				w.zeroPopulation(i)
			}
		case w.Peeps[i].Flags&(WaitForMe|IAmWaiting) != 0:
			w.setFrame(i)
			w.Peeps[i].Population -= w.walkDeath()
			w.Peeps[i].BattlePopulation++
			if w.Peeps[i].BattlePopulation > 14 {
				w.Peeps[i].Flags &^= WaitForMe | IAmWaiting
				w.setFrame(i)
				if w.Peeps[i].Flags == OnMove {
					w.moveExplorer(i)
				}
			}
			if i < len(w.Peeps) && w.Peeps[i].Population <= 0 {
				w.zeroPopulation(i)
			}
		}
	}
	w.resetComputerActionSlots()
}

func (w *World) updateComputerStats() {
	bestLife := [2]int{}
	oldestTown := [2]int{-1, -1}
	youngestTown := [2]int{1 << 30, 1 << 30}
	for player := range w.Computer {
		w.Computer[player].Best1 = -1
		w.Computer[player].Best2 = -1
		w.Computer[player].MyBest = -1
		w.Computer[player].NoTowns = 0
		w.Computer[player].NoCastles = 0
	}

	for index, peep := range w.Peeps {
		if peep.Population <= 0 {
			continue
		}
		player := int(peep.Player)
		if player < 0 || player >= len(w.Computer) {
			continue
		}
		opponent := player ^ 1
		if peep.Flags == OnMove && w.Computer[opponent].Best2 < 0 {
			w.Computer[opponent].Best2 = index
		}
		if peep.Flags != InTown {
			continue
		}
		if peep.Frame == LastTown {
			w.Computer[player].NoCastles++
		} else {
			w.Computer[player].NoTowns++
		}
		life := w.checkLife(player, peep.AtPos)
		if life > bestLife[player] {
			bestLife[player] = life
			w.Computer[opponent].Best1 = index
		}
		age := w.GameTurn - peep.BattlePopulation
		if age >= oldestTown[player] {
			oldestTown[player] = age
			w.Computer[opponent].Best2 = index
		}
		if age < youngestTown[player] {
			youngestTown[player] = age
			w.Computer[player].MyBest = index
		}
	}
}

func (w *World) runComputerPlayer(player int) {
	if !w.computerActionReady(player) {
		return
	}
	if w.computerSetMagnet(player) {
		w.markComputerAction(player)
		return
	}
	if w.computerEffect(player) {
		w.markComputerAction(player)
	}
}

func (w *World) computerActionReady(player int) bool {
	if player < 0 || player >= len(w.Computer) || w.War || !w.ComputerControlled[player] {
		return false
	}
	stats := w.Computer[player]
	return stats.Mode != 0 && stats.Speed > 0 && stats.DoneTurn == 0
}

func (w *World) markComputerAction(player int) {
	if player >= 0 && player < len(w.Computer) {
		w.Computer[player].DoneTurn = w.GameTurn
	}
}

func (w *World) resetComputerActionSlots() {
	for player, controlled := range w.ComputerControlled {
		if !controlled {
			continue
		}
		speed := w.Computer[player].Speed
		if speed > 0 && w.GameTurn%speed == 0 {
			w.Computer[player].DoneTurn = 0
		}
	}
}

func (w *World) computerSetMagnet(player int) bool {
	stats := &w.Computer[player]
	if stats.Mode == 0 {
		return false
	}
	if stats.NoTowns+stats.NoCastles < stats.Skill*2+15 || w.GameTurn%90 < 10+stats.Skill {
		if w.Magnets[player].Flags == MagnetMode {
			return w.SetMagnetMode(player, SettleMode+w.rng.next()%3)
		}
		return false
	}

	imp := w.carriedPeepIndex(player)
	pope := w.carriedPeepIndex(player ^ 1)
	if imp < 0 {
		if w.Magnets[player].Flags != MagnetMode {
			stats.Arrived = 0
			return w.SetMagnetMode(player, MagnetMode)
		}
		return false
	}

	if w.Peeps[imp].AtPos == stats.LastBattle && stats.Arrived > 0 {
		stats.Arrived--
	}
	if w.Peeps[imp].Population < DevilMakesKnight*2 {
		if stats.Arrived > 0 {
			if inMap(stats.LastBattle) && w.Magnets[player].GoTo != stats.LastBattle {
				return w.SetMagnetTo(player, stats.LastBattle)
			}
		} else if w.validPeep(stats.MyBest) && w.Magnets[player].GoTo != w.Peeps[stats.MyBest].AtPos {
			stats.LastBattle = w.Peeps[stats.MyBest].AtPos
			stats.Arrived = 2
			return w.SetMagnetTo(player, stats.LastBattle)
		}
		if w.Magnets[player].Flags != MagnetMode {
			return w.SetMagnetMode(player, MagnetMode)
		}
		return false
	}

	if pope >= 0 && w.Peeps[imp].Population > w.Peeps[pope].Population+500 && w.Magnets[player^1].Flags == MagnetMode {
		if stats.Mode&computerLeader != 0 {
			if w.Magnets[player].Flags != MagnetMode {
				return w.SetMagnetMode(player, MagnetMode)
			}
			if w.Magnets[player].GoTo != w.Magnets[player^1].GoTo {
				return w.SetMagnetTo(player, w.Magnets[player^1].GoTo)
			}
		}
		return false
	}

	if stats.Mode&computerTown != 0 && w.validPeep(stats.Best1) {
		if stats.Arrived == 0 && w.Magnets[player].GoTo != w.Peeps[stats.Best1].AtPos {
			stats.LastBattle = w.Peeps[stats.Best1].AtPos
			stats.Arrived = 2
			return w.SetMagnetTo(player, stats.LastBattle)
		}
		if w.Magnets[player].Flags != MagnetMode {
			return w.SetMagnetMode(player, MagnetMode)
		}
	}
	return false
}

func (w *World) computerEffect(player int) bool {
	stats := &w.Computer[player]
	mode := stats.Mode
	if mode&computerWar != 0 && w.Magnets[player].Mana > ManaWarCost+999 && w.PlayerPopulation(player) > w.PlayerPopulation(player^1) {
		return w.WarPower(player)
	}
	if mode&computerFlood != 0 && w.Magnets[player].Mana > ManaFloodCost+1999 {
		return w.Flood(player)
	}
	carried := w.carriedPeepIndex(player)
	if mode&computerKnight != 0 && carried >= 0 && w.Peeps[carried].Population > DevilMakesKnight && w.Magnets[player].Mana > ManaKnightCost+500 {
		return w.Knight(player)
	}
	if !w.validPeep(stats.Best2) {
		return false
	}
	if mode&computerVolcano != 0 && w.Magnets[player].Mana > ManaVolcanoCost+500 {
		x, y := w.computerPowerTarget(player, stats.Best2)
		if w.VolcanoAt(player, x, y) {
			stats.QuakeCount = 0
			return true
		}
	}
	if mode&computerSwamp != 0 && w.Magnets[player].Mana > ManaSwampCost+500 {
		if (stats.QuakeCount >= stats.NoQuakes || mode&computerQuake == 0) && (stats.QuakeCount <= stats.NoSwamps || mode&computerVolcano == 0) {
			enemyCarrier := w.carriedPeepIndex(player ^ 1)
			if enemyCarrier >= 0 && (carried < 0 || w.Peeps[enemyCarrier].Population > w.Peeps[carried].Population) {
				if w.SwampAt(player, w.Peeps[enemyCarrier].AtPos%MapWidth, w.Peeps[enemyCarrier].AtPos/MapWidth) {
					stats.QuakeCount++
					return true
				}
			}
		}
	}
	if mode&computerQuake != 0 && w.Magnets[player].Mana > ManaQuakeCost+500 && w.Peeps[stats.Best2].Flags == InTown {
		if stats.QuakeCount < stats.NoQuakes || mode&(computerSwamp|computerVolcano) == 0 {
			x, y := w.computerPowerTarget(player, stats.Best2)
			if w.QuakeAt(player, x, y) {
				stats.QuakeCount++
				return true
			}
		}
	}
	return false
}

func (w *World) computerPowerTarget(player, fallbackPeep int) (int, int) {
	if w.Magnets[player].Carried == 0 && inMap(w.Magnets[player].GoTo) {
		if target := int(w.MapWho[w.Magnets[player].GoTo]) - 1; w.validPeep(target) && int(w.Peeps[target].Player) != player {
			return w.Magnets[player].GoTo % MapWidth, w.Magnets[player].GoTo / MapWidth
		}
	}
	x := w.Peeps[fallbackPeep].AtPos%MapWidth - 3
	y := w.Peeps[fallbackPeep].AtPos/MapWidth - 3
	return clamp(x, 0, MapWidth-1), clamp(y, 0, MapHeight-1)
}

func (w *World) carriedPeepIndex(player int) int {
	if player < 0 || player >= len(w.Magnets) {
		return -1
	}
	index := w.Magnets[player].Carried - 1
	if !w.validPeep(index) {
		return -1
	}
	return index
}

func (w *World) validPeep(index int) bool {
	return index >= 0 && index < len(w.Peeps) && w.Peeps[index].Population > 0 && inMap(w.Peeps[index].AtPos)
}

func (w *World) computerMakeLevel(pos, player int) bool {
	if !w.computerActionReady(player) || w.Computer[player].Mode&computerLand == 0 || w.Level.GameMode&GameNoBuild != 0 || !inMap(pos) {
		return false
	}
	x := pos % MapWidth
	y := pos / MapWidth
	thisAlt := w.Alt[x+y*EndWidth]
	for _, delta := range computerFlatSearch {
		xx := x + delta[0]
		yy := y + delta[1]
		if xx < 0 || xx >= MapWidth || yy < 0 || yy >= MapHeight {
			continue
		}
		target := xx + yy*MapWidth
		if int(w.MapBlk[target]) == RockBlock && w.Level.GameMode&GameOnlyRaise == 0 {
			w.MapBlk[target]++
			w.LowerAt(player, xx, yy)
			w.markComputerAction(player)
			return false
		}
		diff := thisAlt - w.Alt[xx+yy*EndWidth]
		if diff > 0 {
			w.RaiseAt(player, xx, yy)
			w.markComputerAction(player)
			return false
		}
		if (diff < 0 || int(w.MapBlk[target]) == BadLand || int(w.MapBlk[target]) == SwampBlock) && w.Level.GameMode&GameOnlyRaise == 0 {
			w.LowerAt(player, xx, yy)
			w.markComputerAction(player)
			return false
		}
	}
	return true
}

func (w *World) computerOneBlockFlat(pos, player int) bool {
	if !w.computerActionReady(player) || w.Computer[player].Mode&computerLand == 0 || w.Level.GameMode&GameNoBuild != 0 || !inMap(pos) {
		return false
	}
	if w.Magnets[player].Mana < 20 || w.Magnets[player].NoTowns > 50 {
		return false
	}
	x := pos % MapWidth
	y := pos / MapWidth
	altSum := w.Alt[x+y*EndWidth] + w.Alt[x+1+y*EndWidth] + w.Alt[x+1+(y+1)*EndWidth] + w.Alt[x+(y+1)*EndWidth]
	if altSum == 1 {
		return false
	}
	mod := altSum % 4
	avg := altSum / 4
	for xx := x; xx <= x+1; xx++ {
		for yy := y; yy <= y+1; yy++ {
			altPos := xx + yy*EndWidth
			if mod == 3 {
				if w.Alt[altPos] == avg {
					w.RaiseAt(player, xx, yy)
					w.markComputerAction(player)
					return true
				}
			} else if mod == 1 && w.Level.GameMode&GameOnlyRaise == 0 {
				if w.Alt[altPos] > avg {
					w.LowerAt(player, xx, yy)
					w.markComputerAction(player)
					return true
				}
			}
		}
	}
	return false
}

func (w *World) processTown(index int) {
	if index < 0 || index >= len(w.Peeps) || w.Peeps[index].Population <= 0 {
		return
	}
	peep := &w.Peeps[index]
	player := int(peep.Player)
	if w.War || peep.HeadFor != 0 {
		w.setTown(index, true)
		peep.Flags = OnMove
		peep.Frame = 0
		peep.Direction = 0
		return
	}
	life := w.checkLife(player, peep.AtPos)
	if life <= 0 {
		w.setTown(index, true)
		peep = &w.Peeps[index]
		peep.Flags = OnMove
		peep.Frame = 0
		peep.Direction = 0
		return
	}

	oldFrame := peep.Frame
	if life >= CityFood {
		peep.Frame = LastTown
	} else {
		peep.Frame = FirstTown + (life*10)/MaxFood
		w.Magnets[player].NoTowns++
	}
	if w.MapWho[peep.AtPos] == 0 {
		w.MapWho[peep.AtPos] = byte(index + 1)
	}

	if w.ComputerControlled[player] && w.computerActionReady(player) {
		if int(w.MapAlt[peep.AtPos]) == 0 {
			if !peep.LandComplete || oldFrame != peep.Frame || w.townHasFlatFootprint(peep.AtPos) {
				peep.LandComplete = w.computerMakeLevel(peep.AtPos, player)
			}
		} else if w.Computer[player].NoTowns+w.Computer[player].NoCastles*3 < 3 && w.GameTurn > 250 {
			peep.LandComplete = w.computerMakeLevel(peep.AtPos, player)
		}
	}

	stage := peep.Frame - FirstTown
	if stage < 0 {
		stage = 0
	}
	if stage >= len(w.Rules.PopulationAdd) {
		stage = len(w.Rules.PopulationAdd) - 1
	}
	if w.GameTurn&7 == 0 {
		w.Magnets[player].Mana += w.Rules.ManaAdd[stage]
		peep.Weapons = w.Rules.WeaponsAdd[stage]
		if peep.Population > life {
			w.spawnWalkerFromTown(index, life)
			peep = &w.Peeps[index]
		}
		if peep.Population > 0 {
			peep.Population += w.Rules.PopulationAdd[stage]
		}
	}
	needsTownUpdate := oldFrame != peep.Frame || int(w.MapBk2[peep.AtPos]) != peep.Frame || w.townHasFlatFootprint(peep.AtPos)
	if peep.Frame == LastTown && w.cityPieceCount(peep.AtPos) < 9 {
		needsTownUpdate = true
	}
	if needsTownUpdate {
		w.setTown(index, false)
	}
}

func (w *World) spawnWalkerFromTown(index, life int) {
	if index < 0 || index >= len(w.Peeps) || len(w.Peeps) >= MaxPeeps {
		return
	}
	town := &w.Peeps[index]
	if life <= 0 || town.Population <= life {
		return
	}
	walkerPopulation := town.Population - (life >> 1)
	town.Population = life >> 1
	walker := Peep{
		Flags:      OnMove,
		Player:     town.Player,
		IQ:         town.IQ,
		Weapons:    town.Weapons,
		Population: walkerPopulation,
		AtPos:      town.AtPos,
		InOut:      town.AtPos,
	}
	if town.IQ < MaxMensa {
		town.IQ++
	}
	w.Peeps = append(w.Peeps, walker)
	newIndex := len(w.Peeps)
	if w.MapWho[walker.AtPos] == 0 || w.MapWho[walker.AtPos] == byte(index+1) {
		w.MapWho[walker.AtPos] = byte(newIndex)
	}
	if w.Magnets[int(town.Player)].Carried == index+1 {
		w.Magnets[int(town.Player)].Carried = newIndex
	}
}

func (w *World) setFrame(index int) bool {
	if index < 0 || index >= len(w.Peeps) || w.Peeps[index].Population <= 0 {
		return false
	}
	peep := &w.Peeps[index]
	switch {
	case peep.Flags&InEffect != 0:
		peep.Frame++
		if peep.Frame >= LastVictorySprite || peep.Frame < VictorySprite {
			return true
		}
	case peep.Flags&InBattle != 0:
		peep.Frame++
		if peep.Frame >= BattleLastFrame || peep.Frame < BattleFirstFrame {
			peep.Frame = BattleFirstFrame
			return true
		}
	case peep.Flags == OnMove:
		peep.Frame++
		if peep.Frame >= 7 {
			peep.Frame = 0
			return true
		}
	case peep.Flags&InWater != 0:
		peep.Frame++
		if peep.Frame > LastWaterSprite || peep.Frame < FirstWaterSprite {
			peep.Frame = FirstWaterSprite
			return true
		}
	case peep.Flags == InTown:
		life := w.checkLife(int(peep.Player), peep.AtPos)
		if life >= CityFood {
			peep.Frame = LastTown
		} else {
			peep.Frame = FirstTown + (life*10)/MaxFood
		}
	case peep.Flags&(IAmWaiting|WaitForMe) != 0:
		peep.Frame++
		if peep.Frame > LastWaitSprite || peep.Frame < FirstWaitSprite {
			peep.Frame = FirstWaitSprite
			return true
		}
	}
	return false
}

func (w *World) moveExplorer(index int) {
	if index < 0 || index >= len(w.Peeps) || w.Peeps[index].Population <= 0 {
		return
	}
	player := int(w.Peeps[index].Player)
	goTo := noMove
	if w.War {
		goTo = w.moveMagnetPeeps(index)
	} else if isHeadedPeep(w.Peeps[index]) {
		goTo = w.moveKnightPeep(index)
	} else if player >= 0 && player < len(w.Magnets) && w.Magnets[player].Flags == MagnetMode {
		goTo = w.moveMagnetPeeps(index)
	} else {
		goTo = w.whereDoIGo(index)
	}
	peep := &w.Peeps[index]

	id := byte(index + 1)
	oldSource := peep.AtPos - peep.Direction
	if inMap(oldSource) && w.MapWho[oldSource] == id {
		w.MapWho[oldSource] = 0
	}
	if !inMap(peep.AtPos) {
		w.zeroPopulation(index)
		return
	}

	if occupant := int(w.MapWho[peep.AtPos]) - 1; occupant >= 0 && occupant != index {
		if !w.resolveContact(index, occupant) {
			return
		}
		peep = &w.Peeps[index]
	}
	if peep.Population <= 0 {
		return
	}

	if goTo == noMove {
		if w.MapWho[peep.AtPos] == 0 {
			w.MapWho[peep.AtPos] = id
		}
		peep.Flags |= IAmWaiting
		peep.BattlePopulation = 7
		return
	}
	peep.Flags &^= IAmWaiting

	if goTo == 0 {
		if isHeadedPeep(*peep) || w.War {
			if w.MapWho[peep.AtPos] == 0 {
				w.MapWho[peep.AtPos] = id
			}
			peep.Flags |= IAmWaiting
			peep.BattlePopulation = 7
			return
		}
		if w.MapWho[peep.AtPos] == 0 {
			w.MapWho[peep.AtPos] = id
		}
		peep.Flags = InTown
		peep.BattlePopulation = w.GameTurn
		peep.Frame = 0
		peep.Direction = 0
		w.setFrame(index)
		w.setTown(index, false)
		return
	}

	if move := w.validMove(peep.AtPos, goTo); move != 0 && !(w.War && move == 2) {
		peep.Flags |= IAmWaiting
		peep.BattlePopulation = 7
		return
	}
	if w.MapSteps[peep.AtPos] < ^uint16(0) {
		w.MapSteps[peep.AtPos]++
	}
	if w.MapWho[peep.AtPos] == 0 {
		w.MapWho[peep.AtPos] = id
	}
	peep.AtPos += goTo
	peep.Direction = goTo
}

func (w *World) resolveContact(moverIndex, foundIndex int) bool {
	if moverIndex < 0 || moverIndex >= len(w.Peeps) || foundIndex < 0 || foundIndex >= len(w.Peeps) {
		return true
	}
	mover := &w.Peeps[moverIndex]
	found := &w.Peeps[foundIndex]
	if found.Population <= 0 {
		w.zeroPopulation(foundIndex)
		return true
	}
	if found.Flags&InRuin != 0 {
		return true
	}
	if found.Flags&InBattle != 0 {
		w.joinBattle(moverIndex, foundIndex)
		return false
	}
	if mover.Player == found.Player {
		w.joinForces(moverIndex, foundIndex)
		return false
	}

	w.setBattle(moverIndex, foundIndex)
	return false
}

func (w *World) setBattle(attackerIndex, defenderIndex int) {
	if attackerIndex < 0 || attackerIndex >= len(w.Peeps) || defenderIndex < 0 || defenderIndex >= len(w.Peeps) || attackerIndex == defenderIndex {
		return
	}
	attacker := &w.Peeps[attackerIndex]
	defender := &w.Peeps[defenderIndex]
	if attacker.Population <= 0 || defender.Population <= 0 {
		return
	}

	attacker.Flags = InBattle
	attacker.BattlePopulation = defenderIndex
	defender.Flags = InBattle | (defender.Flags & (InTown | OnMove | InEffect))
	defender.BattlePopulation = attackerIndex
	w.queueSound(TuneSword1 + (w.GameTurn & 1))
	w.setFrame(attackerIndex)
	w.setFrame(defenderIndex)

	w.clearPeepMapRefs(defenderIndex)
	if defender.Flags&InTown != 0 {
		attacker.AtPos = defender.AtPos
		attacker.Direction = 0
		w.MapWho[defender.AtPos] = byte(attackerIndex + 1)
		return
	}
	defender.AtPos = attacker.AtPos
	defender.Direction = 0
	if inMap(attacker.AtPos) {
		w.MapWho[attacker.AtPos] = byte(attackerIndex + 1)
	}
}

func (w *World) joinBattle(joinerIndex, battleIndex int) {
	if joinerIndex < 0 || joinerIndex >= len(w.Peeps) || battleIndex < 0 || battleIndex >= len(w.Peeps) || joinerIndex == battleIndex {
		return
	}
	joiner := w.Peeps[joinerIndex]
	battle := w.Peeps[battleIndex]
	targetIndex := battleIndex
	if joiner.Player != battle.Player {
		targetIndex = battle.BattlePopulation
	}
	if targetIndex < 0 || targetIndex >= len(w.Peeps) || targetIndex == joinerIndex {
		return
	}
	w.joinForces(joinerIndex, targetIndex)
}

func (w *World) joinForces(sourceIndex, targetIndex int) {
	if sourceIndex < 0 || sourceIndex >= len(w.Peeps) || targetIndex < 0 || targetIndex >= len(w.Peeps) || sourceIndex == targetIndex {
		return
	}
	source := &w.Peeps[sourceIndex]
	target := &w.Peeps[targetIndex]
	if source.Population <= 0 || target.Population <= 0 || source.Player != target.Player {
		return
	}
	sourceWasKnight := source.HeadFor != 0
	sourceHeadFor := source.HeadFor
	if sourceWasKnight && target.Flags&InTown != 0 {
		return
	}
	if source.Flags&InTown != 0 {
		w.setTown(sourceIndex, true)
	}

	sourcePopulation := source.Population
	if target.Population > 32000-sourcePopulation {
		target.Population = 32000
	} else {
		target.Population += sourcePopulation
	}
	if target.IQ < source.IQ {
		target.IQ = source.IQ
	}
	if target.Weapons < source.Weapons {
		target.Weapons = source.Weapons
	}
	if sourceWasKnight {
		target.Status = KnightStatus
		target.HeadFor = sourceHeadFor
	}
	if player := int(source.Player); player >= 0 && player < len(w.Magnets) && w.Magnets[player].Carried == sourceIndex+1 {
		w.Magnets[player].Carried = targetIndex + 1
	}

	w.clearPeepMapRefs(sourceIndex)
	source.Population = 0
	source.Flags = 0
	source.Frame = 0
	source.Direction = 0
	source.BattlePopulation = 0
	source.HeadFor = 0
	source.Status = 0
	target.Flags &^= WaitForMe | IAmWaiting
	if target.Flags == 0 {
		target.Flags = OnMove
	}
	if target.Flags&InBattle == 0 {
		target.Frame = 0
	}
}

func (w *World) doBattle(index int) {
	if index < 0 || index >= len(w.Peeps) || w.Peeps[index].Population <= 0 {
		return
	}
	opponentIndex := w.Peeps[index].BattlePopulation
	if opponentIndex < 0 || opponentIndex >= len(w.Peeps) || opponentIndex == index || w.Peeps[opponentIndex].Population <= 0 {
		w.Peeps[index].Flags &^= InBattle
		if w.Peeps[index].Flags == 0 {
			w.Peeps[index].Flags = OnMove
		}
		return
	}

	peepPower := w.Peeps[index].Population * (w.rng.next()%3 + 1)
	opponentPower := w.Peeps[opponentIndex].Population * (w.rng.next()%3 + 1)
	if opponentPower > peepPower {
		w.Peeps[opponentIndex].Population -= (peepPower/100)*w.Peeps[index].Weapons + 10
		w.Peeps[index].Population -= (peepPower/100)*w.Peeps[opponentIndex].Weapons + 10
	} else {
		w.Peeps[index].Population -= (opponentPower/100)*w.Peeps[opponentIndex].Weapons + 10
		w.Peeps[opponentIndex].Population -= (opponentPower/100)*w.Peeps[index].Weapons + 10
	}
	w.setFrame(index)
	w.setFrame(opponentIndex)

	switch {
	case w.Peeps[index].Population <= 0 && w.Peeps[opponentIndex].Population <= 0:
		w.zeroPopulation(opponentIndex)
		w.zeroPopulation(index)
	case w.Peeps[index].Population <= 0:
		w.battleOver(opponentIndex, index)
	case w.Peeps[opponentIndex].Population <= 0:
		w.battleOver(index, opponentIndex)
	}
}

func (w *World) battleOver(winnerIndex, loserIndex int) {
	if winnerIndex < 0 || winnerIndex >= len(w.Peeps) || loserIndex < 0 || loserIndex >= len(w.Peeps) {
		return
	}
	winner := w.Peeps[winnerIndex]
	loser := w.Peeps[loserIndex]
	reward := w.battleReward(loserIndex)
	winnerWasTown := winner.Flags&InTown != 0
	loserWasTown := loser.Flags&InTown != 0
	knightRazedTown := winner.HeadFor != 0 && loserWasTown

	if knightRazedTown {
		w.queueSound(TuneFire)
		w.razeTownToRuin(loserIndex)
	} else {
		w.zeroPopulation(loserIndex)
	}
	if w.Peeps[winnerIndex].Population <= 0 {
		return
	}

	winnerPtr := &w.Peeps[winnerIndex]
	winnerPtr.Flags &^= InBattle | WaitForMe | IAmWaiting
	winnerPtr.Direction = 0
	if knightRazedTown {
		winnerPtr.Flags = OnMove
	} else if loserWasTown && w.checkLife(int(winnerPtr.Player), winnerPtr.AtPos) > 0 {
		winnerPtr.Flags = InTown
		winnerPtr.BattlePopulation = w.GameTurn
		winnerPtr.Frame = 0
		w.setFrame(winnerIndex)
		w.setTown(winnerIndex, false)
	} else if winnerWasTown {
		winnerPtr.Flags = InTown
	} else {
		winnerPtr.Flags = OnMove
	}
	winnerPtr.Flags |= InEffect
	winnerPtr.Frame = VictorySprite
	if inMap(winnerPtr.AtPos) {
		w.MapWho[winnerPtr.AtPos] = byte(winnerIndex + 1)
	}

	winnerPlayer := int(winnerPtr.Player)
	loserPlayer := int(loser.Player)
	if winnerPlayer >= 0 && winnerPlayer < len(w.Magnets) {
		w.BattleWon[winnerPlayer]++
		w.Magnets[winnerPlayer].Mana += reward
	}
	if loserPlayer >= 0 && loserPlayer < len(w.Magnets) {
		w.Magnets[loserPlayer].Mana -= reward
		if w.Magnets[loserPlayer].Mana < ManaFloor {
			w.Magnets[loserPlayer].Mana = ManaFloor
		}
	}
}

func (w *World) razeTownToRuin(index int) {
	if index < 0 || index >= len(w.Peeps) {
		return
	}
	ruin := &w.Peeps[index]
	if !inMap(ruin.AtPos) {
		return
	}
	ruin.Flags = InRuin
	ruin.Population = 1
	ruin.BattlePopulation = 40
	ruin.Direction = 0
	ruin.HeadFor = 0
	ruin.Status = 0

	const ruinDelta = FirstRuinTown - FirstTown - 1
	if int(w.MapBk2[ruin.AtPos]) == CityCentre {
		for i := 0; i < 25; i++ {
			if w.validMove(ruin.AtPos, offsetVector[i]) == 1 {
				continue
			}
			pos := ruin.AtPos + offsetVector[i]
			if i < 9 && i != 0 {
				overlay := int(w.MapBk2[pos])
				if overlay >= CityTower && overlay <= CityWall2 {
					w.MapBk2[pos] = byte(overlay + ruinDelta)
				}
			}
			if w.MapBlk[pos] == byte(FarmBlock+int(ruin.Player)) {
				w.MapBlk[pos] = BadLand
			}
		}
	} else {
		for i := 0; i < 17; i++ {
			if w.validMove(ruin.AtPos, offsetVector[i]) == 1 {
				continue
			}
			pos := ruin.AtPos + offsetVector[i]
			if w.MapBlk[pos] == byte(FarmBlock+int(ruin.Player)) {
				w.MapBlk[pos] = BadLand
			}
		}
	}
	if overlay := int(w.MapBk2[ruin.AtPos]); overlay >= FirstTown && overlay <= CityCentre {
		w.MapBk2[ruin.AtPos] = byte(overlay + ruinDelta)
	}
	if w.MapWho[ruin.AtPos] == byte(index+1) {
		w.MapWho[ruin.AtPos] = 0
	}
}

func (w *World) scorchTown(town Peep) {
	if !inMap(town.AtPos) {
		return
	}
	limit := 17
	if int(w.MapBk2[town.AtPos]) == CityCentre || town.Frame == LastTown {
		limit = 25
	}
	for i := 0; i < limit; i++ {
		if w.validMove(town.AtPos, offsetVector[i]) == 1 {
			continue
		}
		pos := town.AtPos + offsetVector[i]
		if i < 9 {
			w.MapBk2[pos] = 0
		}
		if int(w.MapBlk[pos]) == FlatBlock || int(w.MapBlk[pos]) == FarmBlock+int(town.Player) {
			w.MapBlk[pos] = BadLand
		}
	}
}

func (w *World) battleReward(loserIndex int) int {
	if loserIndex < 0 || loserIndex >= len(w.Peeps) {
		return 0
	}
	loser := w.Peeps[loserIndex]
	if loser.Flags&InTown != 0 {
		stage := loser.Frame - FirstTown
		if inMap(loser.AtPos) {
			overlay := int(w.MapBk2[loser.AtPos])
			if overlay >= FirstTown && overlay <= CityCentre {
				stage = overlay - FirstTown
			}
		}
		if stage < 0 {
			stage = 0
		}
		if stage >= len(w.Rules.BattleAdd1) {
			stage = len(w.Rules.BattleAdd1) - 1
		}
		return w.Rules.BattleAdd1[stage]
	}
	if loser.HeadFor != 0 {
		return w.Rules.BattleAdd2[1]
	}
	player := int(loser.Player)
	if player >= 0 && player < len(w.Magnets) && w.Magnets[player].Carried == loserIndex+1 {
		return w.Rules.BattleAdd2[2]
	}
	return w.Rules.BattleAdd2[0]
}

func (w *World) clearPeepMapRefs(index int) {
	if index < 0 || index >= len(w.Peeps) {
		return
	}
	id := byte(index + 1)
	peep := w.Peeps[index]
	for _, pos := range [...]int{peep.AtPos, peep.AtPos - peep.Direction} {
		if inMap(pos) && w.MapWho[pos] == id {
			w.MapWho[pos] = 0
		}
	}
}

func (w *World) moveKnightPeep(index int) int {
	if index < 0 || index >= len(w.Peeps) || w.Peeps[index].Population <= 0 {
		return noMove
	}
	targetIndex := w.Peeps[index].HeadFor - 1
	if targetIndex < 0 || targetIndex >= len(w.Peeps) || !w.validKnightTarget(index, targetIndex) {
		targetIndex = w.closestEnemy(index)
		if targetIndex < 0 {
			w.Peeps[index].HeadFor = 0
			return noMove
		}
		w.Peeps[index].HeadFor = targetIndex + 1
	}
	if w.Peeps[targetIndex].AtPos == w.Peeps[index].AtPos {
		return 0
	}
	return w.moveToward(index, w.Peeps[targetIndex].AtPos, true)
}

func (w *World) closestEnemy(index int) int {
	if index < 0 || index >= len(w.Peeps) || !inMap(w.Peeps[index].AtPos) {
		return -1
	}
	peep := w.Peeps[index]
	bestIndex := -1
	bestDistance := 1 << 30
	px := peep.AtPos & (MapWidth - 1)
	py := peep.AtPos / MapWidth
	for i, candidate := range w.Peeps {
		if i == index || !w.validKnightTarget(index, i) {
			continue
		}
		distance := abs(px-(candidate.AtPos&(MapWidth-1))) + abs(py-candidate.AtPos/MapWidth)
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = i
		}
	}
	return bestIndex
}

func (w *World) validKnightTarget(index, target int) bool {
	if index < 0 || index >= len(w.Peeps) || target < 0 || target >= len(w.Peeps) {
		return false
	}
	peep := w.Peeps[index]
	candidate := w.Peeps[target]
	return candidate.Population > 0 && candidate.Player != peep.Player && candidate.Flags&InRuin == 0 && inMap(candidate.AtPos)
}

func isHeadedPeep(peep Peep) bool {
	return peep.Status == KnightStatus || peep.HeadFor != 0
}

func (w *World) moveMagnetPeeps(index int) int {
	if index < 0 || index >= len(w.Peeps) {
		return noMove
	}
	peep := &w.Peeps[index]
	who := int(peep.Player)
	if who < 0 || who >= len(w.Magnets) {
		return noMove
	}

	carried := w.Magnets[who].Carried - 1
	if carried >= 0 && (carried >= len(w.Peeps) || w.Peeps[carried].Population <= 0) {
		w.Magnets[who].Carried = 0
		carried = -1
	}

	target := w.Magnets[who].GoTo
	if w.Magnets[who].Carried == 0 {
		if peep.AtPos == target {
			w.Magnets[who].Carried = index + 1
			return noMove
		}
	} else if carried != index {
		target = w.Peeps[carried].AtPos
	}
	if !inMap(target) {
		return noMove
	}
	return w.moveToward(index, target, true)
}

func (w *World) moveToward(index, target int, avoidSwamp bool) int {
	if index < 0 || index >= len(w.Peeps) || !inMap(target) {
		return noMove
	}
	peep := &w.Peeps[index]
	dx := sign((target & (MapWidth - 1)) - (peep.AtPos & (MapWidth - 1)))
	dy := sign((target / MapWidth) - (peep.AtPos / MapWidth))
	if dx == 0 && dy == 0 {
		return noMove
	}

	pos := toDelta[(dx+1)*3+dy+1]
	if delta := toOffset[pos]; w.canMoveToward(peep, delta, avoidSwamp, true) {
		peep.MagnetLastMove = delta
		return delta
	}

	for dir, tries := pos-1, 0; tries < len(toOffset); dir, tries = dir+1, tries+1 {
		if dir < 0 {
			dir = len(toOffset) - 1
		}
		if dir >= len(toOffset) {
			dir = 0
		}
		delta := toOffset[dir]
		if delta == peep.MagnetLastMove || !w.canMoveToward(peep, delta, avoidSwamp, false) {
			continue
		}
		peep.MagnetLastMove = toOffset[opposite[dir]]
		return delta
	}
	return noMove
}

func (w *World) canMoveToward(peep *Peep, delta int, avoidSwamp, allowWarSpecial bool) bool {
	move := w.validMove(peep.AtPos, delta)
	if move == 0 {
		return !avoidSwamp || int(w.MapBlk[peep.AtPos+delta]) != SwampBlock
	}
	if !w.War || !allowWarSpecial {
		return false
	}
	if move == 2 {
		return true
	}
	if move == 3 {
		w.forceRaiseAt(peep.AtPos%MapWidth, peep.AtPos/MapWidth)
	}
	return false
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (w *World) whereDoIGo(index int) int {
	if index < 0 || index >= len(w.Peeps) {
		return noMove
	}
	peep := w.Peeps[index]
	iq := peep.IQ
	if iq <= 0 {
		iq = 1
	}
	if iq > MaxMensa {
		iq = MaxMensa
	}

	best := [moveToEmpty + 1]int{}
	moveOffset := [moveToEmpty + 1]int{}
	for i := range best {
		best[i] = MaxMensa + 1
	}

	footsteps := 9999
	for c1, c4 := -1, 0; c4 != 9; c4++ {
		if c1 == 0 && c4 == 1 {
			c1 = (w.rng.next() & 7) + 1
		} else {
			c1++
		}
		if c1 == 9 {
			c1 = 0
		}

		offset := peep.AtPos
		for c2 := 0; c2 != iq; c2++ {
			if w.validMove(offset, offsetVector[c1]) != 0 {
				break
			}
			offset += offsetVector[c1]

			if int(w.MapBlk[offset]) == FlatBlock && c2 < best[moveToFlat] {
				if c1 == 0 {
					if w.checkLife(int(peep.Player), offset) > 0 {
						best[moveToFlat] = c2
						moveOffset[moveToFlat] = 0
						continue
					}
				} else if !w.nearExistingTown(offset) {
					best[moveToFlat] = c2
					moveOffset[moveToFlat] = offsetVector[c1]
					continue
				}
			}

			if c1 == 0 {
				continue
			}
			if foundIndex := int(w.MapWho[offset]) - 1; foundIndex >= 0 && foundIndex != index && foundIndex < len(w.Peeps) {
				found := w.Peeps[foundIndex]
				if found.Flags&InBattle != 0 && c2 < best[moveToBattle] {
					best[moveToBattle] = c2
					moveOffset[moveToBattle] = offsetVector[c1]
					continue
				}
				if found.Player != peep.Player && c2 < best[moveToEnemy] {
					best[moveToEnemy] = c2
					moveOffset[moveToEnemy] = offsetVector[c1]
					continue
				}
				if found.Flags&OnMove != 0 && c2 < best[moveToFriend] {
					best[moveToFriend] = c2
					moveOffset[moveToFriend] = offsetVector[c1]
					continue
				}
			}
			if offsetVector[c1] != peep.Direction {
				steps := int(w.MapSteps[offset])
				if steps < footsteps || (steps == footsteps && c2 < best[moveToEmpty]) {
					footsteps = steps
					best[moveToEmpty] = c2
					moveOffset[moveToEmpty] = offsetVector[c1]
				}
			}
		}
	}

	player := int(peep.Player)
	if player >= 0 && player < len(w.Magnets) {
		if w.Magnets[player].Flags == FightMode && best[moveToEnemy] != MaxMensa+1 {
			return moveOffset[moveToEnemy]
		}
		if w.Magnets[player].Flags == JoinMode && best[moveToFriend] != MaxMensa+1 {
			return moveOffset[moveToFriend]
		}
	}
	for i := range best {
		if best[i] != MaxMensa+1 {
			return moveOffset[i]
		}
	}
	return noMove
}

func (w *World) nearExistingTown(pos int) bool {
	for i := 9; i < 17; i++ {
		if w.validMove(pos, offsetVector[i]) != 0 {
			continue
		}
		overlay := int(w.MapBk2[pos+offsetVector[i]])
		if overlay >= FirstTown && overlay <= CityWall2 {
			return true
		}
	}
	return false
}

func (w *World) townHasFlatFootprint(atPos int) bool {
	if !inMap(atPos) {
		return false
	}
	for i := 0; i < 17; i++ {
		if w.validMove(atPos, offsetVector[i]) == 1 {
			continue
		}
		if w.MapBlk[atPos+offsetVector[i]] == FlatBlock {
			return true
		}
	}
	return false
}

func (w *World) cityPieceCount(atPos int) int {
	if !inMap(atPos) {
		return 0
	}
	pieces := 0
	for i := 0; i < 9; i++ {
		if w.validMove(atPos, offsetVector[i]) == 1 {
			continue
		}
		overlay := int(w.MapBk2[atPos+offsetVector[i]])
		if overlay >= CityTower && overlay <= CityWall2 {
			pieces++
		}
	}
	return pieces
}

func (w *World) checkLife(player, position int) int {
	if player < 0 || player > 1 || !inMap(position) {
		return 0
	}
	food := 0
	ownFarm := FarmBlock + player
	centerIsCity := int(w.MapBk2[position]) == CityCentre
	for i := 0; i < 17; i++ {
		move := w.validMove(position, offsetVector[i])
		if move != 0 {
			if move == 2 {
				food += RockLandFood
			}
			continue
		}

		target := position + offsetVector[i]
		block := int(w.MapBlk[target])
		if block == ownFarm || block == FlatBlock {
			if food == 0 {
				food = StartFood
			}
			food += FlatLandFood
		} else if i == 0 {
			return 0
		}

		overlay := int(w.MapBk2[target])
		if i < 9 && centerIsCity && overlay >= CityTower && overlay <= CityWall2 {
			continue
		}
		if i != 0 && overlay >= FirstTown && overlay <= CityWall2 {
			return 0
		}
	}
	if food < StartFood-FlatLandFood {
		food = 0
	}
	if food == MaxFood {
		food = CityFood
	}
	return food
}

func (w *World) setTown(index int, clear bool) {
	if index < 0 || index >= len(w.Peeps) || !inMap(w.Peeps[index].AtPos) {
		return
	}
	peep := w.Peeps[index]
	playerFarm := byte(FarmBlock + int(peep.Player))
	atPos := peep.AtPos

	if clear {
		limit := 17
		if int(w.MapBk2[atPos]) == CityCentre {
			limit = 25
		}
		for i := 0; i < limit; i++ {
			if w.validMove(atPos, offsetVector[i]) == 1 {
				continue
			}
			pos := atPos + offsetVector[i]
			if limit == 25 || i < 9 {
				w.MapBk2[pos] = 0
			}
			if w.MapBlk[pos] == playerFarm {
				w.MapBlk[pos] = FlatBlock
			}
		}
		if limit != 25 {
			w.MapBk2[atPos] = 0
		}
		return
	}

	if peep.Frame == LastTown {
		for i := 0; i < 25; i++ {
			if w.validMove(atPos, offsetVector[i]) == 1 {
				continue
			}
			pos := atPos + offsetVector[i]
			if i < 9 {
				w.MapBk2[pos] = bigCity[i]
			}
			if w.MapBlk[pos] == FlatBlock {
				w.MapBlk[pos] = playerFarm
			}
		}
		return
	}

	if int(w.MapBk2[atPos]) == CityCentre {
		for i := 0; i < 25; i++ {
			if w.validMove(atPos, offsetVector[i]) == 1 {
				continue
			}
			pos := atPos + offsetVector[i]
			w.MapBk2[pos] = 0
			if w.MapBlk[pos] == playerFarm {
				w.MapBlk[pos] = FlatBlock
			}
		}
	}
	for i := 0; i < 17; i++ {
		if w.validMove(atPos, offsetVector[i]) == 1 {
			continue
		}
		pos := atPos + offsetVector[i]
		if i < 9 {
			w.MapBk2[pos] = 0
		}
		if w.MapBlk[pos] == FlatBlock {
			w.MapBlk[pos] = playerFarm
		}
	}
	if w.MapBlk[atPos] == playerFarm {
		w.MapBk2[atPos] = byte(peep.Frame)
	}
}

func (w *World) validMove(pos, delta int) int {
	if delta == 0 {
		return 0
	}
	next := pos + delta
	if !inMap(next) {
		return 1
	}
	dx := delta & (MapWidth - 1)
	if dx > 3 {
		dx = int(int8(byte(dx | 0xc0)))
	}
	x := (pos & (MapWidth - 1)) + dx
	if x < 0 || x >= MapWidth {
		return 1
	}
	switch int(w.MapBlk[next]) {
	case RockBlock:
		return 2
	case WaterBlock:
		return 3
	default:
		return 0
	}
}

func (w *World) zeroPopulation(index int) {
	if index < 0 || index >= len(w.Peeps) {
		return
	}
	if w.Peeps[index].Flags&InBattle != 0 {
		opponent := w.Peeps[index].BattlePopulation
		if opponent >= 0 && opponent < len(w.Peeps) && opponent != index {
			w.Peeps[opponent].Flags &^= InBattle
		}
	}
	if w.Peeps[index].Flags&InTown != 0 {
		w.setTown(index, true)
	}
	w.clearPeepMapRefs(index)
	for i := range w.Magnets {
		if w.Magnets[i].Carried == index+1 {
			w.Magnets[i].Carried = 0
		}
	}
	w.Peeps[index].Population = 0
	w.Peeps[index].Flags = 0
	w.Peeps[index].Frame = 0
	w.Peeps[index].BattlePopulation = 0
	w.Peeps[index].Direction = 0
	w.Peeps[index].HeadFor = 0
	w.Peeps[index].Status = 0
}

func (w *World) walkDeath() int {
	if w.Rules.WalkDeath < 0 {
		return 0
	}
	return w.Rules.WalkDeath
}

func inMap(pos int) bool {
	return pos >= 0 && pos < MapWidth*MapHeight
}

type lcg uint16

func (r *lcg) next() int {
	next := nextRandom(uint16(*r))
	*r = lcg(next)
	return int(next)
}
