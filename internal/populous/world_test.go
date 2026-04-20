package populous

import "testing"

func TestGenerateWorld(t *testing.T) {
	world := GenerateWorld(Level{Number: 0, Terrain: 0, SeedOffset: 0x0019, PlayerPopulation: 3, EnemyPopulation: 4})
	var land int
	for _, block := range world.MapBlk {
		if block != WaterBlock {
			land++
		}
		if int(block) >= BlocksPerLand {
			t.Fatalf("block index %d exceeds land atlas", block)
		}
	}
	if land == 0 {
		t.Fatal("generated world has no land")
	}
	if len(world.Peeps) != 7 {
		t.Fatalf("len(Peeps) = %d, want 7", len(world.Peeps))
	}
	if world.Magnets[GodPlayer].Carried == 0 {
		t.Fatal("god leader was not assigned")
	}
	if world.Magnets[DevilPlayer].Carried == 0 {
		t.Fatal("devil leader was not assigned")
	}
	for i, peep := range world.Peeps {
		if peep.AtPos < 0 || peep.AtPos >= MapWidth*MapHeight {
			t.Fatalf("peep %d AtPos = %d, out of map", i, peep.AtPos)
		}
		if world.MapWho[peep.AtPos] != byte(i+1) {
			t.Fatalf("MapWho for peep %d = %d, want %d", i, world.MapWho[peep.AtPos], i+1)
		}
	}
}

func TestTutorialWorldUsesOriginalSetup(t *testing.T) {
	level := TutorialLevel()
	world := GenerateWorld(level)
	world.ConfigureTutorial()

	if world.Level.SeedOffset != 27068 || world.Level.Terrain != 1 || world.Level.GameMode&GameWaterFatal == 0 {
		t.Fatalf("tutorial level = %+v, want original seed/terrain/fatal water", world.Level)
	}
	if len(world.Peeps) != 18 {
		t.Fatalf("tutorial peeps = %d, want 18", len(world.Peeps))
	}
	if world.Magnets[GodPlayer].Mana != ManaVolcanoCost {
		t.Fatalf("tutorial god mana = %d, want volcano cost %d", world.Magnets[GodPlayer].Mana, ManaVolcanoCost)
	}
	if world.Computer[GodPlayer].Mode&computerKnight == 0 || world.Computer[GodPlayer].Mode&computerVolcano != 0 {
		t.Fatalf("tutorial player mode = %09b, want powers through knight only", world.Computer[GodPlayer].Mode)
	}
	if world.Computer[DevilPlayer].Mode != computerLand|computerTown {
		t.Fatalf("tutorial enemy mode = %09b, want land+town only", world.Computer[DevilPlayer].Mode)
	}
}

func TestWorldTickSettlesPeeps(t *testing.T) {
	world := GenerateWorld(Level{Number: 0, Terrain: 0, SeedOffset: 0x0019, PlayerPopulation: 2, EnemyPopulation: 2})
	for i := 0; i < 8; i++ {
		world.Tick()
	}
	if world.GameTurn != 8 {
		t.Fatalf("GameTurn = %d, want 8", world.GameTurn)
	}

	var towns, farms int
	for _, peep := range world.Peeps {
		if peep.Population > 0 && peep.Flags == InTown {
			towns++
		}
	}
	for _, block := range world.MapBlk {
		if block == FarmBlock+GodPlayer || block == FarmBlock+DevilPlayer {
			farms++
		}
	}
	if towns == 0 {
		t.Fatal("no peep settled into a town")
	}
	if farms == 0 {
		t.Fatal("set_town did not create farm blocks")
	}
	assertMapWhoConsistent(t, world)
}

func TestRaiseLowerAt(t *testing.T) {
	world := GenerateWorld(Level{Number: 0, Terrain: 0, SeedOffset: 0x0019, PlayerPopulation: 1, EnemyPopulation: 1})
	x, y, ok := findEditableAltPoint(world)
	if !ok {
		t.Fatal("no editable altitude point found")
	}
	pos := x + y*EndWidth
	oldAlt := world.Alt[pos]
	oldMana := world.Magnets[GodPlayer].Mana

	if !world.RaiseAt(GodPlayer, x, y) {
		t.Fatal("RaiseAt returned false")
	}
	if world.Alt[pos] != oldAlt+1 {
		t.Fatalf("raised altitude = %d, want %d", world.Alt[pos], oldAlt+1)
	}
	if world.Magnets[GodPlayer].Mana >= oldMana {
		t.Fatalf("mana did not decrease: got %d, old %d", world.Magnets[GodPlayer].Mana, oldMana)
	}
	assertMapBlocksInAtlas(t, world)

	if !world.LowerAt(GodPlayer, x, y) {
		t.Fatal("LowerAt returned false")
	}
	if world.Alt[pos] != oldAlt {
		t.Fatalf("lowered altitude = %d, want %d", world.Alt[pos], oldAlt)
	}
	assertMapBlocksInAtlas(t, world)
}

