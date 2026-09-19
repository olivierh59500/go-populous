package game

import (
	"errors"
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous/internal/mobileui"
	"go-populous/internal/populous"
)

// Front menus are deliberately separate from StateDemo and its rendering.
func (g *Game) mobileMenuActive() bool {
	if g.mobileUI == nil {
		return false
	}
	switch g.state {
	case StateTitle, StateSetup, StateOptions, StateHelp:
		return true
	}
	return false
}

func (g *Game) mobileFrontPage() mobileui.FrontPage {
	switch g.state {
	case StateTitle:
		return mobileui.FrontHome
	case StateOptions:
		return mobileui.FrontOptions
	case StateHelp:
		return mobileui.FrontHelp
	case StateSetup:
		if g.mobileUI.frontPage == mobileui.FrontConquest || g.mobileUI.frontPage == mobileui.FrontPreferences {
			return g.mobileUI.frontPage
		}
	}
	return mobileui.FrontSetup
}

func (g *Game) mobileFrontLayout() mobileui.FrontLayout {
	page := g.mobileFrontPage()
	count := 0
	switch page {
	case mobileui.FrontSetup:
		count = len(setupItems)
	case mobileui.FrontOptions:
		count = optionRowCount
	case mobileui.FrontHelp:
		count = len(mobileHelpPages)
	}
	return mobileui.NewFrontLayout(g.layoutWidth, logicalHeight, page, g.mobileUI.frontOffset, count, optionRowFirstPower)
}

func (g *Game) openMobileFront(page mobileui.FrontPage) {
	g.cancelMobileInput()
	m := g.mobileUI
	m.frontPage, m.frontOffset, m.status = page, 0, ""
	g.titleCode = ""
	if page == mobileui.FrontConquest && g.levelIndex >= 0 && g.levelIndex < len(g.bundle.Levels) &&
		g.bundle.Levels[g.levelIndex].Code == "TUTORIAL" && g.levelIndex < len(g.demoLevels) {
		// Loading a tutorial save can replace the editable level slot. A
		// conquest selector must describe a campaign world, not that lesson.
		g.bundle.Levels[g.levelIndex] = g.demoLevels[g.levelIndex]
	}
	switch page {
	case mobileui.FrontHome:
		g.state = StateTitle
	case mobileui.FrontOptions:
		g.optionReturn = StateTitle
		g.state = StateOptions
	case mobileui.FrontHelp:
		g.helpReturn = StateTitle
		g.state = StateHelp
	default:
		g.openSetup(StateTitle)
	}
}

func (g *Game) updateMobileFrontInput() {
	g.sampleMobileContacts()
	if !ebiten.IsFocused() {
		g.cancelMobileInput()
		return
	}
	m := g.mobileUI
	layout := g.mobileFrontLayout()
	frame := m.gesture.Update(m.contacts, func(p mobileui.Point) mobileui.Region {
		if image.Pt(int(p.X), int(p.Y)).In(image.Rect(0, 0, layout.Width, layout.Height)) {
			return mobileui.RegionUI
		}
		return mobileui.RegionNone
	})
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.mobileFrontBack()
		return
	}
	if layout.Page == mobileui.FrontConquest {
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
			g.mobileFrontAction(mobileui.FrontActionPrevWorld, 1)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
			g.mobileFrontAction(mobileui.FrontActionNextWorld, 1)
			return
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		switch layout.Page {
		case mobileui.FrontHome:
			g.openMobileFront(mobileui.FrontConquest)
		case mobileui.FrontConquest, mobileui.FrontOptions:
			g.mobileFrontAction(mobileui.FrontActionStart, 0)
		case mobileui.FrontHelp:
			g.mobileFrontBack()
		}
		return
	}
	for _, tap := range frame.Taps {
		a, index, hit := layout.Hit(int(tap.X), int(tap.Y))
		startAction, startIndex, startHit := layout.Hit(int(tap.Start.X), int(tap.Start.Y))
		if hit && startHit && a == startAction && index == startIndex {
			if mobileFrontEnabled(g.mobileFrontState(layout), a, index) {
				g.mobileFrontAction(a, index)
			}
			return
		}
	}
}

