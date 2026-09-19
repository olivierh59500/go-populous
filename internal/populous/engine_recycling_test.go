package populous

import (
	"fmt"
	"reflect"
	"testing"
)

func TestTownEmigrationRecyclesFirstDeadSlot(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, size := range []int{8, MaxPeeps} {
			for _, occupied := range []bool{false, true} {
				t.Run(fmt.Sprintf("player%d/size%d/occupied%t", player, size, occupied), func(t *testing.T) {
					w := &World{Peeps: make([]Peep, size, MaxPeeps)}
					for i := range w.Peeps {
						w.Peeps[i] = Peep{Flags: OnMove, Player: byte(player), Population: 1, AtPos: 100 + i}
					}
					// A dead slot can retain nonzero fields and expired ruin map
					// references. None must be inherited by the new emigrant.
					w.Peeps[1] = Peep{Flags: InRuin, Player: byte(player ^ 1), IQ: 7, Weapons: 6, Population: -1, BattlePopulation: 9, AtPos: 700, Direction: 1, Frame: 12, HeadFor: 4, InOut: 699, Status: KnightStatus, LandComplete: true, MagnetLastMove: 31}
					w.Peeps[3].Population = 0
					w.Peeps[5] = Peep{Flags: InTown, Player: byte(player), Population: 200, AtPos: 650, IQ: 3, Weapons: 5}
					w.MapWho[699], w.MapWho[700], w.MapWho[701] = 2, 2, 5
					w.MapWho[650] = 6
					if occupied {
						w.MapWho[650] = 5
					}
					w.Magnets[player].Carried, w.Magnets[player^1].Carried = 6, 2
					population := w.PlayerPopulations()[player]

					w.spawnWalkerFromTown(5, 100)

					want := Peep{Flags: OnMove, Player: byte(player), IQ: 3, Weapons: 5, Population: 150, AtPos: 650, InOut: 650}
					if w.Peeps[1] != want {
						t.Fatalf("recycled walker = %+v, want %+v", w.Peeps[1], want)
					}
					if len(w.Peeps) != size || w.Peeps[3].Population != 0 || w.Peeps[5].Population != 50 || w.Peeps[5].IQ != 4 {
						t.Fatalf("wrong slot or town split: size=%d second dead=%d town=%+v", len(w.Peeps), w.Peeps[3].Population, w.Peeps[5])
					}
					if got := w.PlayerPopulations()[player]; got != population {
						t.Fatalf("emigration created/lost people: %d -> %d", population, got)
					}
					wantOccupant := byte(2)
					if occupied {
						wantOccupant = 5
					}
					if w.MapWho[650] != wantOccupant || w.MapWho[699] != 0 || w.MapWho[700] != 0 || w.MapWho[701] != 5 {
						t.Fatalf("invalid map references: new=%d old=%d/%d unrelated=%d", w.MapWho[650], w.MapWho[699], w.MapWho[700], w.MapWho[701])
					}
					if w.Magnets[player].Carried != 2 || w.Magnets[player^1].Carried != 0 {
						t.Fatalf("wrong carriers after reuse: %+v", w.Magnets)
					}
				})
			}
		}
	}
}

func TestTownEmigrationAtFullLivingCapacityDoesNotChangeTown(t *testing.T) {
	w := &World{Peeps: make([]Peep, MaxPeeps)}
	for i := range w.Peeps {
		w.Peeps[i] = Peep{Flags: InTown, Player: byte(i & 1), Population: 200, IQ: 1, AtPos: 650 + i}
	}
	w.Magnets[0].Carried, w.Magnets[1].Carried = 1, 2
	w.MapWho[650], w.MapWho[651] = 1, 2
	before := w.Snapshot()
	for player := 0; player < 2; player++ {
		w.spawnWalkerFromTown(player, 100)
	}
	if !reflect.DeepEqual(before, w.Snapshot()) {
		t.Fatal("failed emigration changed a full living world")
	}
}

func TestTownEmigrationAppendPreservesTownAndCarrier(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := &World{Peeps: []Peep{{Flags: InTown, Player: byte(player), Population: 200, AtPos: 650, IQ: MaxMensa, Weapons: 2}}}
		w.Magnets[player].Carried = 1
		w.MapWho[650] = 1
		oldTown := &w.Peeps[0]
		w.spawnWalkerFromTown(0, 100)
		if &w.Peeps[0] == oldTown {
			t.Fatal("fixture did not force slice reallocation")
		}
		if len(w.Peeps) != 2 || w.Peeps[0].Population != 50 || w.Peeps[1].Population != 150 || w.Peeps[0].IQ != MaxMensa || w.Peeps[1].Player != byte(player) || w.MapWho[650] != 2 || w.Magnets[player].Carried != 2 {
			t.Fatalf("player %d append broke split/references: peeps=%+v carrier=%d map=%d", player, w.Peeps, w.Magnets[player].Carried, w.MapWho[650])
		}
	}
}