func TestMagnetMovesCarrier(t *testing.T) {
	world := GenerateWorld(Level{Number: 0, Terrain: 0, SeedOffset: 0x0019, PlayerPopulation: 1, EnemyPopulation: 1})
	carried := world.Magnets[GodPlayer].Carried - 1
	if carried < 0 || carried >= len(world.Peeps) {
		t.Fatal("god leader was not carried by magnet")
	}
	start := world.Peeps[carried].AtPos
	target := -1
	for _, delta := range toOffset {
		if world.validMove(start, delta) == 0 {
			target = start + delta
			break
		}
	}
	if target < 0 {
		t.Fatal("no valid adjacent magnet target found")
	}

	world.Magnets[GodPlayer].Mana = 1000
	if !world.SetMagnetTo(GodPlayer, target) {
		t.Fatal("SetMagnetTo returned false")
	}
	world.Peeps[carried].Frame = 6
	world.Tick()

	if world.Magnets[GodPlayer].Flags != MagnetMode {
		t.Fatalf("magnet mode = %d, want %d", world.Magnets[GodPlayer].Flags, MagnetMode)
	}
	if world.Magnets[GodPlayer].GoTo != target {
		t.Fatalf("magnet target = %d, want %d", world.Magnets[GodPlayer].GoTo, target)
	}
	if world.Peeps[carried].AtPos == start {
		t.Fatalf("carrier did not move from %d toward %d", start, target)
	}
}

func TestJoinForcesTransfersPopulationAndMagnet(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, IQ: 3, Weapons: 5, Population: 30, AtPos: pos},
		{Flags: OnMove, Player: GodPlayer, IQ: 1, Weapons: 2, Population: 40, AtPos: pos},
	}
	world.Magnets[GodPlayer].Carried = 1
	world.MapWho[pos] = 2

	if world.resolveContact(0, 1) {
		t.Fatal("resolveContact returned true for a force join")
	}
	if world.Peeps[0].Population != 0 {
		t.Fatalf("joining peep population = %d, want 0", world.Peeps[0].Population)
	}
	if world.Peeps[1].Population != 70 {
		t.Fatalf("joined population = %d, want 70", world.Peeps[1].Population)
	}
	if world.Peeps[1].IQ != 3 || world.Peeps[1].Weapons != 5 {
		t.Fatalf("joined stats IQ/weapons = %d/%d, want 3/5", world.Peeps[1].IQ, world.Peeps[1].Weapons)
	}
	if world.Magnets[GodPlayer].Carried != 2 {
		t.Fatalf("carried peep = %d, want 2", world.Magnets[GodPlayer].Carried)
	}
}

func TestBattleResolvesAndRewardsWinner(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Weapons: 20, Population: 80, AtPos: pos},
		{Flags: OnMove, Player: DevilPlayer, Weapons: 1, Population: 20, AtPos: pos},
	}
	world.MapWho[pos] = 2

	if world.resolveContact(0, 1) {
		t.Fatal("resolveContact returned true for a battle")
	}
	if world.Peeps[0].Flags != InBattle || world.Peeps[1].Flags&InBattle == 0 {
		t.Fatalf("battle flags = %02x/%02x, want attacker primary battle and defender in battle", world.Peeps[0].Flags, world.Peeps[1].Flags)
	}

	for i := 0; i < 8 && world.Peeps[0].Flags&InBattle != 0; i++ {
		world.doBattle(0)
	}
	if world.Peeps[0].Population <= 0 {
		t.Fatal("expected god peep to win the battle")
	}
	if world.Peeps[1].Population != 0 {
		t.Fatalf("defeated peep population = %d, want 0", world.Peeps[1].Population)
	}
	if world.Peeps[0].Flags&InEffect == 0 {
		t.Fatalf("winner flags = %02x, want victory effect", world.Peeps[0].Flags)
	}
	if world.MapWho[pos] != 1 {
		t.Fatalf("MapWho[%d] = %d, want winner id 1", pos, world.MapWho[pos])
	}
	if world.Magnets[GodPlayer].Mana != world.Rules.BattleAdd2[0] {
		t.Fatalf("winner mana = %d, want %d", world.Magnets[GodPlayer].Mana, world.Rules.BattleAdd2[0])
	}
}

func TestBattleCapturesTown(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Weapons: 20, Population: 80, AtPos: pos},
		{Flags: InTown, Player: DevilPlayer, Weapons: 1, Population: 20, AtPos: pos, Frame: FirstTown + 1},
	}
	world.setTown(1, false)
	world.MapWho[pos] = 2

	world.resolveContact(0, 1)
	for i := 0; i < 8 && world.Peeps[0].Flags&InBattle != 0; i++ {
		world.doBattle(0)
	}
	if world.Peeps[0].Population <= 0 {
		t.Fatal("expected attacker to survive town battle")
	}
	if world.Peeps[0].Flags&InTown == 0 {
		t.Fatalf("winner flags = %02x, want captured town", world.Peeps[0].Flags)
	}
	if overlay := int(world.MapBk2[pos]); overlay < FirstTown || overlay > CityWall2 {
		t.Fatalf("captured town overlay = %d, want town/city overlay", overlay)
	}
	if world.Peeps[1].Population != 0 {
		t.Fatalf("defender population = %d, want 0", world.Peeps[1].Population)
	}
	if world.Magnets[GodPlayer].Mana != world.Rules.BattleAdd1[1] {
		t.Fatalf("winner mana = %d, want town reward %d", world.Magnets[GodPlayer].Mana, world.Rules.BattleAdd1[1])
	}
}