func mobileFrontEnabled(state mobileui.FrontState, action mobileui.FrontAction, index int) bool {
	switch action {
	case mobileui.FrontActionTutorial, mobileui.FrontActionConquest, mobileui.FrontActionCustom, mobileui.FrontActionSetup, mobileui.FrontActionPreferences, mobileui.FrontActionLoad, mobileui.FrontActionHelp, mobileui.FrontActionDemo,
		mobileui.FrontActionSetupItem, mobileui.FrontActionDecrease, mobileui.FrontActionIncrease, mobileui.FrontActionTogglePlayerPower, mobileui.FrontActionToggleEnemyPower, mobileui.FrontActionToggleWide, mobileui.FrontActionTogglePad, mobileui.FrontActionToggleScope:
		return index >= 0 && index < len(state.Items) && state.Items[index].Enabled
	case mobileui.FrontActionStart:
		return state.CanStart
	}
	return true
}

func (g *Game) mobileFrontBack() {
	g.cancelMobileInput()
	m := g.mobileUI
	page := g.mobileFrontPage()
	m.frontOffset = 0
	m.status = ""
	switch page {
	case mobileui.FrontHome:
		return
	case mobileui.FrontOptions:
		g.state = g.optionReturn
		m.frontPage = mobileui.FrontSetup
	case mobileui.FrontHelp:
		g.state = g.helpReturn
		if g.state == StateHelp {
			g.state = StateTitle
		}
	case mobileui.FrontSetup:
		g.returnFromSetup()
	default:
		g.state = StateTitle
	}
}

func (g *Game) mobileFrontAction(action mobileui.FrontAction, index int) {
	m := g.mobileUI
	layout := g.mobileFrontLayout()
	g.cancelMobileInput()
	switch action {
	case mobileui.FrontActionTutorial:
		g.selectMobileTool(mobileui.ActionRaise)
		g.startTutorial()
	case mobileui.FrontActionConquest:
		g.openMobileFront(mobileui.FrontConquest)
	case mobileui.FrontActionCustom:
		g.openMobileFront(mobileui.FrontOptions)
	case mobileui.FrontActionSetup:
		g.openMobileFront(mobileui.FrontSetup)
	case mobileui.FrontActionPreferences:
		g.openMobileFront(mobileui.FrontPreferences)
	case mobileui.FrontActionHelp:
		g.openMobileFront(mobileui.FrontHelp)
	case mobileui.FrontActionDemo:
		if g.state == StateTitle {
			g.StartDemo()
		}
	case mobileui.FrontActionBack:
		g.mobileFrontBack()
	case mobileui.FrontActionPrevPage, mobileui.FrontActionNextPage:
		delta := 1
		if action == mobileui.FrontActionPrevPage {
			delta = -1
		}
		m.frontOffset = clampInt(layout.PageIndex+delta, 0, layout.PageCount-1)
	case mobileui.FrontActionPrevWorld, mobileui.FrontActionNextWorld:
		if len(g.bundle.Levels) == 0 {
			return
		}
		delta := clampInt(index, 1, 10)
		if action == mobileui.FrontActionPrevWorld {
			delta = -delta
		}
		g.titleCode = ""
		g.setLevel(wrapInt(g.levelIndex+delta, len(g.bundle.Levels)))
	case mobileui.FrontActionStart:
		if g.world == nil || len(g.bundle.Levels) == 0 {
			m.status = "NO CAMPAIGN DATA"
			return
		}
		if g.tutorialActive || g.world.Level != g.bundle.Levels[g.levelIndex] {
			g.setLevel(g.levelIndex)
		}
		g.titleCode = ""
		g.tutorialPaused = false
		g.selectMobileTool(mobileui.ActionRaise)
		g.startTitleSelection()
	case mobileui.FrontActionLoad:
		g.mobileFrontLoad()
	case mobileui.FrontActionSetupItem:
		if index < 0 || index >= len(setupItems) {
			return
		}
		if setupItems[index].action == setupConquest {
			g.openMobileFront(mobileui.FrontConquest)
			return
		}
		if setupItems[index].action == setupLoad {
			g.mobileFrontLoad()
			return
		}
		g.activateSetupItem(index)
		m.status = g.setupMessage
		if g.state == StateOptions {
			m.frontOffset = 0
		}
	case mobileui.FrontActionDecrease, mobileui.FrontActionIncrease:
		if index < 0 || index >= optionRowFirstPower {
			return
		}
		g.optionCursor = index
		delta := 1
		if action == mobileui.FrontActionDecrease {
			delta = -1
		}
		g.adjustOption(delta)
	case mobileui.FrontActionTogglePlayerPower, mobileui.FrontActionToggleEnemyPower:
		if index < optionRowFirstPower || index >= optionRowCount {
			return
		}
		g.optionCursor = index
		side := 1
		if action == mobileui.FrontActionTogglePlayerPower {
			side = -1
		}
		g.togglePower(side)
	case mobileui.FrontActionToggleWide:
		g.mobileAction(mobileui.ActionToggleWide)
	case mobileui.FrontActionTogglePad:
		g.mobileAction(mobileui.ActionTogglePad)
	case mobileui.FrontActionToggleScope:
		g.mobileAction(mobileui.ActionToggleBuildScope)
	}
}

