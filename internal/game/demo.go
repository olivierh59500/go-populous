package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous/internal/attract"
	"go-populous/internal/populous"
)

const (
	demoGutter     = 8
	demoHeader     = 20
	demoWidth      = 2*logicalWidth + demoGutter
	demoHeight     = demoHeader + logicalHeight + 36
	demoRoundLimit = 20 * 60 * simulationTPS
)

type demoPlayback struct {
	session  *attract.Session
	views    [2]*Game
	frame    *ebiten.Image
	tick     int
	round    int
	selector *attract.Selector
}

// SetDemoSeed selects a repeatable world order. Zero uses a fresh random seed.
// Configure before starting playback or recording.
func (g *Game) SetDemoSeed(seed uint64) {
	g.demoSeed = seed
	if g.demo != nil {
		g.demo.selector = nil
	}
}

// SetDemoWorld selects one original world for reproduction; -1 restores random
// selection. Demo levels are kept separate from mutable custom-game options.
func (g *Game) SetDemoWorld(index int) error {
	if index < -1 || index >= len(g.demoLevels) {
		return fmt.Errorf("demo world index %d outside -1..%d", index, len(g.demoLevels)-1)
	}
	g.demoWorldIndex = index
	return nil
}

// Input is sampled at the frontend's rate and latched until a simulation step,
// including on mobile where a short tap can fit between two 8 Hz steps.
type demoInputState struct {
	keys        []ebiten.Key
	justKeys    []ebiten.Key
	activity    bool
	dismiss     bool
	start       bool
	cursorKnown bool
	cursorX     int
	cursorY     int
}

// SetDemoDelay selects the title screen's inactivity timeout. A non-positive
// delay disables automatic demos; F3 can still start one manually.
func (g *Game) SetDemoDelay(delay time.Duration) {
	ticks := 0
	if delay > 0 {
		// Ceiling division also makes positive delays below one tick useful.
		period := time.Second / simulationTPS
		ticks = int(delay / period)
		if delay%period != 0 {
			ticks++
		}
	}
	g.demoIdle = attract.NewIdle(ticks)
}

// StartDemo starts an isolated spectator match. It never replaces the user's
// world, custom options, save file, or multiplayer session.
func (g *Game) StartDemo() {
	if g.multiplayerEnabled() || (g.state != StateTitle && g.state != StateDemo) {
		return
	}
	if g.demo == nil {
		g.demo = &demoPlayback{frame: ebiten.NewImage(demoWidth, demoHeight)}
		for player := range g.demo.views {
			g.demo.views[player] = g.newDemoView(player)
		}
	}
	g.startDemoRound()
	g.state = StateDemo
	g.demoInput.activity = false
	g.demoInput.dismiss = false
	g.demoInput.start = false
	if g.demoIdle != nil {
		g.demoIdle.Reset()
	}
	if g.sound != nil {
		g.sound.ResetHeartbeat()
	}
}

func (g *Game) startDemoRound() {
	if g.demo.selector == nil {
		seed := g.demoSeed
		if seed == 0 {
			var raw [8]byte
			if _, err := rand.Read(raw[:]); err == nil {
				seed = binary.LittleEndian.Uint64(raw[:])
			} else {
				seed = uint64(time.Now().UnixNano())
			}
		}
		g.demo.selector = attract.NewSelector(g.demoLevels, seed)
	}
	level, advancedPlayer := g.demo.selector.Next()
	if g.demoWorldIndex >= 0 {
		level = g.demoLevels[g.demoWorldIndex]
	}
	rules := populous.DefaultTerrainRules()
	terrain := int(level.Terrain)
	if terrain >= 0 && terrain < len(g.bundle.TerrainRules) {
		rules = g.bundle.TerrainRules[terrain]
	}
	g.demo.session = attract.NewLevelSession(level, rules, g.demo.round, advancedPlayer)
	// Some original profiles cannot break a stalemate. Unattended playback
	// eventually rotates those worlds; recording explicitly disables this cap.
	g.demo.session.LimitTicks = demoRoundLimit
	g.demo.tick = -1
}

