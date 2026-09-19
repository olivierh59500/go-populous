package populous

import "testing"

func advancedKnightDefenseWorld() *World {
	w := advancedTestWorld()
	w.Rules.WalkDeath = 4
	w.Computer[GodPlayer].Mode |= computerSwamp
	w.Magnets[GodPlayer].Mana = ManaSwampCost
	w.Magnets[GodPlayer].GoTo = 60 + 60*MapWidth
	w.rng = lcg(1234)
	return w
}

func TestAdvancedKnightDefenseCastsAheadWithNormalCostAndRNG(t *testing.T) {
	w := advancedKnightDefenseWorld()
	knightPos, targetPos := 10+10*MapWidth, 20+10*MapWidth
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: knightPos},
		{Player: GodPlayer, Flags: InTown, Population: 500, Weapons: 3, AtPos: targetPos},
	}
	w.MapWho[knightPos], w.MapWho[targetPos] = 1, 2
	beforeRNG := w.rng
	if !w.advancedKnightDefense(GodPlayer) {
		t.Fatal("directed enemy knight did not trigger a defensive swamp")
	}
	if w.Magnets[GodPlayer].Mana != 0 {
		t.Fatalf("defensive swamp mana=%d, want 0", w.Magnets[GodPlayer].Mana)
	}
	if w.rng == beforeRNG {
		t.Fatal("committed swamp did not consume the normal simulation RNG")
	}
	if blocks := countBlocks(w, SwampBlock); blocks == 0 {
		t.Fatal("committed defensive spell placed no swamp in its legal area")
	}
	if w.Peeps[0].AtPos != knightPos || w.Peeps[1].AtPos != targetPos {
		t.Fatal("defensive planning moved a unit directly")
	}
}

func TestAdvancedKnightDefenseWorksForEitherSide(t *testing.T) {
	for _, player := range []int{GodPlayer, DevilPlayer} {
		t.Run(string(rune('0'+player)), func(t *testing.T) {
			w := advancedTestWorld()
			w.Rules.WalkDeath = 4
			w.Computer[player].Mode |= computerSwamp
			w.Magnets[player].Mana = ManaSwampCost
			w.Magnets[player].GoTo = 60 + 60*MapWidth
			knightPos, targetPos := 10+10*MapWidth, 20+10*MapWidth
			w.Peeps = []Peep{
				{Player: byte(player ^ 1), Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: knightPos},
				{Player: byte(player), Flags: InTown, Population: 500, Weapons: 3, AtPos: targetPos},
			}
			w.MapWho[knightPos], w.MapWho[targetPos] = 1, 2
			if !w.advancedKnightDefense(player) || w.Magnets[player].Mana != 0 {
				t.Fatal("defensive swamp is not symmetric between camps")
			}
		})
	}
}

func TestAdvancedKnightDefenseChoosesHighestThreat(t *testing.T) {
	w := advancedKnightDefenseWorld()
	w.Magnets[GodPlayer].Mana = ManaSwampCost * 2
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: 10 + 10*MapWidth},
		{Player: GodPlayer, Flags: InTown, Population: 500, Weapons: 3, AtPos: 20 + 10*MapWidth},
		{Player: DevilPlayer, Flags: OnMove, Population: 500, Weapons: 1, Status: KnightStatus, HeadFor: 4, AtPos: 40 + 40*MapWidth},
		{Player: GodPlayer, Flags: OnMove, Population: 100, Weapons: 1, AtPos: 55 + 40*MapWidth},
	}
	for i, peep := range w.Peeps {
		w.MapWho[peep.AtPos] = byte(i + 1)
	}
	beforeRNG := w.rng
	x, y, ok := w.advancedKnightDefenseTarget(GodPlayer)
	if !ok || x != 11 || y != 10 {
		t.Fatalf("defense target=(%d,%d,%t), want next step (11,10,true)", x, y, ok)
	}
	if w.rng != beforeRNG || w.Magnets[GodPlayer].Mana != ManaSwampCost*2 {
		t.Fatal("threat scoring consumed RNG or mana")
	}
}