func TestKnightRazesTownIntoRuin(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Weapons: 20, Population: 80, AtPos: pos, Status: KnightStatus, HeadFor: 2},
		{Flags: InTown, Player: DevilPlayer, Weapons: 1, Population: 20, AtPos: pos, Frame: LastTown},
	}
	world.setTown(1, false)
	world.MapWho[pos] = 2

	world.resolveContact(0, 1)
	for i := 0; i < 8 && world.Peeps[0].Flags&InBattle != 0; i++ {
		world.doBattle(0)
	}

	if world.Peeps[0].Flags&InTown != 0 {
		t.Fatalf("winner flags = %02x, should not capture a ruined town", world.Peeps[0].Flags)
	}
	if world.Peeps[1].Flags != InRuin {
		t.Fatalf("loser flags = %02x, want ruin", world.Peeps[1].Flags)
	}
	if world.Peeps[1].Population != 1 {
		t.Fatalf("ruin population = %d, want 1", world.Peeps[1].Population)
	}
	if world.Peeps[1].BattlePopulation != 40 {
		t.Fatalf("ruin timer = %d, want 40", world.Peeps[1].BattlePopulation)
	}
	if got := int(world.MapBk2[pos]); got != LastTown+(FirstRuinTown-FirstTown-1) {
		t.Fatalf("ruined castle centre = %d, want %d", got, LastTown+(FirstRuinTown-FirstTown-1))
	}
	if world.MapBlk[pos+offsetVector[5]] != BadLand {
		t.Fatalf("ruined castle farmland = %d, want bad land", world.MapBlk[pos+offsetVector[5]])
	}
}

func TestWarStatusDoesNotRazeTownWithoutKnightHeading(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), War: true}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Weapons: 20, Population: 80, AtPos: pos, Status: KnightStatus},
		{Flags: InTown, Player: DevilPlayer, Weapons: 1, Population: 20, AtPos: pos, Frame: LastTown},
	}
	world.setTown(1, false)
	world.MapWho[pos] = 2

	world.resolveContact(0, 1)
	for i := 0; i < 8 && world.Peeps[0].Flags&InBattle != 0; i++ {
		world.doBattle(0)
	}

	if world.Peeps[1].Flags == InRuin {
		t.Fatal("war-only knight status razed town without an original head_for target")
	}
	if world.Peeps[1].Population != 0 {
		t.Fatalf("defender population = %d, want 0", world.Peeps[1].Population)
	}
}

func TestSwampAtPlacesSwamps(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.Magnets[GodPlayer].Mana = ManaSwampCost

	if !world.SwampAt(GodPlayer, 20, 20) {
		t.Fatal("SwampAt returned false")
	}
	if world.Magnets[GodPlayer].Mana != 0 {
		t.Fatalf("mana after swamp = %d, want 0", world.Magnets[GodPlayer].Mana)
	}
	if swamps := countBlocks(world, SwampBlock); swamps == 0 {
		t.Fatal("SwampAt did not place any swamp block")
	}
}

func TestKnightConvertsCarrier(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	godPos := 20 + 20*MapWidth
	devilPos := 24 + 24*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 100, AtPos: godPos},
		{Flags: OnMove, Player: DevilPlayer, Population: 100, AtPos: devilPos},
	}
	world.MapWho[godPos] = 1
	world.MapWho[devilPos] = 2
	world.Magnets[GodPlayer].Carried = 1
	world.Magnets[GodPlayer].Mana = ManaKnightCost

	if !world.Knight(GodPlayer) {
		t.Fatal("Knight returned false")
	}
	if world.Magnets[GodPlayer].Carried != 0 {
		t.Fatalf("carried peep = %d, want 0", world.Magnets[GodPlayer].Carried)
	}
	if world.Peeps[0].Status != KnightStatus {
		t.Fatalf("peep status = %d, want knight", world.Peeps[0].Status)
	}
	if world.Peeps[0].HeadFor != 2 {
		t.Fatalf("knight target = %d, want peep id 2", world.Peeps[0].HeadFor)
	}
	if world.Magnets[GodPlayer].Mana != 0 {
		t.Fatalf("mana after knight = %d, want 0", world.Magnets[GodPlayer].Mana)
	}
}

func TestKnightRequiresMagnetCarrier(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{{Flags: OnMove, Player: GodPlayer, Population: 100, AtPos: pos}}
	world.MapWho[pos] = 1
	world.Magnets[GodPlayer].Mana = ManaKnightCost

	if world.Knight(GodPlayer) {
		t.Fatal("Knight returned true without a papal magnet carrier")
	}
	if world.Peeps[0].Status == KnightStatus {
		t.Fatal("peep was knighted without carrying the papal magnet")
	}
	if world.Magnets[GodPlayer].Mana != ManaKnightCost {
		t.Fatalf("mana after failed knight = %d, want %d", world.Magnets[GodPlayer].Mana, ManaKnightCost)
	}
}

func TestKnightStartsBattleAtTargetTile(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 100, AtPos: pos, Status: KnightStatus, HeadFor: 2},
		{Flags: OnMove, Player: DevilPlayer, Population: 100, AtPos: pos},
	}
	world.MapWho[pos] = 2

	world.moveExplorer(0)

	if world.Peeps[0].Flags&InBattle == 0 || world.Peeps[1].Flags&InBattle == 0 {
		t.Fatalf("battle was not started: flags=%02x/%02x", world.Peeps[0].Flags, world.Peeps[1].Flags)
	}
}

func TestQuakeAtChangesTerrain(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	world.Magnets[GodPlayer].Mana = ManaQuakeCost
	for y := 10; y < 19; y++ {
		for x := 10; x < 19; x++ {
			world.Alt[x+y*EndWidth] = 2
		}
	}
	world.makeMap(9, 9, 19, 19)
	before := sumAlt(world)

	if !world.QuakeAt(GodPlayer, 10, 10) {
		t.Fatal("QuakeAt returned false")
	}
	if after := sumAlt(world); after == before {
		t.Fatalf("altitude sum unchanged at %d", after)
	}
}

