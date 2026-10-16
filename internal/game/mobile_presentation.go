package game

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous/internal/mobileui"
	"go-populous/internal/populous"
	"go-populous/internal/touchui"
)

type mobileOperation struct {
	command populous.Command
	build   bool
	view    mobileui.Viewport
	scope   mobileui.BuildScope
}

type mobilePresentation struct {
	prefs        mobileui.Preferences
	gesture      mobileui.Gesture
	contacts     []mobileui.Contact
	pending      []mobileOperation
	tool         mobileui.Action
	overlay      mobileui.Overlay
	confirm      mobileui.Action
	status       string
	panX, panY   float64
	preview      mobileui.Point
	hasPreview   bool
	lastState    State
	lastWorld    *populous.World
	lastWidth    int
	lastFocused  bool
	layout       mobileui.HUDLayout
	padTicks     int
	mapCache     *ebiten.Image
	mapCacheTick int
	mapCacheView mobileui.Viewport
	endPanel     *ebiten.Image
	frontPage    mobileui.FrontPage
	frontOffset  int
}

// SetMobileUI enables the separate mobile presentation without changing world
// rules, AI, tick rate or desktop rendering. Android enables it at startup;
// desktop can opt in to exercise the same controls with a mouse.
func (g *Game) SetMobileUI(enabled bool) {
	if !enabled {
		g.mobileUI = nil
		return
	}
	if g.mobileUI == nil {
		g.mobileUI = &mobilePresentation{
			prefs: mobileui.DefaultPreferences(), tool: mobileui.ActionRaise,
			lastState: g.state, lastWorld: g.world, mapCacheTick: -1,
		}
	}
	g.loadMobilePreferences()
}

func (g *Game) mobileSceneActive() bool {
	return g.mobileUI != nil && (g.state == StateGame || g.state == StateEnd)
}

// CancelInput may be called by a platform lifecycle callback on another
// thread. The game loop consumes the request before observing any contacts.
func (g *Game) CancelInput() { g.mobileInputReset.Store(true) }

func (g *Game) mobilePreferencesPath() string {
	if g.savePath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(g.savePath), "go-populous-ui.json")
}

func (g *Game) loadMobilePreferences() {
	if g.mobileUI == nil || g.mobilePreferencesPath() == "" {
		return
	}
	prefs, err := mobileui.LoadPreferences(g.mobilePreferencesPath())
	if err == nil {
		g.mobileUI.prefs = prefs
	} else if !errors.Is(err, os.ErrNotExist) {
		g.mobileUI.status = "UI SETTINGS COULD NOT BE LOADED"
	}
}

func (g *Game) saveMobilePreferences() {
	if path := g.mobilePreferencesPath(); path != "" {
		if err := mobileui.SavePreferences(path, g.mobileUI.prefs); err != nil {
			g.mobileUI.status = "UI SETTINGS NOT SAVED"
		}
	}
}

func (g *Game) cancelMobileInput() {
	if m := g.mobileUI; m != nil {
		m.gesture.Cancel()
		m.pending = m.pending[:0]
		m.hasPreview = false
		m.padTicks = 0
	}
	g.touch.latch = touchui.Latch{}
}

func (g *Game) syncMobilePresentation() {
	m := g.mobileUI
	if m == nil {
		return
	}
	focused := ebiten.IsFocused()
	if g.mobileInputReset.Swap(false) || m.lastState != g.state || m.lastWorld != g.world || m.lastWidth != g.layoutWidth || m.lastFocused != focused {
		g.cancelMobileInput()
		m.mapCacheTick = -1
	}
	if m.lastState != g.state || m.lastWorld != g.world {
		m.overlay, m.confirm = mobileui.OverlayNone, mobileui.ActionNone
		m.panX, m.panY = 0, 0
		g.hoverOK = false
	}
	if m.lastState != g.state && g.state == StateHelp {
		m.frontOffset = 0
	}
	m.lastState, m.lastWorld, m.lastWidth, m.lastFocused = g.state, g.world, g.layoutWidth, focused
	g.mobileLayout()
}