func (g *Game) mobileFrontLoad() {
	if err := g.loadGameState(); err != nil {
		g.mobileUI.status = "SAVED GAME COULD NOT BE LOADED"
		if errors.Is(err, os.ErrNotExist) {
			g.mobileUI.status = "NO SAVED GAME YET"
		}
		return
	}
	g.tutorialPaused = false
	g.tutorialActive = g.world != nil && g.world.Level.Code == "TUTORIAL"
	g.state = StateGame
	g.selectMobileTool(mobileui.ActionRaise)
	g.mobileUI.status = "GAME LOADED"
}

func (g *Game) mobileFrontState(layout mobileui.FrontLayout) mobileui.FrontState {
	m := g.mobileUI
	s := mobileui.FrontState{Title: "POPULOUS", Status: m.status, CanStart: g.world != nil && len(g.bundle.Levels) > 0}
	level := populous.Level{}
	if len(g.bundle.Levels) > 0 {
		level = g.bundle.Levels[clampInt(g.levelIndex, 0, len(g.bundle.Levels)-1)]
	}
	s.WorldTitle = fmt.Sprintf("WORLD %03d", level.Number)
	s.WorldCode = level.Code
	s.WorldDetail = fmt.Sprintf("%s  PEOPLE %d / %d", terrainName(level.Terrain), level.PlayerPopulation, level.EnemyPopulation)
	s.Subtitle = s.WorldTitle + "  " + s.WorldCode
	switch layout.Page {
	case mobileui.FrontHome:
		s.Items = []mobileui.FrontItem{
			{Label: "TUTORIAL", Detail: "Learn terrain and powers", Enabled: true},
			{Label: "CONQUEST", Detail: "Choose a world and play", Enabled: s.CanStart},
			{Label: "CUSTOM GAME", Detail: "Terrain, rules and powers", Enabled: s.CanStart},
			{Label: "GAME SETUP", Detail: "Side, control, save and load", Enabled: s.CanStart},
			{Label: "TOUCH SETTINGS", Detail: "Wide view and build area", Enabled: true},
			{Label: "LOAD GAME", Detail: "Continue your saved game", Enabled: g.savePath != ""},
			{Label: "HELP", Detail: "Touch controls and rules", Enabled: true},
			{Label: "DEMO", Detail: "Watch the two AIs", Enabled: true},
		}
		if g.demoIdle != nil && g.demoIdle.LimitTicks > 0 {
			s.IdleEnabled = true
			s.IdleSeconds = max(0, g.demoIdle.LimitTicks-g.demoIdle.ElapsedTicks+simulationTPS-1) / simulationTPS
		}
	case mobileui.FrontConquest:
		s.Title = "CONQUEST"
		s.Subtitle = "CHOOSE A WORLD, THEN PLAY"
	case mobileui.FrontSetup:
		s.Title = "GAME SETUP"
		s.Subtitle = "SELECT AN ACTION"
		s.Items = make([]mobileui.FrontItem, len(setupItems))
		for i, item := range setupItems {
			label := g.setupLine(item.action)
			s.Items[i] = mobileui.FrontItem{Label: strings.TrimPrefix(strings.TrimPrefix(label, "[X] "), "[ ] "), Selected: strings.HasPrefix(label, "[X] "), Enabled: !setupItemDisabled(item.action)}
		}
	case mobileui.FrontOptions:
		s.Title = "CUSTOM GAME"
		s.Subtitle = s.WorldTitle + "  " + s.WorldCode + "  -  CHANGE OPTIONS THEN PLAY"
		s.Items = make([]mobileui.FrontItem, optionRowCount)
		for row := range s.Items {
			s.Items[row] = g.mobileFrontOption(row, level)
		}
	case mobileui.FrontPreferences:
		s.Title = "TOUCH SETTINGS"
		s.Subtitle = "SAVED AUTOMATICALLY ON THIS DEVICE"
		scope, scopeDetail := "LOCAL 8x8", "Allied presence near the target"
		if m.prefs.BuildScope == mobileui.BuildScopeVisible {
			scope = "VISIBLE MAP"
			scopeDetail = "Allied presence anywhere in view"
		}
		if g.multiplayerEnabled() {
			scope = "LOCAL - LOCKED ONLINE"
		}
		wide, wideDetail := "OFF", "View limited to 8x8 tiles"
		if m.prefs.Wide {
			wide, wideDetail = "ON", "More terrain on screen"
		}
		pad, padDetail := "OFF", "Pan with two fingers"
		if m.prefs.ShowPad {
			pad, padDetail = "ON", "Directional buttons enabled"
		}
		s.Items = []mobileui.FrontItem{
			{Label: "WIDE TERRAIN", Value: wide, Detail: wideDetail, Selected: m.prefs.Wide, Enabled: true},
			{Label: "VIRTUAL PAD", Value: pad, Detail: padDetail, Selected: m.prefs.ShowPad, Enabled: true},
			{Label: "BUILD AREA", Value: scope, Detail: scopeDetail, Selected: m.prefs.BuildScope == mobileui.BuildScopeVisible, Enabled: !g.multiplayerEnabled()},
		}
	case mobileui.FrontHelp:
		s.Title = "HELP"
		s.Subtitle = "TOUCH CONTROLS"
		s.HelpLines = mobileHelpPages[clampInt(layout.PageIndex, 0, len(mobileHelpPages)-1)]
	}
	return s
}

