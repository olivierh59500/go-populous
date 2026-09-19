package attract

import "go-populous/internal/populous"

const (
	ResultHoldTicks = 8 * TicksPerSecond
	cameraDwell     = 8 * TicksPerSecond
	cameraMinDwell  = 2 * TicksPerSecond
	cameraViewSize  = 8
)

// Camera follows one camp in map coordinates. Peep is a zero-based index into
// World.Peeps, or -1 when no living follower remains.
type Camera struct {
	X, Y int
	Peep int
}

// Session owns a separate world; it never receives or mutates a player's world.
// Rounds normally run until elimination. An optional time limit is not a win:
// Winner remains -1, and Outcome explains why the round stopped.
type Session struct {
	World          *populous.World
	AdvancedPlayer int
	Cameras        [2]Camera
	Round          int
	ElapsedTicks   int
	LimitTicks     int // Zero disables the time limit, including during recording.
	Finished       bool
	Winner         int
	Outcome        string

	cameraAge [2]int
	resultAge int
}

// NewLevelSession preserves the selected campaign level's terrain, population,
// mana, powers, and historical computer settings. The strategic computer takes
// the specified side; callers normally use GodPlayer so that the original
// opponent keeps its campaign-specific restrictions and difficulty. Campaign
// levels are intentionally asymmetric, unlike the balanced AI benchmark.
func NewLevelSession(level populous.Level, rules populous.TerrainRules, round, advancedPlayer int) *Session {
	if advancedPlayer != populous.GodPlayer && advancedPlayer != populous.DevilPlayer {
		advancedPlayer = populous.GodPlayer
	}
	session := &Session{
		World:          populous.GenerateWorldWithRules(level, rules),
		AdvancedPlayer: advancedPlayer,
		Round:          max(0, round),
		Winner:         -1,
	}
	session.World.ComputerControlled = [2]bool{true, true}
	for player := range session.Cameras {
		session.Cameras[player].Peep = session.chooseFocus(player, -1)
		x, y := session.focusPosition(player)
		session.Cameras[player].X = x
		session.Cameras[player].Y = y
	}
	return session
}

// Tick advances at the normal game's eight ticks per second. Finished worlds
// remain frozen while the result is shown; the UI starts a fresh round once
// ShouldRestart returns true.
func (session *Session) Tick() {
	if session.Finished {
		if session.resultAge < ResultHoldTicks {
			session.resultAge++
		}
		return
	}
	// Check before advancing too, so elimination never grants an extra AI turn.
	if session.checkElimination() {
		return
	}
	session.World.TickWithAdvancedComputer(session.AdvancedPlayer)
	session.ElapsedTicks++
	for player := range session.Cameras {
		session.updateCamera(player)
	}
	if session.checkElimination() {
		return
	}
	if session.LimitTicks > 0 && session.ElapsedTicks >= session.LimitTicks {
		session.Finished = true
		session.Outcome = "TIME LIMIT"
	}
}

func (session *Session) ShouldRestart() bool {
	return session.Finished && session.resultAge >= ResultHoldTicks
}

func (session *Session) checkElimination() bool {
	var alive [2]bool
	for _, peep := range session.World.Peeps {
		if peep.Population > 0 && peep.Flags&populous.InRuin == 0 && peep.Player < 2 {
			alive[peep.Player] = true
		}
	}
	if alive[0] && alive[1] {
		return false
	}
	session.Finished = true
	switch {
	case alive[populous.GodPlayer]:
		session.Winner = populous.GodPlayer
		session.Outcome = "BLUE WINS"
	case alive[populous.DevilPlayer]:
		session.Winner = populous.DevilPlayer
		session.Outcome = "RED WINS"
	default:
		session.Outcome = "DRAW"
	}
	return true
}

func (session *Session) updateCamera(player int) {
	camera := &session.Cameras[player]
	currentScore := session.focusScore(player, camera.Peep)
	if currentScore >= 0 {
		pos := session.World.Peeps[camera.Peep].AtPos
		x, y := pos%populous.MapWidth, pos/populous.MapWidth
		if x >= camera.X && x < camera.X+cameraViewSize && y >= camera.Y && y < camera.Y+cameraViewSize {
			// Travel time does not consume the dwell: distant targets should
			// actually reach the screen before another settlement is selected.
			session.cameraAge[player]++
		}
	}
	next := session.chooseFocus(player, -1)
	if currentScore < 0 || session.cameraAge[player] >= cameraDwell {
		// A small penalty lets ordinary town/leader views alternate after the
		// dwell, while an ongoing battle remains the most interesting focus.
		next = session.chooseFocus(player, camera.Peep)
		camera.Peep = next
		session.cameraAge[player] = 0
	} else if session.cameraAge[player] >= cameraMinDwell && session.focusScore(player, next) >= currentScore+2500 {
		camera.Peep = next
		session.cameraAge[player] = 0
	}
	// Pan at four map tiles per second instead of jumping across the map.
	if session.ElapsedTicks%2 == 0 {
		x, y := session.focusPosition(player)
		camera.X = approach(camera.X, x)
		camera.Y = approach(camera.Y, y)
	}
}

func (session *Session) chooseFocus(player, previous int) int {
	best, bestScore := -1, -1
	for index := range session.World.Peeps {
		score := session.focusScore(player, index)
		if score < 0 {
			continue
		}
		if index == previous {
			score -= 2000
		}
		if best < 0 || score > bestScore {
			best, bestScore = index, score
		}
	}
	return best
}

func (session *Session) focusScore(player, index int) int {
	if index < 0 || index >= len(session.World.Peeps) {
		return -1
	}
	peep := session.World.Peeps[index]
	if peep.Population <= 0 || peep.Flags&populous.InRuin != 0 || int(peep.Player) != player || peep.AtPos < 0 || peep.AtPos >= populous.MapWidth*populous.MapHeight {
		return -1
	}
	switch {
	case peep.Flags&populous.InBattle != 0:
		return 10000
	case peep.HeadFor != 0:
		return 7000
	case session.World.Magnets[player].Carried == index+1:
		return 3000
	case peep.Flags&populous.InTown != 0:
		return 2000 + min(peep.Population/10, 800)
	default:
		return 1000
	}
}

func (session *Session) focusPosition(player int) (int, int) {
	pos := session.World.Magnets[player].GoTo
	if index := session.Cameras[player].Peep; session.focusScore(player, index) >= 0 {
		pos = session.World.Peeps[index].AtPos
	}
	x := max(0, min(pos%populous.MapWidth-cameraViewSize/2, populous.MapWidth-cameraViewSize))
	y := max(0, min(pos/populous.MapWidth-cameraViewSize/2, populous.MapHeight-cameraViewSize))
	return x, y
}

// FocusLabel is short enough for the split-screen HUD.
func (session *Session) FocusLabel(player int) string {
	if player < 0 || player >= len(session.Cameras) {
		return "FOLLOWERS"
	}
	switch session.focusScore(player, session.Cameras[player].Peep) {
	case 10000:
		return "BATTLE"
	case 7000:
		return "KNIGHT"
	case 3000:
		return "LEADER"
	default:
		if session.focusScore(player, session.Cameras[player].Peep) >= 2000 {
			return "TOWN"
		}
		return "FOLLOWERS"
	}
}

func approach(current, target int) int {
	if current < target {
		return current + 1
	}
	if current > target {
		return current - 1
	}
	return current
}