func (g *Game) mobileLayout() mobileui.HUDLayout {
	m := g.mobileUI
	width := max(logicalWidth, g.layoutWidth)
	if m.layout.Width != width || m.layout.Overlay != m.overlay || (m.layout.DPadRect.Empty() == m.prefs.ShowPad && m.overlay == mobileui.OverlayNone) {
		m.layout = mobileui.NewHUDLayout(width, logicalHeight, m.overlay, m.prefs.ShowPad)
	}
	return m.layout
}

func (g *Game) mobileViewport() mobileui.Viewport {
	l := g.mobileLayout()
	v := mobileui.Viewport{Rect: l.MapRect, CenterX: float64(g.xoff) + 3.5 + g.mobileUI.panX, CenterY: float64(g.yoff) + 3.5 + g.mobileUI.panY, Zoom: 1}
	if !g.mobileUI.prefs.Wide {
		v.Limit = image.Rect(g.xoff, g.yoff, g.xoff+8, g.yoff+8)
	}
	return v
}

func (g *Game) moveMobileCamera(view mobileui.Viewport) {
	view = view.Clamp(populous.MapWidth, populous.MapHeight)
	if !g.mobileUI.prefs.Wide {
		view.CenterX = max(3.5, min(59.5, view.CenterX))
		view.CenterY = max(3.5, min(59.5, view.CenterY))
	}
	g.xoff = clampInt(int(math.Floor(view.CenterX-3.5)), 0, populous.MapWidth-8)
	g.yoff = clampInt(int(math.Floor(view.CenterY-3.5)), 0, populous.MapHeight-8)
	g.mobileUI.panX = view.CenterX - float64(g.xoff) - 3.5
	g.mobileUI.panY = view.CenterY - float64(g.yoff) - 3.5
}

func (g *Game) updateMobileInput() {
	layout := g.mobileLayout()
	g.sampleMobileContacts()
	if !ebiten.IsFocused() {
		g.cancelMobileInput()
		return
	}
	g.handleMobileSceneContacts(layout)
}

// Menus and gameplay share the same 60 Hz contact sampling and release-only
// recognizer, including desktop mouse preview and Android ID reuse markers.
func (g *Game) sampleMobileContacts() {
	m := g.mobileUI
	m.contacts = m.contacts[:0]
	for _, contact := range g.touch.contacts {
		started := false
		for _, id := range g.touch.justContactIDs {
			started = started || id == contact.ID
		}
		m.contacts = append(m.contacts, mobileui.Contact{ID: contact.ID, X: float64(contact.X), Y: float64(contact.Y), Started: started})
	}
	// Desktop preview uses exactly the same recognizer. A right-button drag
	// supplies two co-located contacts; a left click supplies a single finger.
	x, y := ebiten.CursorPosition()
	if len(m.contacts) == 0 {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			for _, id := range [...]int{-1001, -1002} {
				m.contacts = append(m.contacts, mobileui.Contact{ID: id, X: float64(x), Y: float64(y), Started: inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)})
			}
		} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			m.contacts = append(m.contacts, mobileui.Contact{ID: -1000, X: float64(x), Y: float64(y), Started: inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)})
		}
	}
}