func (g *Game) stopDemo() {
	g.demo.session = nil
	for _, view := range g.demo.views {
		view.world = nil
	}
	g.state = StateTitle
	g.frameCacheValid = false
	g.demoIdle.Reset()
	g.demoInput.activity = false
	g.demoInput.dismiss = false
	// Consume the dismissing touch, so it cannot also activate a menu item.
	g.touch = touchInputState{}
}

func (g *Game) sampleDemoInput() {
	input := &g.demoInput
	input.keys = inpututil.AppendPressedKeys(input.keys[:0])
	input.justKeys = inpututil.AppendJustPressedKeys(input.justKeys[:0])
	input.start = input.start || (g.state == StateTitle && inpututil.IsKeyJustPressed(ebiten.KeyF3))
	held := len(input.keys) > 0 || len(g.touch.contacts) > 0
	pressed := len(input.justKeys) > 0 || len(g.touch.justIDs) > 0
	for _, button := range [...]ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight, ebiten.MouseButtonMiddle} {
		held = held || ebiten.IsMouseButtonPressed(button)
		pressed = pressed || inpututil.IsMouseButtonJustPressed(button)
	}
	wheelX, wheelY := ebiten.Wheel()
	pressed = pressed || wheelX != 0 || wheelY != 0
	x, y := ebiten.CursorPosition()
	moved := input.cursorKnown && (x != input.cursorX || y != input.cursorY)
	input.cursorX, input.cursorY, input.cursorKnown = x, y, true
	input.activity = input.activity || held || pressed || moved
	// Motion postpones the demo on the title, but only deliberate input exits
	// playback. In particular, changing Layout must not act like a mouse move.
	input.dismiss = input.dismiss || pressed
}

// updateDemo runs before normal input handling, preventing spectator input
// from issuing game commands or activating the menu on the same tick.
func (g *Game) updateDemo() bool {
	activity, dismiss, start := g.demoInput.activity, g.demoInput.dismiss, g.demoInput.start
	g.demoInput.activity, g.demoInput.dismiss, g.demoInput.start = false, false, false
	if g.state == StateDemo {
		if dismiss {
			g.stopDemo()
			return true
		}
		g.demo.session.Tick()
		// Attract mode is quiet; drain effects so events cannot accumulate.
		g.demo.session.World.DrainSoundEvents()
		if g.demo.session.ShouldRestart() {
			g.demo.round++
			g.startDemoRound()
		}
		return true
	}
	if g.demoIdle == nil {
		return false
	}
	if g.state != StateTitle || g.multiplayerEnabled() {
		g.demoIdle.Reset()
		return false
	}
	if start {
		g.StartDemo()
		return true
	}
	if g.demoIdle.Tick(activity) {
		g.StartDemo()
		return true
	}
	return false
}

func (g *Game) drawDemoHint(screen *ebiten.Image) {
	label := "F3 DEMO"
	if g.demoIdle != nil && g.demoIdle.LimitTicks > 0 {
		seconds := max(0, g.demoIdle.LimitTicks-g.demoIdle.ElapsedTicks+simulationTPS-1) / simulationTPS
		label = fmt.Sprintf("F3 DEMO %ds", seconds)
	}
	ebitenutil.DebugPrintAt(screen, label, logicalWidth-8-len(label)*6, 216)
}

// A view borrows immutable graphics, but owns its viewport and minimap cache.
// The user's Game and World remain untouched while either camera is rendered.
func (g *Game) newDemoView(player int) *Game {
	return &Game{
		bundle: g.bundle, images: g.images, landFrames: g.landFrames,
		spriteFrames: g.spriteFrames, bigSpriteFrames: g.bigSpriteFrames,
		panel: g.panel, player: player, viewPeep: -1,
		computerControlled: [2]bool{true, true},
		frameCache:         ebiten.NewImage(logicalWidth, logicalHeight),
		miniMap:            ebiten.NewImage(miniMapWidth, miniMapHeight),
		miniPixels:         make([]byte, miniMapWidth*miniMapHeight*4),
	}
}