func TestVolcanoAtRaisesAndPlacesRocks(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.Magnets[GodPlayer].Mana = ManaVolcanoCost
	before := sumAlt(world)

	if !world.VolcanoAt(GodPlayer, 20, 20) {
		t.Fatal("VolcanoAt returned false")
	}
	if after := sumAlt(world); after <= before {
		t.Fatalf("altitude sum = %d, want > %d", after, before)
	}
	if rocks := countBlocks(world, RockBlock); rocks == 0 {
		t.Fatal("VolcanoAt did not place any rock block")
	}
}

func TestFloodLowersLand(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	world.Magnets[GodPlayer].Mana = ManaFloodCost
	pos := 10 + 10*EndWidth
	world.Alt[pos] = 2
	world.makeMap(9, 9, 10, 10)

	if !world.Flood(GodPlayer) {
		t.Fatal("Flood returned false")
	}
	if world.Alt[pos] != 1 {
		t.Fatalf("flooded altitude = %d, want 1", world.Alt[pos])
	}
	if world.Magnets[GodPlayer].Mana != 0 {
		t.Fatalf("mana after flood = %d, want 0", world.Magnets[GodPlayer].Mana)
	}
}

func TestWarPowerEnablesWar(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	world.Magnets[GodPlayer].Mana = ManaWarCost
	world.Magnets[GodPlayer].GoTo = 3 + 4*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 20, AtPos: 10 + 10*MapWidth},
		{Flags: InTown, Player: DevilPlayer, Population: 20, AtPos: 20 + 20*MapWidth, Frame: FirstTown},
	}
	world.MapWho[world.Peeps[0].AtPos] = 1
	world.MapWho[world.Peeps[1].AtPos] = 2

	if !world.WarPower(GodPlayer) {
		t.Fatal("WarPower returned false")
	}
	if !world.War {
		t.Fatal("world war flag was not enabled")
	}
	center := MapWidth/2 + (MapHeight/2)*MapWidth
	if world.Magnets[GodPlayer].GoTo != center || world.Magnets[DevilPlayer].GoTo != center {
		t.Fatalf("war magnets = %d/%d, want center %d", world.Magnets[GodPlayer].GoTo, world.Magnets[DevilPlayer].GoTo, center)
	}
	events := world.DrainSoundEvents()
	if len(events) != 1 || events[0] != TuneWar {
		t.Fatalf("war sound events = %v, want [%d]", events, TuneWar)
	}
	for i, peep := range world.Peeps {
		if peep.Status != KnightStatus {
			t.Fatalf("peep %d status = %d, want knight", i, peep.Status)
		}
		if peep.Flags&InTown != 0 {
			t.Fatalf("peep %d stayed in town during war", i)
		}
	}
}

func TestWarMovementCrossesRocksAndRaisesWaterLikeOriginal(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), War: true}
	fillFlat(world)
	pos := 10 + 10*MapWidth
	rockPos := pos + 1
	world.MapBlk[rockPos] = RockBlock
	world.Peeps = []Peep{{Flags: OnMove, Player: GodPlayer, Population: 20, AtPos: pos}}
	world.MapWho[pos] = 1
	world.Magnets[GodPlayer].GoTo = rockPos

	if delta := world.moveToward(0, rockPos, true); delta != 1 {
		t.Fatalf("war rock movement delta = %d, want 1", delta)
	}
	world.moveExplorer(0)
	if world.Peeps[0].AtPos != rockPos {
		t.Fatalf("war rock movement at pos = %d, want %d", world.Peeps[0].AtPos, rockPos)
	}

	waterPos := 20 + 20*MapWidth
	world = &World{Rules: DefaultTerrainRules(), War: true}
	world.MapBlk[waterPos] = WaterBlock
	world.Peeps = []Peep{{Flags: OnMove | InWater, Player: GodPlayer, Population: 20, AtPos: waterPos}}
	world.MapWho[waterPos] = 1

	world.TickWithComputer([2]bool{})

	if world.Alt[20+20*EndWidth] == 0 {
		t.Fatal("war peep in water did not raise its ground point")
	}
}

func TestWarWaitingPeepKeepsCurrentMapReference(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), War: true}
	fillFlat(world)
	center := MapWidth/2 + (MapHeight/2)*MapWidth
	world.Magnets[GodPlayer].GoTo = center
	world.Peeps = []Peep{{Flags: OnMove, Player: GodPlayer, Population: 20, AtPos: center, Direction: 1, Status: KnightStatus}}
	world.MapWho[center-1] = 1

	world.moveExplorer(0)

	if world.MapWho[center-1] != 0 {
		t.Fatalf("old MapWho = %d, want cleared", world.MapWho[center-1])
	}
	if world.MapWho[center] != 1 {
		t.Fatalf("center MapWho = %d, want peep reference", world.MapWho[center])
	}
	if world.Peeps[0].Flags&IAmWaiting == 0 {
		t.Fatalf("peep flags = %08b, want waiting", world.Peeps[0].Flags)
	}
}