func (g *Game) handleMobileSceneContacts(layout mobileui.HUDLayout) {
	m := g.mobileUI
	frame := m.gesture.Update(m.contacts, func(p mobileui.Point) mobileui.Region {
		if g.state == StateEnd || g.tutorialPaused {
			return mobileui.RegionUI
		}
		if layout.IsTerrainPoint(int(p.X), int(p.Y)) {
			return mobileui.RegionTerrain
		}
		return mobileui.RegionUI
	})
	m.preview, m.hasPreview = frame.Preview, frame.HasPreview && !frame.Panning
	if frame.Panning {
		g.moveMobileCamera(g.mobileViewport().Pan(frame.Pan.X, frame.Pan.Y))
		g.hoverOK = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.state == StateEnd {
			g.cancelMobileInput()
			g.returnToTitleFromTouch()
			return
		}
		if m.overlay == mobileui.OverlayNone {
			g.setMobileOverlay(mobileui.OverlayMenu)
		} else {
			g.setMobileOverlay(mobileui.OverlayNone)
		}
		return
	}
	// Optional pad repeats at frontend rate, but is never combined with a
	// second contact. Terrain and power taps remain release-only.
	if frame.UIHeld && m.overlay == mobileui.OverlayNone {
		a := layout.Hit(int(frame.UIPoint.X), int(frame.UIPoint.Y))
		if a >= mobileui.ActionPanUp && a <= mobileui.ActionPanRight && a == layout.Hit(int(frame.UIStart.X), int(frame.UIStart.Y)) {
			m.padTicks++
			if m.padTicks == 1 || m.padTicks%6 == 0 {
				g.mobilePad(a)
			}
		} else {
			m.padTicks = 0
		}
	} else {
		m.padTicks = 0
	}
	for _, tap := range frame.Taps {
		if g.state == StateEnd {
			g.cancelMobileInput()
			g.continueEndFromTouch()
			return
		}
		if g.tutorialPaused {
			g.tutorialPaused = false
			g.cancelMobileInput()
			return
		}
		if tap.Region == mobileui.RegionTerrain {
			g.mobileTerrainTap(tap.Point)
			continue
		}
		if layout.IsMiniMapPoint(int(tap.X), int(tap.Y)) && layout.IsMiniMapPoint(int(tap.Start.X), int(tap.Start.Y)) {
			sx, sy := (tap.X-float64(layout.MiniMapRect.Min.X))*2, (tap.Y-float64(layout.MiniMapRect.Min.Y))*2
			if mx, my, ok := miniMapTileAt(int(sx), int(sy)); ok {
				g.centerOnMapTile(mx, my)
			}
			continue
		}
		a := layout.Hit(int(tap.X), int(tap.Y))
		if a != mobileui.ActionNone && a == layout.Hit(int(tap.Start.X), int(tap.Start.Y)) && a < mobileui.ActionPanUp {
			g.mobileAction(a)
		}
	}
}

func (g *Game) mobilePad(action mobileui.Action) {
	v := g.mobileViewport()
	switch action {
	case mobileui.ActionPanUp:
		v.CenterY--
	case mobileui.ActionPanDown:
		v.CenterY++
	case mobileui.ActionPanLeft:
		v.CenterX--
	case mobileui.ActionPanRight:
		v.CenterX++
	}
	g.moveMobileCamera(v)
}

func (g *Game) setMobileOverlay(overlay mobileui.Overlay) {
	g.cancelMobileInput()
	g.mobileUI.overlay = overlay
	g.mobileUI.confirm = mobileui.ActionNone
}

func (g *Game) selectMobileTool(action mobileui.Action) {
	g.setMobileOverlay(mobileui.OverlayNone)
	g.mobileUI.tool = action
	g.mobileUI.status = ""
	mode := ModeSculpt
	switch action {
	case mobileui.ActionMagnet:
		mode = ModeMagnet
	case mobileui.ActionInspect:
		mode = ModeInspect
	case mobileui.ActionSwamp:
		mode = ModeSwamp
	}
	g.setActionMode(mode)
}