func TestAdvancedKnightDefenseRequiresRealFriendlyTarget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headFor int
		target  Peep
	}{
		{name: "no assigned target", headFor: 0, target: Peep{Player: GodPlayer, Flags: InTown, Population: 500}},
		{name: "enemy target", headFor: 2, target: Peep{Player: DevilPlayer, Flags: InTown, Population: 500}},
		{name: "dead target", headFor: 2, target: Peep{Player: GodPlayer, Flags: InTown}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedKnightDefenseWorld()
			knightPos, targetPos := 10+10*MapWidth, 20+10*MapWidth
			tc.target.AtPos = targetPos
			w.Peeps = []Peep{{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: tc.headFor, AtPos: knightPos}, tc.target}
			w.MapWho[knightPos], w.MapWho[targetPos] = 1, 2
			before := w.StateHash()
			if w.advancedKnightDefense(GodPlayer) || w.StateHash() != before {
				t.Fatal("defense acted without a living friendly directed target")
			}
		})
	}
}

func TestAdvancedKnightDefenseHonoursPowerRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode int
		mana int
		war  bool
	}{
		{name: "power locked", mode: computerLand, mana: ManaSwampCost},
		{name: "insufficient mana", mode: computerSwamp, mana: ManaSwampCost - 1},
		{name: "armageddon", mode: computerSwamp, mana: ManaSwampCost, war: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := advancedKnightDefenseWorld()
			w.Computer[GodPlayer].Mode = tc.mode
			w.Magnets[GodPlayer].Mana = tc.mana
			w.War = tc.war
			knightPos, targetPos := 10+10*MapWidth, 20+10*MapWidth
			w.Peeps = []Peep{
				{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: knightPos},
				{Player: GodPlayer, Flags: InTown, Population: 500, AtPos: targetPos},
			}
			w.MapWho[knightPos], w.MapWho[targetPos] = 1, 2
			before := w.StateHash()
			if w.advancedKnightDefense(GodPlayer) || w.StateHash() != before {
				t.Fatal("defense ignored a normal power restriction")
			}
		})
	}
}

func TestAdvancedKnightDefenseAvoidsFriendlyCollateral(t *testing.T) {
	w := advancedKnightDefenseWorld()
	knightPos, targetPos, bystanderPos := 10+10*MapWidth, 20+10*MapWidth, 12+10*MapWidth
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: knightPos},
		{Player: GodPlayer, Flags: InTown, Population: 500, AtPos: targetPos},
		{Player: GodPlayer, Flags: OnMove, Population: 200, AtPos: bystanderPos},
	}
	for i, peep := range w.Peeps {
		w.MapWho[peep.AtPos] = byte(i + 1)
	}
	before := w.StateHash()
	if w.advancedKnightDefense(GodPlayer) || w.StateHash() != before {
		t.Fatal("defensive swamp endangered a friendly bystander")
	}
}

func TestAdvancedKnightDefenseDoesNotSpamExistingSwampZone(t *testing.T) {
	w := advancedKnightDefenseWorld()
	knightPos, targetPos := 10+10*MapWidth, 20+10*MapWidth
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: knightPos},
		{Player: GodPlayer, Flags: InTown, Population: 500, AtPos: targetPos},
	}
	w.MapWho[knightPos], w.MapWho[targetPos] = 1, 2
	w.MapBlk[12+10*MapWidth] = SwampBlock
	before := w.StateHash()
	if w.advancedKnightDefense(GodPlayer) || w.StateHash() != before {
		t.Fatal("defense recast swamp over an already protected path")
	}
}

func TestAdvancedKnightDefenseDoesNotRecastItsOwnZone(t *testing.T) {
	w := advancedKnightDefenseWorld()
	w.Magnets[GodPlayer].Mana = 2 * ManaSwampCost
	knightPos, targetPos := 10+10*MapWidth, 20+10*MapWidth
	w.Peeps = []Peep{
		{Player: DevilPlayer, Flags: OnMove, Population: 4000, Weapons: 5, Status: KnightStatus, HeadFor: 2, AtPos: knightPos},
		{Player: GodPlayer, Flags: InTown, Population: 500, AtPos: targetPos},
	}
	w.MapWho[knightPos], w.MapWho[targetPos] = 1, 2
	if !w.advancedKnightDefense(GodPlayer) {
		t.Fatal("first defensive swamp was rejected")
	}
	afterFirst := w.StateHash()
	if w.Magnets[GodPlayer].Mana != ManaSwampCost {
		t.Fatalf("mana after first defense=%d, want one remaining cast", w.Magnets[GodPlayer].Mana)
	}
	if w.advancedKnightDefense(GodPlayer) || w.StateHash() != afterFirst {
		t.Fatal("defense spent a second action on its existing swamp zone")
	}
}