func (g *Game) drawDemo(screen *ebiten.Image) {
	if g.demo == nil || g.demo.session == nil {
		screen.Clear()
		return
	}
	if g.demo.tick != g.tick {
		g.renderDemoFrame()
		g.demo.tick = g.tick
	}
	screen.DrawImage(g.demo.frame, nil)
	if g.recording != nil {
		g.captureDemoRecordingFrame()
	}
}

func (g *Game) renderDemoFrame() {
	demo := g.demo
	session := demo.session
	demo.frame.Fill(color.RGBA{R: 12, G: 15, B: 22, A: 255})
	for player, view := range demo.views {
		camera := session.Cameras[player]
		view.world, view.tick = session.World, g.tick
		view.xoff, view.yoff, view.viewPeep = camera.X, camera.Y, camera.Peep
		view.miniMapDirty = true
		view.frameCache.Clear()
		if background := view.images["qaz"]; background != nil {
			view.frameCache.DrawImage(background, nil)
		}
		view.drawWorld(view.frameCache)
		view.drawMiniMap(view.frameCache)
		view.drawInterfaceGauges(view.frameCache)
		view.drawViewedPeepStatus(view.frameCache)
		view.drawPanel(view.frameCache)
		summary := session.World.SummaryFor(player)
		ebitenutil.DebugPrintAt(view.frameCache, fmt.Sprintf("POP %d  MANA %d", summary.Population, session.World.Magnets[player].Mana), 6, 202)
		ebitenutil.DebugPrintAt(view.frameCache, fmt.Sprintf("TOWNS %d  CASTLES %d  KNIGHTS %d", summary.Towns, summary.Castles, summary.Knights), 6, 214)
		ebitenutil.DebugPrintAt(view.frameCache, "FOLLOWING: "+session.FocusLabel(player), 6, 226)

		x := player * (logicalWidth + demoGutter)
		accent := color.RGBA{R: 70, G: 142, B: 240, A: 255}
		side := "BLUE"
		if player == populous.DevilPlayer {
			accent = color.RGBA{R: 231, G: 80, B: 71, A: 255}
			side = "RED"
		}
		ai := "ORIGINAL AI"
		if player == session.AdvancedPlayer {
			ai = "STRATEGIC AI"
		}
		ebitenutil.DrawRect(demo.frame, float64(x), 0, logicalWidth, 2, accent)
		ebitenutil.DebugPrintAt(demo.frame, side+" / "+ai, x+8, 5)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), demoHeader)
		demo.frame.DrawImage(view.frameCache, op)
	}
	seconds := session.ElapsedTicks / simulationTPS
	level := session.World.Level
	profile := "SPELLS"
	if level.EnemyPowers == 0 {
		profile = "NO SPELLS"
	}
	ebitenutil.DebugPrintAt(demo.frame, fmt.Sprintf("WORLD %03d %s  %s  ORIGINAL: %s", level.Number, level.Code, terrainName(level.Terrain), profile), 8, demoHeight-32)
	status := fmt.Sprintf("DEMO %02d   %02d:%02d", session.Round+1, seconds/60, seconds%60)
	if session.Finished {
		status += "   " + session.Outcome
	}
	ebitenutil.DebugPrintAt(demo.frame, status, 8, demoHeight-20)
	hint := "KEY / CLICK / TOUCH: MENU"
	if g.recording != nil {
		hint = "AI VS AI / FULL MATCH"
	}
	ebitenutil.DebugPrintAt(demo.frame, hint, demoWidth-152, demoHeight-20)
}