func TestPowerRequiresLevelMode(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	world.Computer[GodPlayer].Mode = computerLand | computerTown | computerLeader
	world.Magnets[GodPlayer].Mana = ManaQuakeCost

	if world.QuakeAt(GodPlayer, 10, 10) {
		t.Fatal("QuakeAt returned true without quake power")
	}
	if world.Magnets[GodPlayer].Mana != ManaQuakeCost {
		t.Fatalf("mana after blocked quake = %d, want %d", world.Magnets[GodPlayer].Mana, ManaQuakeCost)
	}
}

func TestComputerUsesWarWhenStrongerAndUnlocked(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{
		Mode:   computerLand | computerTown | computerLeader | computerWar,
		Skill:  1,
		Speed:  1,
		Best1:  -1,
		Best2:  -1,
		MyBest: -1,
	}
	world.Magnets[DevilPlayer] = Magnet{Flags: SettleMode, Mana: ManaWarCost + 1000}
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 50, AtPos: 10 + 10*MapWidth},
		{Flags: OnMove, Player: DevilPlayer, Population: 200, AtPos: 40 + 40*MapWidth},
	}
	world.MapWho[world.Peeps[0].AtPos] = 1
	world.MapWho[world.Peeps[1].AtPos] = 2

	world.Tick()

	if !world.War {
		t.Fatal("computer did not trigger war")
	}
	if world.Magnets[DevilPlayer].Mana != 1000 {
		t.Fatalf("devil mana after war = %d, want 1000", world.Magnets[DevilPlayer].Mana)
	}
}

func TestComputerControlCanRunGodSide(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.ComputerControlled[GodPlayer] = true
	world.Computer[GodPlayer] = ComputerStats{
		Mode:   computerLand | computerTown | computerLeader | computerWar,
		Skill:  1,
		Speed:  1,
		Best1:  -1,
		Best2:  -1,
		MyBest: -1,
	}
	world.Magnets[GodPlayer] = Magnet{Flags: SettleMode, Mana: ManaWarCost + 1000}
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 200, AtPos: 10 + 10*MapWidth},
		{Flags: OnMove, Player: DevilPlayer, Population: 50, AtPos: 40 + 40*MapWidth},
	}
	world.MapWho[world.Peeps[0].AtPos] = 1
	world.MapWho[world.Peeps[1].AtPos] = 2

	world.TickWithComputer([2]bool{true, false})

	if !world.War {
		t.Fatal("god-side computer did not trigger war")
	}
}

func TestComputerVolcanoTargetsOldEnemyTown(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{
		Mode:   computerLand | computerTown | computerLeader | computerVolcano,
		Skill:  1,
		Speed:  1,
		Best1:  -1,
		Best2:  -1,
		MyBest: -1,
	}
	world.Magnets[DevilPlayer] = Magnet{Flags: SettleMode, Mana: ManaVolcanoCost + 501}
	townPos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: InTown, Player: GodPlayer, Population: CityFood, AtPos: townPos, Frame: LastTown, BattlePopulation: 1},
		{Flags: OnMove, Player: DevilPlayer, Population: 200, AtPos: 40 + 40*MapWidth},
	}
	world.MapWho[townPos] = 1
	world.MapWho[world.Peeps[1].AtPos] = 2

	world.Tick()

	if world.Magnets[DevilPlayer].Mana != 501 {
		t.Fatalf("devil mana after volcano = %d, want 501", world.Magnets[DevilPlayer].Mana)
	}
	if sumAlt(world) == 0 {
		t.Fatal("computer volcano did not alter terrain")
	}
}

func TestComputerActionSlotOpensOnOriginalSpeedCadence(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), GameTurn: 9}
	fillFlat(world)
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{
		Mode:     computerLand | computerTown | computerLeader | computerWar,
		Skill:    1,
		Speed:    10,
		DoneTurn: 1,
		Best1:    -1,
		Best2:    -1,
		MyBest:   -1,
	}
	world.Magnets[DevilPlayer] = Magnet{Flags: SettleMode, Mana: ManaWarCost + 1000}
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 50, AtPos: 10 + 10*MapWidth},
		{Flags: OnMove, Player: DevilPlayer, Population: 200, AtPos: 40 + 40*MapWidth},
	}
	world.MapWho[world.Peeps[0].AtPos] = 1
	world.MapWho[world.Peeps[1].AtPos] = 2

	world.TickWithComputer([2]bool{false, true})
	if world.War {
		t.Fatal("computer acted before its speed slot opened")
	}
	if world.Computer[DevilPlayer].DoneTurn != 0 {
		t.Fatalf("computer action slot = %d, want open at speed boundary", world.Computer[DevilPlayer].DoneTurn)
	}

	world.TickWithComputer([2]bool{false, true})
	if !world.War {
		t.Fatal("computer did not use the action slot on the following turn")
	}
}

func TestComputerVolcanoCanTargetMovingEnemyLikeOriginal(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{
		Mode:   computerLand | computerTown | computerLeader | computerVolcano,
		Skill:  1,
		Speed:  1,
		Best1:  -1,
		Best2:  -1,
		MyBest: -1,
	}
	world.Magnets[DevilPlayer] = Magnet{Flags: SettleMode, Mana: ManaVolcanoCost + 501}
	enemyPos := 20 + 20*MapWidth
	world.Peeps = []Peep{
		{Flags: OnMove, Player: GodPlayer, Population: 100, AtPos: enemyPos},
		{Flags: OnMove, Player: DevilPlayer, Population: 200, AtPos: 40 + 40*MapWidth},
	}
	world.MapWho[enemyPos] = 1
	world.MapWho[world.Peeps[1].AtPos] = 2

	world.Tick()

	if world.Magnets[DevilPlayer].Mana != 501 {
		t.Fatalf("devil mana after moving-target volcano = %d, want 501", world.Magnets[DevilPlayer].Mana)
	}
}