func (g *Game) mobileAction(a mobileui.Action) {
	m := g.mobileUI
	switch a {
	case mobileui.ActionRaise, mobileui.ActionLower, mobileui.ActionMagnet, mobileui.ActionInspect:
		g.selectMobileTool(a)
	case mobileui.ActionQuake, mobileui.ActionSwamp, mobileui.ActionVolcano:
		if g.mobilePowerEnabled(a) {
			g.selectMobileTool(a)
		} else {
			m.status = "POWER UNAVAILABLE / NOT ENOUGH MANA"
		}
	case mobileui.ActionPowers:
		g.setMobileOverlay(mobileui.OverlayPowers)
	case mobileui.ActionPeople:
		g.setMobileOverlay(mobileui.OverlayPeople)
	case mobileui.ActionMenu:
		g.setMobileOverlay(mobileui.OverlayMenu)
	case mobileui.ActionSettings:
		g.setMobileOverlay(mobileui.OverlaySettings)
	case mobileui.ActionResume, mobileui.ActionCancel:
		g.setMobileOverlay(mobileui.OverlayNone)
	case mobileui.ActionBack:
		if m.overlay == mobileui.OverlaySettings {
			g.setMobileOverlay(mobileui.OverlayMenu)
		} else {
			g.setMobileOverlay(mobileui.OverlayNone)
		}
	case mobileui.ActionLeader:
		g.centerOnPlayerLeader()
		g.viewPlayerLeaderTemporarily()
	case mobileui.ActionSettle, mobileui.ActionJoin, mobileui.ActionFight, mobileui.ActionFollow:
		mode := populous.SettleMode
		switch a {
		case mobileui.ActionJoin:
			mode = populous.JoinMode
		case mobileui.ActionFight:
			mode = populous.FightMode
		case mobileui.ActionFollow:
			mode = populous.MagnetMode
		}
		g.selectMobileTool(mobileui.ActionRaise)
		g.queueMobileCommand(populous.Command{Kind: populous.CommandSetTendency, Player: g.player, Value: mode}, false)
	case mobileui.ActionFlood, mobileui.ActionArmageddon:
		if g.mobilePowerEnabled(a) {
			g.setMobileOverlay(mobileui.OverlayConfirm)
			m.confirm = a
		} else {
			m.status = "POWER UNAVAILABLE / NOT ENOUGH MANA"
		}
	case mobileui.ActionKnight:
		if g.mobilePowerEnabled(a) {
			g.setMobileOverlay(mobileui.OverlayNone)
			g.queueMobileCommand(populous.Command{Kind: populous.CommandKnight, Player: g.player}, false)
		} else {
			m.status = "NEED A LEADER AND ENOUGH MANA"
		}
	case mobileui.ActionConfirm:
		action := m.confirm
		g.setMobileOverlay(mobileui.OverlayNone)
		if g.mobilePowerEnabled(action) {
			kind := populous.CommandFlood
			if action == mobileui.ActionArmageddon {
				kind = populous.CommandArmageddon
			}
			g.queueMobileCommand(populous.Command{Kind: kind, Player: g.player}, false)
		}
	case mobileui.ActionSave:
		if g.multiplayerEnabled() {
			m.status = "SAVE DISABLED ONLINE"
			return
		}
		if err := g.saveGameState(); err != nil {
			m.status = "SAVE FAILED: " + err.Error()
		} else {
			m.status = "GAME SAVED"
		}
	case mobileui.ActionLoad:
		if g.multiplayerEnabled() {
			m.status = "LOAD DISABLED ONLINE"
			return
		}
		if err := g.loadGameState(); err != nil {
			m.status = "LOAD FAILED: " + err.Error()
		} else {
			g.selectMobileTool(mobileui.ActionRaise)
			m.status = "GAME LOADED"
		}
	case mobileui.ActionHelp:
		if g.multiplayerEnabled() {
			m.status = "TWO FINGERS PAN / ONE FINGER ACTS"
			return
		}
		g.setMobileOverlay(mobileui.OverlayNone)
		g.openHelp(StateGame)
	case mobileui.ActionTitle:
		g.setMobileOverlay(mobileui.OverlayNone)
		g.returnToTitleFromTouch()
	case mobileui.ActionToggleWide:
		m.prefs.Wide = !m.prefs.Wide
		g.cancelMobileInput()
		g.saveMobilePreferences()
	case mobileui.ActionTogglePad:
		m.prefs.ShowPad = !m.prefs.ShowPad
		g.cancelMobileInput()
		g.saveMobilePreferences()
	case mobileui.ActionToggleBuildScope:
		if g.multiplayerEnabled() {
			m.status = "CLASSIC BUILD AREA REQUIRED ONLINE"
			return
		}
		if m.prefs.BuildScope == mobileui.BuildScopeClassic {
			m.prefs.BuildScope = mobileui.BuildScopeVisible
		} else {
			m.prefs.BuildScope = mobileui.BuildScopeClassic
		}
		g.cancelMobileInput()
		g.saveMobilePreferences()
	}
}