func (g *Game) mobileFrontOption(row int, level populous.Level) mobileui.FrontItem {
	item := mobileui.FrontItem{Enabled: len(g.bundle.Levels) > 0}
	switch row {
	case optionRowWorld:
		item.Label = "WORLD"
		item.Value = fmt.Sprintf("%03d  %s", level.Number, level.Code)
	case optionRowTerrain:
		item.Label = "TERRAIN"
		item.Value = terrainName(level.Terrain)
	case optionRowReaction:
		item.Label = "ENEMY REACTIONS"
		item.Value = fmt.Sprintf("%s (%d)", speedName(level.EnemyReactionSpeed), level.EnemyReactionSpeed)
	case optionRowRating:
		item.Label = "ENEMY RATING"
		item.Value = fmt.Sprintf("%s (%d)", ratingName(level.EnemyRating), level.EnemyRating)
	case optionRowBuild:
		item.Label = "BUILD RULES"
		item.Value = buildModeNames[buildModeIndex(level.GameMode)]
	case optionRowSwamps:
		item.Label = "SWAMPS"
		item.Value = "SHALLOW"
		if level.GameMode&populous.GameSwampRemain != 0 {
			item.Value = "BOTTOMLESS"
		}
	case optionRowWater:
		item.Label = "WATER"
		item.Value = "HARMFUL"
		if level.GameMode&populous.GameWaterFatal != 0 {
			item.Value = "FATAL"
		}
	case optionRowPopulationYou:
		item.Label = "BLUE STARTING PEOPLE"
		item.Value = fmt.Sprint(level.PlayerPopulation)
	case optionRowPopulationHim:
		item.Label = "RED STARTING PEOPLE"
		item.Value = fmt.Sprint(level.EnemyPopulation)
	default:
		power := row - optionRowFirstPower
		if power >= 0 && power < len(optionPowerNames) {
			item.Label = optionPowerNames[power]
			item.Value = "POWERS PER SIDE"
			item.PlayerPower = level.PlayerPowers&(1<<power) != 0
			item.EnemyPower = level.EnemyPowers&(1<<power) != 0
		}
	}
	return item
}

var mobileHelpPages = [][]string{
	{"Select a tool, then touch the terrain.", "RAISE / LOWER: sculpt on finger release.", "Two fingers on terrain: move the map.", "Lifting one finger never sculpts.", "Minimap: jump to another location.", "LOOK: inspect a person or town.", "LEAD: find your leader."},
	{"POWER: choose a spell, then aim.", "Costs and availability are displayed.", "Flood and Armageddon ask to confirm.", "TRIBE: settle, join, fight or follow.", "MENU: save, load, help and settings.", "Settings keep wide view and optional pad.", "BUILD AREA selects local or visible."},
	{"CONQUEST: select a world, then PLAY.", "CUSTOM GAME: use minus / plus controls.", "Set BLUE and RED powers separately.", "Use NEXT / PREVIOUS to see every option.", "LOAD GAME restores your existing save.", "The title starts an AI demo when idle.", "Touch the demo to return to this menu."},
}