func TestComputerMakeLevelRaisesTowardTownAltitude(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.GameTurn = 1
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{Mode: computerLand, Skill: 1, Speed: 1}
	world.Magnets[DevilPlayer].Mana = 1000
	pos := 10 + 10*MapWidth
	world.Alt[10+10*EndWidth] = 2
	targetAlt := (10 - 4) + (10-4)*EndWidth

	complete := world.computerMakeLevel(pos, DevilPlayer)

	if complete {
		t.Fatal("computerMakeLevel returned complete while terrain was uneven")
	}
	if world.Alt[targetAlt] == 0 {
		t.Fatal("computerMakeLevel did not raise lower terrain toward town altitude")
	}
	if world.Computer[DevilPlayer].DoneTurn != world.GameTurn {
		t.Fatalf("DoneTurn = %d, want %d", world.Computer[DevilPlayer].DoneTurn, world.GameTurn)
	}
}

func TestComputerMakeLevelWeakensRockBeforeLowering(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	for i := range world.Alt {
		world.Alt[i] = 2
	}
	world.GameTurn = 1
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{Mode: computerLand, Skill: 1, Speed: 1}
	world.Magnets[DevilPlayer].Mana = 1000
	pos := 10 + 10*MapWidth
	target := (10 - 4) + (10-4)*MapWidth
	world.MapBlk[target] = RockBlock

	world.computerMakeLevel(pos, DevilPlayer)

	if world.MapBlk[target] == RockBlock {
		t.Fatal("computerMakeLevel kept an exact ROCK_BLOCK instead of weakening it")
	}
}

func TestComputerOneBlockFlatRaisesLowCorner(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.GameTurn = 1
	world.ComputerControlled[DevilPlayer] = true
	world.Computer[DevilPlayer] = ComputerStats{Mode: computerLand, Skill: 1, Speed: 1}
	world.Magnets[DevilPlayer].Mana = 1000
	pos := 10 + 10*MapWidth
	world.Alt[10+10*EndWidth] = 1
	world.Alt[11+10*EndWidth] = 1
	world.Alt[11+11*EndWidth] = 1

	if !world.computerOneBlockFlat(pos, DevilPlayer) {
		t.Fatal("computerOneBlockFlat returned false")
	}
	if world.Alt[10+11*EndWidth] == 0 {
		t.Fatal("computerOneBlockFlat did not raise the low corner")
	}
}

func TestGameModeRestrictsSculpting(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), Level: Level{GameMode: GameNoBuild}}
	world.Magnets[GodPlayer].Mana = 1000
	if world.RaiseAt(GodPlayer, 10, 10) {
		t.Fatal("RaiseAt returned true in no-build mode")
	}

	world = &World{Rules: DefaultTerrainRules(), Level: Level{GameMode: GameOnlyRaise}}
	world.Magnets[GodPlayer].Mana = 1000
	world.Alt[10+10*EndWidth] = 1
	if world.LowerAt(GodPlayer, 10, 10) {
		t.Fatal("LowerAt returned true in only-raise mode")
	}
}

func TestPaintModeBypassesManaAndBuildMode(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), Level: Level{GameMode: GameNoBuild}}
	if !world.PaintRaiseAt(10, 10) {
		t.Fatal("PaintRaiseAt returned false in no-build mode without mana")
	}
	if world.Alt[10+10*EndWidth] == 0 {
		t.Fatal("PaintRaiseAt did not raise the point")
	}

	world = &World{Rules: DefaultTerrainRules(), Level: Level{GameMode: GameOnlyRaise}}
	world.Alt[10+10*EndWidth] = 1
	if !world.PaintLowerAt(10, 10) {
		t.Fatal("PaintLowerAt returned false in only-raise mode without mana")
	}
	if world.Alt[10+10*EndWidth] != 0 {
		t.Fatalf("PaintLowerAt altitude = %d, want 0", world.Alt[10+10*EndWidth])
	}
}

func TestBuildPresenceCanRequireTown(t *testing.T) {
	pos := 10 + 10*MapWidth
	world := &World{}
	world.Peeps = []Peep{{Flags: OnMove, Player: GodPlayer, Population: 20, AtPos: pos}}
	world.MapWho[pos] = 1

	if !world.HasBuildPresence(GodPlayer, 8, 8, 8, 8) {
		t.Fatal("HasBuildPresence did not accept a visible own peep")
	}

	world.Level.GameMode = GameRaiseTown
	if world.HasBuildPresence(GodPlayer, 8, 8, 8, 8) {
		t.Fatal("HasBuildPresence accepted a non-town peep in town-build mode")
	}

	world.Peeps[0].Flags = InTown
	if !world.HasBuildPresence(GodPlayer, 8, 8, 8, 8) {
		t.Fatal("HasBuildPresence did not accept a visible own town in town-build mode")
	}
}

func TestWaterFatalKillsPeopleInWater(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules(), Level: Level{GameMode: GameWaterFatal}}
	pos := 10 + 10*MapWidth
	world.MapBlk[pos] = WaterBlock
	world.Peeps = []Peep{{Flags: OnMove | InWater, Player: GodPlayer, Population: 20, AtPos: pos}}
	world.MapWho[pos] = 1

	world.Tick()

	if world.Peeps[0].Population != 0 {
		t.Fatalf("population in fatal water = %d, want 0", world.Peeps[0].Population)
	}
	if world.MapWho[pos] != 0 {
		t.Fatalf("MapWho after fatal water = %d, want cleared", world.MapWho[pos])
	}
}