func (g *Game) mobileTerrainTap(point mobileui.Point) {
	if g.world == nil {
		return
	}
	v := g.mobileViewport()
	if g.mobileUI.tool == mobileui.ActionInspect {
		if index, ok := g.mobileVisiblePeepAt(v, int(point.X), int(point.Y)); ok {
			g.setViewedPeep(index)
		}
		return
	}
	if !g.humanControlsPlayer() {
		return
	}
	x, y, ok := v.PickTile(point.X, point.Y, populous.MapWidth, populous.MapHeight, func(x, y int) float64 { return float64(g.world.MapAlt[x+y*populous.MapWidth]) })
	if !ok {
		return
	}
	c := populous.Command{Player: g.player, X: x, Y: y}
	build := false
	switch g.mobileUI.tool {
	case mobileui.ActionRaise:
		c.Kind = populous.CommandRaise
		build = true
	case mobileui.ActionLower:
		c.Kind = populous.CommandLower
		build = true
	case mobileui.ActionMagnet:
		c.Kind = populous.CommandSetMagnet
	case mobileui.ActionSwamp:
		c.Kind = populous.CommandSwamp
	case mobileui.ActionQuake:
		c.Kind = populous.CommandQuake
		c.X = clampInt(x-4, 0, populous.MapWidth-9)
		c.Y = clampInt(y-4, 0, populous.MapHeight-9)
	case mobileui.ActionVolcano:
		c.Kind = populous.CommandVolcano
		c.X = clampInt(x-4, 0, populous.MapWidth-9)
		c.Y = clampInt(y-4, 0, populous.MapHeight-9)
	default:
		return
	}
	if build && g.paintMap {
		if c.Kind == populous.CommandRaise {
			c.Kind = populous.CommandPaintRaise
		} else {
			c.Kind = populous.CommandPaintLower
		}
		build = false
	}
	g.queueMobileCommand(c, build)
}

func (g *Game) queueMobileCommand(command populous.Command, build bool) {
	if !g.humanControlsPlayer() || len(g.mobileUI.pending) >= 16 {
		return
	}
	scope := g.mobileUI.prefs.BuildScope
	if g.multiplayerEnabled() {
		scope = mobileui.BuildScopeClassic
	}
	g.mobileUI.pending = append(g.mobileUI.pending, mobileOperation{command: command, build: build, view: g.mobileViewport(), scope: scope})
}

func (g *Game) updateMobileGameTick() error {
	m := g.mobileUI
	g.advanceViewedPeep()
	for _, op := range m.pending {
		if op.build && !mobileui.CanBuild(g.world, g.player, op.command.X, op.command.Y, op.scope, func(x, y int) bool { return g.mobileTileVisible(op.view, x, y) }) {
			m.status = "NEED YOUR PEOPLE IN THE BUILD AREA"
			continue
		}
		if !g.issueCommand(op.command) {
			m.status = "ACTION UNAVAILABLE / NOT ENOUGH MANA"
		} else {
			m.status = ""
		}
	}
	m.pending = m.pending[:0]
	paused := g.tutorialPaused || ((m.overlay == mobileui.OverlayMenu || m.overlay == mobileui.OverlaySettings || m.overlay == mobileui.OverlayConfirm) && !g.multiplayerEnabled())
	if paused {
		if g.sound != nil {
			g.sound.ResetHeartbeat()
		}
		return nil
	}
	if g.world != nil {
		advanced := g.advanceWorld()
		g.updateHeartbeat()
		if advanced {
			g.drainWorldSounds()
			g.updateEndState()
		}
	}
	return nil
}