func TestKnightCrossesFriendlyTownWithoutMerging(t *testing.T) {
	for player := 0; player < 2; player++ {
		w := &World{Peeps: []Peep{
			{Flags: OnMove, Player: byte(player), Population: 1500, AtPos: 650, HeadFor: 3, Status: KnightStatus},
			{Flags: InTown, Player: byte(player), Population: 100, AtPos: 650},
			{Flags: InTown, Player: byte(player ^ 1), Population: 100, AtPos: 654},
		}}
		for i := range w.MapBlk {
			w.MapBlk[i] = FlatBlock
		}
		w.MapWho[650], w.MapWho[654] = 2, 3
		w.moveExplorer(0)
		if w.Peeps[0].AtPos != 651 || w.Peeps[0].Population != 1500 || w.Peeps[1].Population != 100 || w.MapWho[650] != 2 || w.Peeps[0].HeadFor != 3 {
			t.Fatalf("player %d knight failed to cross friendly town: peeps=%+v town map=%d", player, w.Peeps, w.MapWho[650])
		}
	}
}

func TestContactStillStopsMergedWalkersAndStartsEnemyBattles(t *testing.T) {
	for player := 0; player < 2; player++ {
		for _, enemy := range []bool{false, true} {
			w := &World{Peeps: []Peep{
				{Flags: OnMove, Player: byte(player), Population: 150, AtPos: 650},
				{Flags: InTown, Player: byte(player), Population: 100, AtPos: 650},
			}}
			w.MapWho[650] = 2
			w.Magnets[player].Carried = 1
			if enemy {
				w.Peeps[0].Status, w.Peeps[0].HeadFor = KnightStatus, 2
				w.Peeps[1].Player ^= 1
			}
			if w.resolveContact(0, 1) {
				t.Fatalf("player %d enemy=%t contact unexpectedly permits movement", player, enemy)
			}
			if enemy {
				if w.Peeps[0].Flags != InBattle || w.Peeps[1].Flags != InBattle|InTown || w.Peeps[0].BattlePopulation != 1 || w.Peeps[1].BattlePopulation != 0 || w.Peeps[0].Population != 150 || w.Peeps[1].Population != 100 || w.MapWho[650] != 1 {
					t.Fatalf("player %d enemy contact no longer starts a battle: %+v", player, w.Peeps)
				}
			} else if w.Peeps[0].Population != 0 || w.Peeps[1].Population != 250 || w.Magnets[player].Carried != 2 || w.MapWho[650] != 2 {
				t.Fatalf("player %d ordinary merger changed: %+v", player, w.Peeps)
			}
		}
	}
}

func TestRecycledPeopleStayDeterministicAcrossTicksAndSnapshot(t *testing.T) {
	w := GenerateWorld(Level{SeedOffset: 25, PlayerPopulation: 1, EnemyPopulation: 1})
	for i := range w.Alt {
		w.Alt[i] = 1
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
	w.MapWho = [MapWidth * MapHeight]byte{}
	w.MapBk2 = [MapWidth * MapHeight]byte{}
	w.Peeps = make([]Peep, 4)
	for player := 0; player < 2; player++ {
		index, pos := player*2+1, 650+player*1950
		w.Peeps[index] = Peep{Flags: InTown, Player: byte(player), Population: CityFood + 100, AtPos: pos, IQ: 1, Weapons: 1}
		w.Magnets[player].Carried = index + 1
		w.MapWho[pos] = byte(index + 1)
	}
	w.GameTurn = 7
	right := WorldFromSnapshot(w.Snapshot(), w.Rules)
	for tick := 0; tick < 128; tick++ {
		w.TickWithComputer([2]bool{})
		right.TickWithComputer([2]bool{})
		if tick == 0 {
			if len(w.Peeps) != 4 || w.Peeps[0].Population <= 0 || w.Peeps[2].Population <= 0 || w.Peeps[0].Player != GodPlayer || w.Peeps[2].Player != DevilPlayer {
				t.Fatalf("fixture did not recycle both slots in Tick: %+v", w.Peeps)
			}
		}
		if tick == 31 {
			right = WorldFromSnapshot(right.Snapshot(), right.Rules)
		}
		if w.StateHash() != right.StateHash() {
			t.Fatalf("recycled people diverged at tick %d", tick)
		}
	}
}