func TestSwampRemainModeControlsConsumedSwamp(t *testing.T) {
	pos := 10 + 10*MapWidth
	world := &World{Rules: DefaultTerrainRules()}
	world.MapBlk[pos] = SwampBlock
	world.Peeps = []Peep{{Flags: OnMove, Player: GodPlayer, Population: 20, AtPos: pos, Frame: 6}}
	world.MapWho[pos] = 1

	world.Tick()

	if world.Peeps[0].Population != 0 {
		t.Fatalf("population in swamp = %d, want 0", world.Peeps[0].Population)
	}
	if world.MapBlk[pos] != FlatBlock {
		t.Fatalf("consumed swamp block = %d, want flat", world.MapBlk[pos])
	}

	world = &World{Rules: DefaultTerrainRules(), Level: Level{GameMode: GameSwampRemain}}
	world.MapBlk[pos] = SwampBlock
	world.Peeps = []Peep{{Flags: OnMove, Player: GodPlayer, Population: 20, AtPos: pos, Frame: 6}}
	world.MapWho[pos] = 1

	world.Tick()

	if world.Peeps[0].Population != 0 {
		t.Fatalf("population in remaining swamp = %d, want 0", world.Peeps[0].Population)
	}
	if world.MapBlk[pos] != SwampBlock {
		t.Fatalf("remaining swamp block = %d, want swamp", world.MapBlk[pos])
	}
}

func TestResultSummaryAndEndScore(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	world.Score = 100
	world.BattleWon[GodPlayer] = 3
	world.BattleWon[DevilPlayer] = 1
	allPowers := computerQuake | computerSwamp | computerKnight | computerVolcano | computerFlood | computerWar
	world.Computer[GodPlayer].Mode = computerLand | computerTown | computerLeader | allPowers
	world.Computer[DevilPlayer].Mode = computerLand | computerTown | computerLeader
	world.Computer[DevilPlayer].Speed = 10
	world.Peeps = []Peep{
		{Flags: InTown, Player: GodPlayer, Population: 40, AtPos: 10 + 10*MapWidth, Frame: FirstTown + 1},
		{Flags: InTown, Player: GodPlayer, Population: 60, AtPos: 14 + 14*MapWidth, Frame: LastTown},
		{Flags: OnMove, Player: GodPlayer, Population: 30, AtPos: 18 + 18*MapWidth, Status: KnightStatus, HeadFor: 4},
		{Flags: OnMove, Player: GodPlayer, Population: 30, AtPos: 22 + 22*MapWidth, Status: KnightStatus},
		{Flags: OnMove, Player: DevilPlayer, Population: 20, AtPos: 40 + 40*MapWidth},
	}

	if result := world.ResultFor(GodPlayer); result != ResultOngoing {
		t.Fatalf("result = %d, want ongoing", result)
	}
	summary := world.SummaryFor(GodPlayer)
	if summary.BattlesWon != 3 || summary.Knights != 1 || summary.Towns != 1 || summary.Castles != 1 {
		t.Fatalf("summary = %+v, want battles=3 knights=1 towns=1 castles=1", summary)
	}
	if score := world.EndScore(GodPlayer, false); score != (100+ScoreBattle)*ScoreWon {
		t.Fatalf("end score = %d, want %d", score, (100+ScoreBattle)*ScoreWon)
	}

	world.Peeps[4].Population = 0
	if result := world.ResultFor(GodPlayer); result != ResultWon {
		t.Fatalf("result = %d, want won", result)
	}
	world.Peeps[0].Population = 0
	world.Peeps[1].Population = 0
	world.Peeps[2].Population = 0
	world.Peeps[3].Population = 0
	if result := world.ResultFor(GodPlayer); result != ResultLost {
		t.Fatalf("result = %d, want lost", result)
	}
}

func TestEndScoreAddsOriginalOptionAndSpeedBonuses(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	world.Score = 100
	world.Computer[GodPlayer].Mode = computerLand | computerTown | computerLeader | computerQuake
	world.Computer[DevilPlayer].Mode = computerLand | computerTown | computerLeader | computerQuake | computerWar
	world.Computer[DevilPlayer].Speed = 8

	got := world.EndScore(GodPlayer, false)
	want := (100 + 5*ScoreYourOptions + 2*ScoreHisOptions + 2*ScoreSpeed) * ScoreWon
	if got != want {
		t.Fatalf("end score = %d, want %d", got, want)
	}
}

func TestNextConquestLevelIndexMatchesOriginalAdvance(t *testing.T) {
	if got := NextConquestLevelIndex(0, 500); got != 1 {
		t.Fatalf("next level from 0 score 500 = %d, want 1", got)
	}
	if got := NextConquestLevelIndex(10, 555555); got != 33 {
		t.Fatalf("next level from 10 score 555555 = %d, want 33", got)
	}
	if got := NextConquestLevelIndex(493, 555555); got != 494 {
		t.Fatalf("next level from 493 score 555555 = %d, want 494", got)
	}
	if got := NextConquestLevelIndex(494, 555555); got != 0 {
		t.Fatalf("next level from 494 score 555555 = %d, want 0", got)
	}
}