func (g *Game) mobileTileVisible(view mobileui.Viewport, x, y int) bool {
	if !view.Limit.Empty() && !image.Pt(x, y).In(view.Limit) {
		return false
	}
	px, py := view.Project(float64(x), float64(y), float64(g.world.MapAlt[x+y*populous.MapWidth]))
	p := image.Pt(int(math.Round(px)), int(math.Round(py)))
	l := g.mobileLayout()
	return p.In(view.Rect) && !p.In(l.MiniMapRect) && !p.In(l.DPadRect)
}

func mobilePowerDetails(action mobileui.Action) (cost, bit int) {
	switch action {
	case mobileui.ActionQuake:
		return populous.ManaQuakeCost, 0
	case mobileui.ActionSwamp:
		return populous.ManaSwampCost, 1
	case mobileui.ActionKnight:
		return populous.ManaKnightCost, 2
	case mobileui.ActionVolcano:
		return populous.ManaVolcanoCost, 3
	case mobileui.ActionFlood:
		return populous.ManaFloodCost, 4
	case mobileui.ActionArmageddon:
		return populous.ManaWarCost, 5
	}
	return 0, -1
}

func (g *Game) mobilePowerEnabled(action mobileui.Action) bool {
	if g.world == nil || !g.humanControlsPlayer() || g.world.War {
		return false
	}
	cost, bit := mobilePowerDetails(action)
	if bit < 0 {
		return false
	}
	powers := g.world.Level.PlayerPowers
	if g.player == populous.DevilPlayer {
		powers = g.world.Level.EnemyPowers
	}
	if powers&(1<<bit) == 0 || g.world.Magnets[g.player].Mana < cost {
		return false
	}
	if action == mobileui.ActionKnight {
		return g.validViewedPeep(g.world.Magnets[g.player].Carried - 1)
	}
	return true
}

func (g *Game) mobileHUDState() mobileui.HUDState {
	m := g.mobileUI
	s := mobileui.HUDState{Selected: m.tool, Wide: m.prefs.Wide, ShowPad: m.prefs.ShowPad, VisibleBuildScope: m.prefs.BuildScope == mobileui.BuildScopeVisible && !g.multiplayerEnabled(), Online: g.multiplayerEnabled(), Status: m.status}
	if g.world != nil {
		s.Mana = g.world.Magnets[g.player].Mana
		s.Population = g.world.PlayerPopulation(g.player)
	}
	for _, a := range [...]mobileui.Action{mobileui.ActionQuake, mobileui.ActionSwamp, mobileui.ActionKnight, mobileui.ActionVolcano, mobileui.ActionFlood, mobileui.ActionArmageddon} {
		cost, _ := mobilePowerDetails(a)
		s.Powers[a] = mobileui.PowerState{Cost: cost, Enabled: g.mobilePowerEnabled(a)}
	}
	if m.confirm == mobileui.ActionFlood {
		s.Confirmation = "CAST FLOOD? AFFECTS BOTH CAMPS"
	} else {
		s.Confirmation = "START ARMAGEDDON? NO RETURN"
	}
	if g.multiplayerEnabled() && s.Status == "" {
		s.Status = g.network.displayStatus()
	}
	if s.Status == "" && m.tool == mobileui.ActionInspect && g.validViewedPeep(g.viewPeep) {
		p := g.world.Peeps[g.viewPeep]
		kind := "PERSON"
		if p.Flags&populous.InTown != 0 {
			kind = "TOWN"
		}
		if g.peepLooksLikeKnight(p) {
			kind = "KNIGHT"
		}
		side := "BLUE"
		if p.Player == populous.DevilPlayer {
			side = "RED"
		}
		s.Status = fmt.Sprintf("%s %s  PEOPLE %d  WEAPONS %d", side, kind, p.Population, p.Weapons)
	}
	return s
}
