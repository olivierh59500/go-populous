package attract

import (
	"testing"

	"go-populous/internal/populous"
)

func newTestSession(round int) *Session {
	level := populous.TutorialLevel()
	level.Number = round
	level.SeedOffset += uint16(round * 7919)
	return NewLevelSession(level, populous.DefaultTerrainRules(), round, round&1)
}

func TestLevelSessionPreservesCampaignRulesAndResources(t *testing.T) {
	levels := campaignLevels(t)
	for _, index := range []int{0, len(levels) / 3, len(levels) / 2, len(levels) - 1} {
		level := levels[index]
		rules := populous.DefaultTerrainRules()
		reference := populous.GenerateWorldWithRules(level, rules)
		reference.ComputerControlled = [2]bool{true, true}
		session := NewLevelSession(level, rules, 5, populous.GodPlayer)
		if session.World.StateHash() != reference.StateHash() {
			t.Fatalf("level %d demo changed original world, mana, people or computer settings", level.Number)
		}
		if session.AdvancedPlayer != populous.GodPlayer || session.Round != 5 || session.LimitTicks != 0 {
			t.Fatalf("unexpected campaign session defaults: %+v", session)
		}
		assertCameras(t, session)
	}
}

func TestLevelSessionContinuesUntilEliminationUnlessLimitRequested(t *testing.T) {
	session := NewLevelSession(populous.TutorialLevel(), populous.DefaultTerrainRules(), 0, populous.GodPlayer)
	const formerLimit = 4 * 60 * TicksPerSecond
	session.ElapsedTicks = formerLimit
	session.Tick()
	if session.Finished || session.ElapsedTicks != formerLimit+1 {
		t.Fatal("complete demo stopped at former four-minute limit")
	}
	session.LimitTicks = session.ElapsedTicks + 1
	session.Tick()
	if !session.Finished || session.Winner != -1 || session.Outcome != "TIME LIMIT" {
		t.Fatal("explicit limit did not stop without declaring a false win")
	}
}

func TestSessionIsDeterministicAndIsolated(t *testing.T) {
	left := newTestSession(0)
	right := newTestSession(0)
	untouched := newTestSession(1)
	untouchedHash := untouched.World.StateHash()
	for tick := range 320 {
		previous := left.Cameras
		left.Tick()
		right.Tick()
		assertCameras(t, left)
		for player, camera := range left.Cameras {
			if abs(camera.X-previous[player].X) > 1 || abs(camera.Y-previous[player].Y) > 1 {
				t.Fatalf("tick %d camera jumped: previous=%+v current=%+v", tick, previous[player], camera)
			}
		}
		if left.Cameras != right.Cameras || left.Outcome != right.Outcome {
			t.Fatalf("presentation diverged at tick %d", tick)
		}
	}
	if left.World.StateHash() != right.World.StateHash() {
		t.Fatal("identical demo rounds diverged")
	}
	if untouched.World.StateHash() != untouchedHash || untouched.ElapsedTicks != 0 {
		t.Fatal("running a demo modified an unrelated world")
	}
	if left.ElapsedTicks == 0 || left.World.GameTurn != left.ElapsedTicks {
		t.Fatal("demo did not advance the world at one turn per tick")
	}
}

func TestEliminationFreezesRoundThenRequestsRestart(t *testing.T) {
	session := newTestSession(0)
	for i := range session.World.Peeps {
		if session.World.Peeps[i].Player == populous.DevilPlayer {
			// Ruins have positive population but are not surviving followers.
			session.World.Peeps[i].Flags = populous.InRuin
		}
	}
	session.Tick()
	if !session.Finished || session.Winner != populous.GodPlayer || session.Outcome != "BLUE WINS" {
		t.Fatalf("elimination did not finish round: winner=%d outcome=%q", session.Winner, session.Outcome)
	}
	hash := session.World.StateHash()
	for range ResultHoldTicks - 1 {
		session.Tick()
		if session.ShouldRestart() {
			t.Fatal("result was not held for its full duration")
		}
	}
	session.Tick()
	if !session.ShouldRestart() {
		t.Fatal("finished round did not request restart")
	}
	if session.World.StateHash() != hash {
		t.Fatal("world continued running underneath result overlay")
	}
}

func TestRoundDurationIsBoundedWithoutDeclaringFalseWinner(t *testing.T) {
	session := newTestSession(0)
	session.LimitTicks = 60 * TicksPerSecond
	session.ElapsedTicks = session.LimitTicks - 1
	session.Tick()
	if !session.Finished || session.Winner != -1 || session.Outcome != "TIME LIMIT" {
		t.Fatalf("time limit did not end round honestly: winner=%d outcome=%q", session.Winner, session.Outcome)
	}
	if session.ShouldRestart() {
		t.Fatal("time-limit result was skipped")
	}
}

func TestCameraPrioritizesActionButKeepsMinimumDwell(t *testing.T) {
	session := &Session{
		World: &populous.World{
			Peeps: []populous.Peep{
				{Player: 0, Population: 100, AtPos: 10 + 10*populous.MapWidth, Flags: populous.InTown},
				{Player: 0, Population: 100, AtPos: 55 + 55*populous.MapWidth, Flags: populous.OnMove},
			},
			Magnets: [2]populous.Magnet{{Carried: 1}},
		},
		Cameras: [2]Camera{{X: 6, Y: 6, Peep: 0}, {Peep: -1}},
	}
	if session.FocusLabel(0) != "LEADER" {
		t.Fatal("camera did not identify the leader")
	}
	session.World.Peeps[1].Flags = populous.InBattle
	for range cameraMinDwell - 1 {
		session.ElapsedTicks++
		session.updateCamera(0)
		if session.Cameras[0].Peep != 0 {
			t.Fatal("camera switched before minimum dwell")
		}
	}
	session.ElapsedTicks++
	session.updateCamera(0)
	if session.Cameras[0].Peep != 1 || session.FocusLabel(0) != "BATTLE" {
		t.Fatal("camera did not move toward the battle after minimum dwell")
	}
	if session.Cameras[0].X > 7 || session.Cameras[0].Y > 7 {
		t.Fatal("camera teleported to its new focus")
	}
	// More than a full dwell may be needed to travel across the map. The
	// camera must reach and show the battle before choosing a different view.
	session.World.Peeps[1].Flags = populous.InTown
	for range cameraDwell {
		session.ElapsedTicks++
		session.updateCamera(0)
		if session.Cameras[0].Peep != 1 {
			t.Fatal("camera changed focus before the distant target reached the screen")
		}
	}
	session.World.Peeps[1].Population = 0
	session.updateCamera(0)
	if session.Cameras[0].Peep != 0 {
		t.Fatal("camera kept following a dead follower")
	}
}

func assertCameras(t *testing.T, session *Session) {
	t.Helper()
	for player, camera := range session.Cameras {
		if camera.X < 0 || camera.X > populous.MapWidth-cameraViewSize || camera.Y < 0 || camera.Y > populous.MapHeight-cameraViewSize {
			t.Fatalf("camera %d out of bounds: %+v", player, camera)
		}
		if camera.Peep >= 0 && session.focusScore(player, camera.Peep) < 0 {
			t.Fatalf("camera %d follows an invalid follower: %+v", player, camera)
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