func TestSculptingPreservesDecorUntilWater(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	world.Magnets[GodPlayer].Mana = 1000
	for i := range world.Alt {
		world.Alt[i] = 2
	}

	x, y := 10, 10
	pos := x + y*MapWidth
	world.MapBlk[pos] = RockBlock
	world.MapBk2[pos] = TreeBlock

	if !world.RaiseAt(GodPlayer, x, y) {
		t.Fatal("RaiseAt returned false")
	}
	if world.MapBlk[pos] != RockBlock {
		t.Fatalf("MapBlk[%d] = %d, want preserved rock", pos, world.MapBlk[pos])
	}
	if world.MapBk2[pos] != TreeBlock {
		t.Fatalf("MapBk2[%d] = %d, want preserved tree", pos, world.MapBk2[pos])
	}

	world = &World{Rules: DefaultTerrainRules()}
	world.Magnets[GodPlayer].Mana = 1000

	world.Alt[x+y*EndWidth] = 1
	world.MapBlk[pos] = RockBlock
	world.MapBk2[pos] = TreeBlock

	if !world.LowerAt(GodPlayer, x, y) {
		t.Fatal("LowerAt returned false")
	}
	if world.MapBlk[pos] != WaterBlock {
		t.Fatalf("MapBlk[%d] = %d, want water", pos, world.MapBlk[pos])
	}
	if world.MapBk2[pos] != 0 {
		t.Fatalf("MapBk2[%d] = %d, want cleared overlay", pos, world.MapBk2[pos])
	}
}

func TestSetTownCastleKeepsAllPiecesNearAnotherTown(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)

	posA := 20 + 20*MapWidth
	posB := posA + 1
	world.Peeps = []Peep{
		{Flags: InTown, Player: GodPlayer, Population: CityFood, AtPos: posA, Frame: LastTown},
		{Flags: InTown, Player: GodPlayer, Population: StartFood, AtPos: posB, Frame: FirstTown + 1},
	}

	world.setTown(1, false)
	world.setTown(0, false)

	if pieces := world.cityPieceCount(posA); pieces != 9 {
		t.Fatalf("castle pieces = %d, want 9", pieces)
	}
}

func TestProcessTownRepairsIncompleteCity(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)

	pos := 20 + 20*MapWidth
	world.Peeps = []Peep{{Flags: InTown, Player: GodPlayer, Population: CityFood, AtPos: pos, Frame: LastTown}}
	world.MapWho[pos] = 1
	world.setTown(0, false)

	damaged := pos + offsetVector[5]
	world.MapBk2[damaged] = 0
	if pieces := world.cityPieceCount(pos); pieces != 8 {
		t.Fatalf("damaged city pieces = %d, want 8", pieces)
	}

	world.processTown(0)
	if world.MapBk2[damaged] != bigCity[5] {
		t.Fatalf("repaired city piece = %d, want %d", world.MapBk2[damaged], bigCity[5])
	}
	if pieces := world.cityPieceCount(pos); pieces != 9 {
		t.Fatalf("repaired city pieces = %d, want 9", pieces)
	}
}

func TestRuinExpiresAfterTimer(t *testing.T) {
	world := &World{Rules: DefaultTerrainRules()}
	fillFlat(world)
	pos := 12 + 12*MapWidth
	world.Peeps = []Peep{{Flags: InRuin, Player: GodPlayer, Population: 1, AtPos: pos, BattlePopulation: 1}}

	world.Tick()
	if world.Peeps[0].Population != 1 || world.Peeps[0].BattlePopulation != 0 {
		t.Fatalf("after first tick ruin = pop %d timer %d, want pop 1 timer 0", world.Peeps[0].Population, world.Peeps[0].BattlePopulation)
	}
	world.Tick()
	if world.Peeps[0].Population != 0 {
		t.Fatalf("after second tick ruin population = %d, want 0", world.Peeps[0].Population)
	}
}

func fillFlat(world *World) {
	for i := range world.MapBlk {
		world.MapBlk[i] = FlatBlock
	}
}

func countBlocks(world *World, block int) int {
	count := 0
	for _, value := range world.MapBlk {
		if int(value) == block {
			count++
		}
	}
	return count
}

func sumAlt(world *World) int {
	sum := 0
	for _, alt := range world.Alt {
		sum += alt
	}
	return sum
}

func findEditableAltPoint(world *World) (int, int, bool) {
	for y := 1; y < MapHeight; y++ {
		for x := 1; x < MapWidth; x++ {
			alt := world.Alt[x+y*EndWidth]
			if alt > 0 && alt < 8 {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func assertMapBlocksInAtlas(t *testing.T, world *World) {
	t.Helper()
	for pos, block := range world.MapBlk {
		if int(block) >= BlocksPerLand {
			t.Fatalf("MapBlk[%d] = %d, outside land atlas", pos, block)
		}
	}
}

func assertMapWhoConsistent(t *testing.T, world *World) {
	t.Helper()
	for pos, who := range world.MapWho {
		if who == 0 {
			continue
		}
		index := int(who) - 1
		if index < 0 || index >= len(world.Peeps) {
			t.Fatalf("MapWho[%d] = %d, outside peeps", pos, who)
		}
		peep := world.Peeps[index]
		if peep.Population <= 0 {
			t.Fatalf("MapWho[%d] points to dead peep %d", pos, index)
		}
		if pos != peep.AtPos && pos != peep.AtPos-peep.Direction {
			t.Fatalf("MapWho[%d] points to peep %d at %d dir %d", pos, index, peep.AtPos, peep.Direction)
		}
	}
}
