package game

import (
	"encoding/gob"
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous/internal/assets"
	"go-populous/internal/populous"
)

const (
	logicalWidth  = 320
	logicalHeight = 240
	miniMapWidth  = 128
	miniMapHeight = 64
	saveVersion   = 2
	saveFileName  = "go-populous.sav"
)

var manaGaugeValues = [...]int{
	populous.ManaFloor,
	populous.ManaPointCost,
	populous.ManaMagnetCost,
	populous.ManaQuakeCost,
	populous.ManaSwampCost,
	populous.ManaKnightCost,
	populous.ManaVolcanoCost,
	populous.ManaFloodCost,
	populous.ManaWarCost,
	160000,
	1999999,
}

const (
	optionRowWorld = iota
	optionRowTerrain
	optionRowReaction
	optionRowRating
	optionRowBuild
	optionRowSwamps
	optionRowWater
	optionRowPopulationYou
	optionRowPopulationHim
	optionRowFirstPower
	optionRowCount = optionRowFirstPower + len(optionPowerNames)
)

var terrainNames = [...]string{"GRASS PLANES", "DESERT", "SNOW AND ICE", "ROCKY"}

var speedNames = [...]string{"VERY SLOW", "SLOW", "MEDIUM", "FAST", "VERY FAST"}

var ratingNames = [...]string{"VERY POOR", "POOR", "AVERAGE", "GOOD", "VERY GOOD"}

var buildModeNames = [...]string{"BUILT ON PEOPLE", "ONLY BUILT UP", "BUILT JUST ON TOWNS", "CANNOT BE BUILT"}

var optionPowerNames = [...]string{"EARTHQUAKES", "SWAMPS", "KNIGHTS", "VOLCANOS", "FLOODS", "ARMAGEDDON"}

const (
	setupPlay = iota
	setupPaint
	setupGood
	setupEvil
	setupHuman
	setupPPC
	setupConquest
	setupCustom
	setupOptions
	setupSave
	setupLoad
	setupNextMap
	setupRestart
	setupSurrender
)

const (
	titleTutorial = iota
	titleConquest
	titleCustom
	titleOptions
	titleHelp
)

type setupItem struct {
	action int
	x      int
	y      int
	width  int
}

type titleMenuItem struct {
	action int
	label  string
}

type saveState struct {
	Version            int
	LevelIndex         int
	Player             int
	ComputerControlled [2]bool
	Xoff               int
	Yoff               int
	Mode               ActionMode
	PaintMap           bool
	World              populous.WorldSnapshot
}

var setupItems = [...]setupItem{
	{action: setupPlay, x: 16, y: 46, width: 88},
	{action: setupPaint, x: 152, y: 46, width: 88},
	{action: setupGood, x: 16, y: 56, width: 64},
	{action: setupEvil, x: 152, y: 56, width: 64},
	{action: setupHuman, x: 16, y: 66, width: 112},
	{action: setupPPC, x: 152, y: 66, width: 88},
	{action: setupConquest, x: 16, y: 76, width: 88},
	{action: setupCustom, x: 152, y: 76, width: 112},
	{action: setupOptions, x: 80, y: 95, width: 128},
	{action: setupSave, x: 80, y: 105, width: 128},
	{action: setupLoad, x: 80, y: 115, width: 128},
	{action: setupNextMap, x: 80, y: 125, width: 128},
	{action: setupRestart, x: 80, y: 135, width: 128},
	{action: setupSurrender, x: 80, y: 145, width: 152},
}

var titleMenuItems = [...]titleMenuItem{
	{action: titleTutorial, label: "TUTORIAL"},
	{action: titleConquest, label: "CONQUEST"},
	{action: titleCustom, label: "CUSTOM GAME"},
	{action: titleOptions, label: "OPTIONS"},
	{action: titleHelp, label: "HELP"},
}

var conquestRankNames = [...]string{
	"MORTAL",
	"IMMORTAL",
	"ETERNAL",
	"DEVA",
	"GREATER BEING",
	"DEITY",
	"GREATER DEITY",
	"MORTAL GOD",
	"GREATER GOD",
	"ETERNAL GOD",
}

type State int

const (
	StateTitle State = iota
	StateSetup
	StateOptions
	StateGame
	StateEnd
	StateHelp
	StateLord
)

type ActionMode int

const (
	ModeSculpt ActionMode = iota
	ModeMagnet
	ModeSwamp
)

type Game struct {
	bundle             *assets.Bundle
	state              State
	atlasView          bool
	fullscreen         bool
	images             map[string]*ebiten.Image
	lands              []*ebiten.Image
	blocks             *ebiten.Image
	sprites            *ebiten.Image
	bigSprites         *ebiten.Image
	mouths             *ebiten.Image
	miniMap            *ebiten.Image
	miniPixels         []byte
	levelIndex         int
	titleCode          string
	titleMessage       string
	player             int
	computerControlled [2]bool
	titleMenuCursor    int
	setupCursor        int
	setupReturn        State
	setupMessage       string
	optionCursor       int
	optionReturn       State
	helpReturn         State
	tutorialActive     bool
	tutorialPaused     bool
	lordVoiceStarted   bool
	paintMap           bool
	mode               ActionMode
	xoff               int
	yoff               int
	hoverX             int
	hoverY             int
	hoverLocalX        int
	hoverLocalY        int
	hoverOK            bool
	viewFight          int
	viewPeople         int
	endLost            bool
	endSummary         [2]populous.PlayerSummary
	endScore           int
	endNextLevel       int
	tick               int
	world              *populous.World
	sound              *soundPlayer
}

func New(bundle *assets.Bundle) *Game {
	g := &Game{
		bundle:             bundle,
		images:             map[string]*ebiten.Image{},
		player:             populous.GodPlayer,
		computerControlled: [2]bool{false, true},
		titleMenuCursor:    titleConquest,
		sound:              newSoundPlayer(bundle.SoundBank),
	}
	for name, img := range bundle.Screens {
		g.images[name] = ebiten.NewImageFromImage(img)
	}
	for _, img := range bundle.Lands {
		g.lands = append(g.lands, ebiten.NewImageFromImage(img))
	}
	if bundle.Blocks != nil {
		g.blocks = ebiten.NewImageFromImage(bundle.Blocks)
	}
	if bundle.Sprites != nil {
		g.sprites = ebiten.NewImageFromImage(bundle.Sprites)
	}
	if bundle.BigSprites != nil {
		g.bigSprites = ebiten.NewImageFromImage(bundle.BigSprites)
	}
	if bundle.Mouths != nil {
		g.mouths = ebiten.NewImageFromImage(bundle.Mouths)
	}
	g.setLevel(0)
	return g
}

func (g *Game) Update() error {
	g.tick++
	if g.sound != nil {
		g.sound.Update()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.fullscreen = !g.fullscreen
		ebiten.SetFullscreen(g.fullscreen)
	}

	switch g.state {
	case StateTitle:
		g.handleTitleInput()
	case StateSetup:
		g.handleSetupInput()
	case StateOptions:
		g.handleOptionsInput()
	case StateGame:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.openSetup(StateGame)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			g.openHelp(StateGame)
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			g.atlasView = !g.atlasView
		}
		if g.humanControlsPlayer() && inpututil.IsKeyJustPressed(ebiten.KeyM) {
			g.toggleMagnetMode()
		}
		if g.humanControlsPlayer() && inpututil.IsKeyJustPressed(ebiten.KeyDigit1) {
			g.setPlayerTendency(populous.SettleMode)
		}
		if g.humanControlsPlayer() && inpututil.IsKeyJustPressed(ebiten.KeyDigit2) {
			g.setPlayerTendency(populous.JoinMode)
		}
		if g.humanControlsPlayer() && inpututil.IsKeyJustPressed(ebiten.KeyDigit3) {
			g.setPlayerTendency(populous.FightMode)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) && len(g.bundle.Levels) > 0 {
			g.setLevel((g.levelIndex + 1) % len(g.bundle.Levels))
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) && len(g.bundle.Levels) > 0 {
			next := g.levelIndex - 1
			if next < 0 {
				next = len(g.bundle.Levels) - 1
			}
			g.setLevel(next)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyD) && g.xoff < populous.MapWidth-8 {
			g.xoff++
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyA) && g.xoff > 0 {
			g.xoff--
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyS) && g.yoff < populous.MapHeight-8 {
			g.yoff++
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyW) && g.yoff > 0 {
			g.yoff--
		}
		g.updateHoverTile()
		if !g.handleIconInput() && !g.handleTargetPowerInput() && !g.handleMouseNavigation() {
			g.handleSculptInput()
		}
		if g.tutorialPaused {
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				g.tutorialPaused = false
			}
			g.updateHeartbeat()
			return nil
		}
		if g.world != nil {
			g.world.TickWithComputer(g.computerControlled)
			g.updateHeartbeat()
			g.drainWorldSounds()
			g.updateEndState()
		}
	case StateEnd:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.tutorialActive = false
			g.tutorialPaused = false
			g.setLevel(g.levelIndex)
			g.state = StateTitle
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if g.tutorialActive {
				if g.endLost {
					g.startTutorial()
				} else {
					g.tutorialActive = false
					g.tutorialPaused = false
					g.setLevel(g.levelIndex)
					g.state = StateTitle
				}
			} else if g.endLost {
				g.setLevel(g.levelIndex)
			} else {
				g.openLordScreen()
			}
			if g.state == StateEnd {
				g.state = StateGame
			}
		}
	case StateHelp:
		g.handleHelpInput()
	case StateLord:
		g.handleLordInput()
	}
	return nil
}

func (g *Game) drainWorldSounds() {
	if g.world == nil || g.sound == nil {
		return
	}
	for _, sound := range g.world.DrainSoundEvents() {
		g.sound.Play(sound)
	}
}

func (g *Game) updateHeartbeat() {
	if g.sound == nil {
		return
	}
	if g.world == nil || g.state != StateGame {
		g.sound.ResetHeartbeat()
		return
	}
	g.sound.SetHeartbeat(g.world.PlayerPopulation(g.player), g.world.PlayerPopulation(g.opponent()))
}

func (g *Game) updateEndState() {
	if g.world == nil {
		return
	}
	switch g.world.ResultFor(g.player) {
	case populous.ResultWon:
		g.finishGame(false)
	case populous.ResultLost:
		g.finishGame(true)
	}
}

func (g *Game) finishGame(lost bool) {
	if g.world == nil {
		return
	}
	g.endLost = lost
	if g.sound != nil {
		g.sound.ResetHeartbeat()
	}
	g.endSummary[g.player] = g.world.SummaryFor(g.player)
	g.endSummary[g.opponent()] = g.world.SummaryFor(g.opponent())
	g.endScore = g.world.EndScore(g.player, lost)
	if g.tutorialActive {
		g.endNextLevel = g.levelIndex
	} else {
		g.endNextLevel = g.world.NextConquestLevelIndex(g.endScore)
	}
	g.lordVoiceStarted = false
	g.state = StateEnd
}

func (g *Game) playConquestVoice() {
	if g.sound == nil {
		return
	}
	g.sound.PlaySequence(populous.ConquestVoiceSequence(g.endNextLevel, g.conquestCompleted()))
}

func (g *Game) conquestCompleted() bool {
	return g.levelIndex*5 == 2470 && g.endNextLevel == 0
}

func (g *Game) openLordScreen() {
	g.state = StateLord
	g.startLordVoice()
}

func (g *Game) startLordVoice() {
	if g.lordVoiceStarted {
		return
	}
	g.lordVoiceStarted = true
	g.playConquestVoice()
}

func (g *Game) handleLordInput() {
	g.startLordVoice()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.setLevel(g.levelIndex)
		g.state = StateTitle
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.continueFromLord()
	}
}

func (g *Game) continueFromLord() {
	g.setLevel(g.endNextLevel)
	g.state = StateGame
}

func (g *Game) opponent() int {
	if g.player == populous.DevilPlayer {
		return populous.GodPlayer
	}
	return populous.DevilPlayer
}

func (g *Game) humanControlsPlayer() bool {
	if g.player < 0 || g.player >= len(g.computerControlled) {
		return false
	}
	return !g.computerControlled[g.player]
}

func (g *Game) handleTitleInput() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.openSetup(StateTitle)
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) || (g.titleCode == "" && inpututil.IsKeyJustPressed(ebiten.KeyO)) {
		g.titleMessage = ""
		g.optionReturn = StateTitle
		g.state = StateOptions
		return
	}
	if g.titleCode == "" {
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			g.titleMenuCursor = wrapInt(g.titleMenuCursor-1, len(titleMenuItems))
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			g.titleMenuCursor = wrapInt(g.titleMenuCursor+1, len(titleMenuItems))
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyT) {
			g.startTutorial()
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			g.openHelp(StateTitle)
			return
		}
	}
	if len(g.bundle.Levels) > 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
			g.setLevel((g.levelIndex + 1) % len(g.bundle.Levels))
			g.titleCode = ""
			g.titleMessage = ""
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
			next := g.levelIndex - 1
			if next < 0 {
				next = len(g.bundle.Levels) - 1
			}
			g.setLevel(next)
			g.titleCode = ""
			g.titleMessage = ""
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if index, ok := titleMenuItemAt(x, y); ok {
			g.titleMenuCursor = index
			g.activateTitleMenu(index)
			return
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.titleCode) > 0 {
		g.titleCode = g.titleCode[:len(g.titleCode)-1]
		g.titleMessage = ""
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.titleCode = ""
		g.titleMessage = ""
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if len(g.titleCode) >= 12 {
			break
		}
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		if r >= 'A' && r <= 'Z' {
			g.titleCode += string(r)
			g.titleMessage = ""
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if g.titleCode == "" {
			g.activateTitleMenu(g.titleMenuCursor)
			return
		}
		g.startTitleSelection()
	}
}

func titleMenuItemAt(x, y int) (int, bool) {
	const menuX = 224
	const menuY = 132
	const menuWidth = 88
	const lineHeight = 12
	if x < menuX || x >= menuX+menuWidth || y < menuY {
		return 0, false
	}
	index := (y - menuY) / lineHeight
	if index < 0 || index >= len(titleMenuItems) || y >= menuY+index*lineHeight+9 {
		return 0, false
	}
	return index, true
}

func (g *Game) activateTitleMenu(index int) {
	if index < 0 || index >= len(titleMenuItems) {
		return
	}
	g.titleMessage = ""
	switch titleMenuItems[index].action {
	case titleTutorial:
		g.startTutorial()
	case titleConquest:
		g.startTitleSelection()
	case titleCustom:
		g.optionReturn = StateTitle
		g.state = StateOptions
	case titleOptions:
		g.openSetup(StateTitle)
	case titleHelp:
		g.openHelp(StateTitle)
	}
}

func (g *Game) startTitleSelection() {
	if len(g.bundle.Levels) == 0 {
		g.state = StateGame
		return
	}
	if g.titleCode == "" {
		g.state = StateGame
		return
	}
	if level, ok := populous.DecodeLevelCode(g.titleCode, len(g.bundle.Levels)-1); ok {
		g.setLevel(level)
		g.titleCode = ""
		g.titleMessage = ""
		g.state = StateGame
		return
	}
	g.titleMessage = "NO SUCH WORLD"
}

func (g *Game) startTutorial() {
	level := populous.TutorialLevel()
	rules := populous.DefaultTerrainRules()
	if terrain := int(level.Terrain); terrain >= 0 && terrain < len(g.bundle.TerrainRules) {
		rules = g.bundle.TerrainRules[terrain]
	}
	g.tutorialActive = true
	g.tutorialPaused = true
	g.player = populous.GodPlayer
	g.computerControlled = [2]bool{false, true}
	g.paintMap = false
	g.mode = ModeSculpt
	g.world = populous.GenerateWorldWithRules(level, rules)
	g.world.ConfigureTutorial()
	g.world.SetScorePlayer(g.player)
	g.world.ComputerControlled = g.computerControlled
	g.viewFight = 0
	g.viewPeople = 0
	g.endLost = false
	g.endSummary = [2]populous.PlayerSummary{}
	g.endScore = 0
	g.endNextLevel = g.levelIndex
	g.titleCode = ""
	g.titleMessage = ""
	if g.sound != nil {
		g.sound.ResetHeartbeat()
	}
	g.centerOnPlayerLeader()
	g.state = StateGame
}

func (g *Game) openHelp(returnState State) {
	g.helpReturn = returnState
	g.state = StateHelp
}

func (g *Game) handleHelpInput() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.helpReturn == StateHelp {
			g.helpReturn = StateTitle
		}
		g.state = g.helpReturn
	}
}

func (g *Game) openSetup(returnState State) {
	g.setupReturn = returnState
	g.setupMessage = ""
	g.state = StateSetup
}

func (g *Game) handleSetupInput() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.returnFromSetup()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		g.optionReturn = StateSetup
		g.state = StateOptions
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.state = StateTitle
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		g.moveSetupCursor(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		g.moveSetupCursor(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.activateSetupItem(g.setupCursor)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if index, ok := setupItemAt(x, y); ok {
			g.setupCursor = index
			g.activateSetupItem(index)
		}
	}
}

func (g *Game) returnFromSetup() {
	if g.setupReturn == StateTitle {
		g.state = StateTitle
		return
	}
	g.state = StateGame
}

func (g *Game) moveSetupCursor(delta int) {
	if len(setupItems) == 0 {
		return
	}
	for {
		g.setupCursor = wrapInt(g.setupCursor+delta, len(setupItems))
		if !setupItemDisabled(setupItems[g.setupCursor].action) {
			return
		}
	}
}

func setupItemAt(x, y int) (int, bool) {
	for i, item := range setupItems {
		if x >= 16+item.x && x <= 16+item.x+item.width && y >= 16+item.y && y <= 16+item.y+8 {
			return i, true
		}
	}
	return 0, false
}

func setupItemDisabled(action int) bool {
	return false
}

func (g *Game) activateSetupItem(index int) {
	if index < 0 || index >= len(setupItems) {
		return
	}
	action := setupItems[index].action
	if setupItemDisabled(action) {
		g.setupMessage = "NOT PORTED YET"
		return
	}
	g.setupMessage = ""
	switch action {
	case setupPlay:
		g.state = StateGame
	case setupPaint:
		g.paintMap = !g.paintMap
	case setupGood:
		g.setPlayerSide(populous.GodPlayer)
	case setupEvil:
		g.setPlayerSide(populous.DevilPlayer)
	case setupHuman:
		g.computerControlled[g.player] = false
		g.computerControlled[g.opponent()] = true
		if g.world != nil {
			g.world.ComputerControlled = g.computerControlled
		}
	case setupPPC:
		g.computerControlled = [2]bool{true, true}
		if g.world != nil {
			g.world.ComputerControlled = g.computerControlled
		}
	case setupConquest:
		// Already the active one-player conquest setup.
	case setupCustom, setupOptions:
		g.optionReturn = StateSetup
		g.state = StateOptions
	case setupSave:
		if err := g.saveGameState(); err != nil {
			g.setupMessage = "SAVE FAILED: " + err.Error()
		} else {
			g.setupMessage = "SAVED TO " + saveFileName
		}
	case setupLoad:
		if err := g.loadGameState(); err != nil {
			g.setupMessage = "LOAD FAILED: " + err.Error()
		} else {
			g.state = StateGame
		}
	case setupNextMap:
		if len(g.bundle.Levels) > 0 {
			g.setLevel((g.levelIndex + 1) % len(g.bundle.Levels))
		}
		g.state = StateGame
	case setupRestart:
		if g.tutorialActive {
			g.startTutorial()
		} else {
			g.setLevel(g.levelIndex)
		}
		g.state = StateGame
	case setupSurrender:
		g.finishGame(true)
	}
}

func (g *Game) setPlayerSide(player int) {
	if player != populous.GodPlayer && player != populous.DevilPlayer {
		return
	}
	wasPPC := g.computerControlled[populous.GodPlayer] && g.computerControlled[populous.DevilPlayer]
	g.player = player
	if !wasPPC {
		g.computerControlled[g.player] = false
		g.computerControlled[g.opponent()] = true
	}
	if g.world != nil {
		g.world.SetScorePlayer(g.player)
		g.world.ComputerControlled = g.computerControlled
	}
	g.centerOnPlayerLeader()
}

func (g *Game) saveGameState() error {
	if g.world == nil {
		return fmt.Errorf("no active world")
	}
	file, err := os.Create(saveFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	state := saveState{
		Version:            saveVersion,
		LevelIndex:         g.levelIndex,
		Player:             g.player,
		ComputerControlled: g.computerControlled,
		Xoff:               g.xoff,
		Yoff:               g.yoff,
		Mode:               g.mode,
		PaintMap:           g.paintMap,
		World:              g.world.Snapshot(),
	}
	return gob.NewEncoder(file).Encode(state)
}

func (g *Game) loadGameState() error {
	file, err := os.Open(saveFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	var state saveState
	if err := gob.NewDecoder(file).Decode(&state); err != nil {
		return err
	}
	if state.Version != saveVersion {
		return fmt.Errorf("unsupported save version %d", state.Version)
	}
	if len(g.bundle.Levels) > 0 {
		g.levelIndex = clampInt(state.LevelIndex, 0, len(g.bundle.Levels)-1)
		g.bundle.Levels[g.levelIndex] = state.World.Level
	} else {
		g.levelIndex = 0
	}
	g.player = clampInt(state.Player, populous.GodPlayer, populous.DevilPlayer)
	g.computerControlled = state.ComputerControlled

	rules := populous.DefaultTerrainRules()
	terrain := state.World.Terrain
	if terrain < 0 || terrain > 3 {
		terrain = int(state.World.Level.Terrain)
	}
	if terrain >= 0 && terrain < len(g.bundle.TerrainRules) {
		rules = g.bundle.TerrainRules[terrain]
	}
	g.world = populous.WorldFromSnapshot(state.World, rules)
	g.world.SetScorePlayer(g.player)
	g.world.ComputerControlled = g.computerControlled
	g.mode = state.Mode
	g.paintMap = state.PaintMap
	g.xoff = state.Xoff
	g.yoff = state.Yoff
	g.clampView()
	g.viewFight = 0
	g.viewPeople = 0
	g.endLost = false
	g.endSummary = [2]populous.PlayerSummary{}
	g.endScore = 0
	g.endNextLevel = g.levelIndex
	return nil
}

func (g *Game) handleOptionsInput() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.state = g.optionReturn
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		g.optionCursor--
		if g.optionCursor < 0 {
			g.optionCursor = optionRowCount - 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		g.optionCursor = (g.optionCursor + 1) % optionRowCount
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.adjustOption(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		g.adjustOption(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.toggleOption()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		x, y := ebiten.CursorPosition()
		if row, ok := optionRowAt(x, y); ok {
			g.optionCursor = row
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
				g.adjustOption(-1)
			} else {
				g.adjustOption(1)
			}
		}
	}
}

func optionRowAt(x, y int) (int, bool) {
	if x < 12 || x >= 308 {
		return 0, false
	}
	row := (y - 42) / 10
	if row < 0 || row >= optionRowCount || y < 42+row*10 || y >= 42+row*10+8 {
		return 0, false
	}
	return row, true
}

func (g *Game) adjustOption(delta int) {
	if len(g.bundle.Levels) == 0 || delta == 0 {
		return
	}
	switch {
	case g.optionCursor == optionRowWorld:
		next := (g.levelIndex + delta) % len(g.bundle.Levels)
		if next < 0 {
			next += len(g.bundle.Levels)
		}
		g.setLevel(next)
	case g.optionCursor == optionRowTerrain:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.Terrain = byte(wrapInt(int(level.Terrain)+delta, len(terrainNames)))
		})
	case g.optionCursor == optionRowReaction:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.EnemyReactionSpeed = byte(clampInt(int(level.EnemyReactionSpeed)+delta, 1, 10))
		})
	case g.optionCursor == optionRowRating:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.EnemyRating = byte(clampInt(int(level.EnemyRating)+delta, 1, 10))
		})
	case g.optionCursor == optionRowBuild:
		g.updateCurrentLevel(func(level *populous.Level) {
			setBuildMode(level, wrapInt(buildModeIndex(level.GameMode)+delta, len(buildModeNames)))
		})
	case g.optionCursor == optionRowSwamps:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.GameMode ^= populous.GameSwampRemain
		})
	case g.optionCursor == optionRowWater:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.GameMode ^= populous.GameWaterFatal
		})
	case g.optionCursor == optionRowPopulationYou:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.PlayerPopulation = byte(clampInt(int(level.PlayerPopulation)+delta, 1, 30))
		})
	case g.optionCursor == optionRowPopulationHim:
		g.updateCurrentLevel(func(level *populous.Level) {
			level.EnemyPopulation = byte(clampInt(int(level.EnemyPopulation)+delta, 1, 30))
		})
	case g.optionCursor >= optionRowFirstPower:
		g.togglePower(delta)
	}
}

func (g *Game) toggleOption() {
	if g.optionCursor >= optionRowFirstPower {
		g.updateCurrentLevel(func(level *populous.Level) {
			bit := byte(1 << (g.optionCursor - optionRowFirstPower))
			level.PlayerPowers ^= bit
			level.EnemyPowers ^= bit
		})
		return
	}
	g.adjustOption(1)
}

func (g *Game) togglePower(delta int) {
	power := g.optionCursor - optionRowFirstPower
	if power < 0 || power >= len(optionPowerNames) {
		return
	}
	g.updateCurrentLevel(func(level *populous.Level) {
		bit := byte(1 << power)
		if delta < 0 {
			level.PlayerPowers ^= bit
			return
		}
		level.EnemyPowers ^= bit
	})
}

func (g *Game) updateCurrentLevel(update func(*populous.Level)) {
	if len(g.bundle.Levels) == 0 {
		return
	}
	level := g.bundle.Levels[g.levelIndex]
	update(&level)
	g.bundle.Levels[g.levelIndex] = level
	g.setLevel(g.levelIndex)
}

func buildModeIndex(mode byte) int {
	switch {
	case mode&populous.GameNoBuild != 0:
		return 3
	case mode&populous.GameRaiseTown != 0:
		return 2
	case mode&populous.GameOnlyRaise != 0:
		return 1
	default:
		return 0
	}
}

func setBuildMode(level *populous.Level, index int) {
	level.GameMode &^= populous.GameNoBuild | populous.GameOnlyRaise | populous.GameRaiseTown
	switch index {
	case 1:
		level.GameMode |= populous.GameOnlyRaise
	case 2:
		level.GameMode |= populous.GameRaiseTown
	case 3:
		level.GameMode |= populous.GameNoBuild
	}
}

func (g *Game) toggleMagnetMode() {
	if g.mode == ModeMagnet {
		g.mode = ModeSculpt
		if g.world != nil {
			g.world.SetMagnetMode(g.player, populous.SettleMode)
		}
		return
	}
	g.mode = ModeMagnet
	if g.world != nil {
		g.world.SetMagnetMode(g.player, populous.MagnetMode)
	}
}

func (g *Game) setActionMode(mode ActionMode) {
	g.mode = mode
}

func (g *Game) setPlayerTendency(tendency int) {
	g.mode = ModeSculpt
	if g.world != nil {
		g.world.SetMagnetMode(g.player, tendency)
	}
}

func (g *Game) handleTargetPowerInput() bool {
	if g.world == nil || !g.humanControlsPlayer() || g.atlasView || (g.mode != ModeMagnet && g.mode != ModeSwamp) || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return false
	}
	mouseX, mouseY := ebiten.CursorPosition()
	if mapX, mapY, ok := miniMapTileAt(mouseX, mouseY); ok {
		g.applyTargetPower(mapX, mapY)
		return true
	}
	if g.hoverOK {
		g.applyTargetPower(g.hoverX, g.hoverY)
		return true
	}
	return false
}

func (g *Game) applyTargetPower(mapX, mapY int) {
	switch g.mode {
	case ModeMagnet:
		g.world.SetMagnetToTile(g.player, mapX, mapY)
	case ModeSwamp:
		if g.world.SwampAtTile(g.player, mapX, mapY) {
			g.mode = ModeSculpt
		}
	}
}

func (g *Game) handleIconInput() bool {
	if g.world == nil || g.atlasView {
		return false
	}
	mouseX, mouseY := ebiten.CursorPosition()
	iconX, iconY, ok := controlIconAt(mouseX, mouseY)
	if !ok {
		return false
	}

	leftPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rightPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	if !leftPressed && !rightPressed {
		return false
	}
	justPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if iconX >= 3 && iconX <= 5 && iconY >= 0 && iconY <= 2 {
		if !justPressed && g.tick%4 != 0 {
			return true
		}
		g.handleDirectionIcon(iconX, iconY)
		return true
	}
	if !justPressed {
		return true
	}
	g.handleActionIcon(iconX, iconY, rightPressed)
	return true
}

func (g *Game) handleDirectionIcon(iconX, iconY int) {
	if iconX == 4 && iconY == 1 {
		g.centerOnPlayerLeader()
		return
	}
	dx := 0
	if iconX == 3 {
		dx = -1
	} else if iconX == 5 {
		dx = 1
	}
	dy := 0
	if iconY == 0 {
		dy = -1
	} else if iconY == 2 {
		dy = 1
	}
	if dx != 0 || dy != 0 {
		g.scrollView(dx, dy)
	}
}

func (g *Game) handleActionIcon(iconX, iconY int, rightButton bool) {
	if iconX == 0 && iconY == 3 {
		g.toggleMusic()
		return
	}
	if iconX == 0 && iconY == 4 {
		g.toggleEffects()
		return
	}
	if !g.humanControlsPlayer() {
		switch {
		case iconX == 7 && iconY == 0:
			if rightButton {
				g.centerOnPlayerLeader()
			} else {
				g.centerOnMapPos(g.world.Magnets[g.player].GoTo)
			}
		case iconX == 7 && iconY == 1:
			g.centerOnNextBattle()
		case iconX == 8 && iconY == 0:
			g.centerOnNextOwnPeep(rightButton)
		}
		return
	}
	switch iconX {
	case 0:
		if iconY == 0 {
			g.world.Flood(g.player)
		}
	case 1:
		if iconY == 0 {
			g.world.WarPower(g.player)
		} else if iconY == 1 {
			g.world.VolcanoAtTile(g.player, g.xoff, g.yoff)
		}
	case 2:
		switch iconY {
		case 0:
			g.world.QuakeAtTile(g.player, g.xoff, g.yoff)
		case 1:
			g.world.Knight(g.player)
		case 2:
			g.setActionMode(ModeSwamp)
		}
	case 3:
		if iconY == 3 {
			g.setPlayerTendency(populous.MagnetMode)
		}
	case 4:
		if iconY == 3 {
			g.setPlayerTendency(populous.SettleMode)
		} else if iconY == 4 {
			g.setPlayerTendency(populous.FightMode)
		}
	case 5:
		if iconY == 3 {
			g.setPlayerTendency(populous.JoinMode)
		}
	case 6:
		if iconY == 0 || iconY == 1 {
			g.setActionMode(ModeSculpt)
		} else if iconY == 2 {
			g.setActionMode(ModeMagnet)
		}
	case 7:
		if iconY == 0 {
			if rightButton {
				g.centerOnPlayerLeader()
			} else {
				g.centerOnMapPos(g.world.Magnets[g.player].GoTo)
			}
		} else if iconY == 1 {
			g.centerOnNextBattle()
		}
	case 8:
		if iconY == 0 {
			g.centerOnNextOwnPeep(rightButton)
		}
	}
}

func (g *Game) toggleMusic() {
	if g.sound == nil {
		return
	}
	g.sound.SetMusicEnabled(!g.sound.MusicEnabled())
}

func (g *Game) toggleEffects() {
	if g.sound == nil {
		return
	}
	g.sound.SetEffectsEnabled(!g.sound.EffectsEnabled())
}

func (g *Game) handleMouseNavigation() bool {
	if g.world == nil || g.atlasView || g.mode == ModeMagnet || g.mode == ModeSwamp || !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		return false
	}
	x, y := ebiten.CursorPosition()
	if mapX, mapY, ok := miniMapTileAt(x, y); ok {
		g.centerOnMapTile(mapX, mapY)
		return true
	}

	justPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if !justPressed && g.tick%4 != 0 {
		return false
	}
	if dx, dy, ok := directionFromOriginalControls(x, y); ok {
		g.scrollView(dx, dy)
		return true
	}
	return false
}

func (g *Game) handleSculptInput() {
	if g.world == nil || !g.humanControlsPlayer() || g.atlasView || g.mode != ModeSculpt || !g.hoverOK {
		return
	}
	if !g.paintMap && !g.playerCanSculptView(byte(g.player)) {
		return
	}
	left := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	right := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	if !left && !right {
		return
	}
	justPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if !justPressed && g.tick%4 != 0 {
		return
	}
	if right {
		if g.paintMap {
			g.world.PaintLowerAt(g.hoverX, g.hoverY)
			return
		}
		g.world.LowerAt(g.player, g.hoverX, g.hoverY)
		return
	}
	if g.paintMap {
		g.world.PaintRaiseAt(g.hoverX, g.hoverY)
		return
	}
	g.world.RaiseAt(g.player, g.hoverX, g.hoverY)
}

func (g *Game) updateHoverTile() {
	g.hoverOK = false
	if g.world == nil || g.atlasView {
		return
	}
	mouseX, mouseY := ebiten.CursorPosition()
	mapX, mapY, localX, localY, ok := g.viewportTileAt(mouseX, mouseY)
	if !ok {
		return
	}
	g.hoverX = mapX
	g.hoverY = mapY
	g.hoverLocalX = localX
	g.hoverLocalY = localY
	g.hoverOK = true
}

func miniMapTileAt(mouseX, mouseY int) (int, int, bool) {
	mapX := mouseY + (mouseX >> 1) - 32
	mapY := mouseY - (mouseX >> 1) + 32
	if mapX < 0 || mapX >= populous.MapWidth || mapY < 0 || mapY >= populous.MapHeight {
		return 0, 0, false
	}
	return mapX, mapY, true
}

func directionFromOriginalControls(mouseX, mouseY int) (int, int, bool) {
	iconX, iconY, ok := controlIconAt(mouseX, mouseY)
	if !ok || iconX < 3 || iconX > 5 || iconY < 0 || iconY > 2 {
		return 0, 0, false
	}
	dx := 0
	if iconX == 3 {
		dx = -1
	} else if iconX == 5 {
		dx = 1
	}
	dy := 0
	if iconY == 0 {
		dy = -1
	} else if iconY == 2 {
		dy = 1
	}
	if dx == 0 && dy == 0 {
		return 0, 0, false
	}
	return dx, dy, true
}

func controlIconAt(mouseX, mouseY int) (int, int, bool) {
	mapX := mouseY + (mouseX >> 1) - 32
	mapY := mouseY - (mouseX >> 1) + 32
	if mapY < 146 {
		return 0, 0, false
	}
	if mapX < 96 || mapX >= 96+9*16 || mapY < 144 || mapY >= 144+5*16 {
		return 0, 0, false
	}
	iconX := (mapX - 96) / 16
	iconY := (mapY - 144) / 16
	if iconX < 0 || iconX > 8 || iconY < 0 || iconY > 4 {
		return 0, 0, false
	}
	return iconX, iconY, true
}

func (g *Game) viewportTileAt(mouseX, mouseY int) (int, int, int, int, bool) {
	if g.world == nil {
		return 0, 0, 0, 0, false
	}
	bestScore := 1 << 30
	bestMapX, bestMapY := 0, 0
	bestLocalX, bestLocalY := 0, 0
	for localY := 0; localY < 8; localY++ {
		for localX := 0; localX < 8; localX++ {
			mapX := g.xoff + localX
			mapY := g.yoff + localY
			pos := mapX + mapY*populous.MapWidth
			topLeftX, topLeftY := blockScreenPosition(localX, localY, int(g.world.MapAlt[pos])<<3)
			centerX := topLeftX + populous.BlockWidth/2
			centerY := topLeftY + 8
			dx := absInt(mouseX - centerX)
			dy := absInt(mouseY - centerY)
			score := dx*8 + dy*16
			if score > 128 || score >= bestScore {
				continue
			}
			bestScore = score
			bestMapX = mapX
			bestMapY = mapY
			bestLocalX = localX
			bestLocalY = localY
		}
	}
	if bestScore == 1<<30 {
		return 0, 0, 0, 0, false
	}
	return bestMapX, bestMapY, bestLocalX, bestLocalY, true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 0, 255})
	switch g.state {
	case StateTitle:
		g.drawTitle(screen)
	case StateSetup:
		g.drawSetup(screen)
	case StateOptions:
		g.drawOptions(screen)
	case StateGame:
		g.drawGame(screen)
	case StateEnd:
		g.drawGame(screen)
		g.drawEndScreen(screen)
	case StateHelp:
		g.drawHelp(screen)
	case StateLord:
		g.drawLordScreen(screen)
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return logicalWidth, logicalHeight
}

func (g *Game) setLevel(index int) {
	if len(g.bundle.Levels) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(g.bundle.Levels) {
		index = len(g.bundle.Levels) - 1
	}
	g.levelIndex = index
	g.tutorialActive = false
	g.tutorialPaused = false
	g.viewFight = 0
	g.viewPeople = 0
	level := g.bundle.Levels[g.levelIndex]
	rules := populous.DefaultTerrainRules()
	if terrain := int(level.Terrain); terrain >= 0 && terrain < len(g.bundle.TerrainRules) {
		rules = g.bundle.TerrainRules[terrain]
	}
	g.world = populous.GenerateWorldWithRules(level, rules)
	g.world.SetScorePlayer(g.player)
	g.world.ComputerControlled = g.computerControlled
	if g.mode == ModeMagnet {
		g.world.SetMagnetMode(g.player, populous.MagnetMode)
	}
	g.centerOnPlayerLeader()
	g.endLost = false
	g.endSummary = [2]populous.PlayerSummary{}
	g.endScore = 0
	g.endNextLevel = g.levelIndex
	if g.sound != nil {
		g.sound.ResetHeartbeat()
	}
}

func (g *Game) centerOnPlayerLeader() {
	if g.world == nil {
		g.xoff = 0
		g.yoff = 0
		return
	}
	if carried := g.world.Magnets[g.player].Carried; carried > 0 && carried <= len(g.world.Peeps) {
		g.centerOnMapPos(g.world.Peeps[carried-1].AtPos)
		return
	}
	for _, peep := range g.world.Peeps {
		if peep.Population > 0 && int(peep.Player) == g.player {
			g.centerOnMapPos(peep.AtPos)
			return
		}
	}
	g.xoff = 0
	g.yoff = 0
}

func (g *Game) centerOnMapPos(pos int) {
	if pos < 0 || pos >= populous.MapWidth*populous.MapHeight {
		return
	}
	g.centerOnMapTile(pos%populous.MapWidth, pos/populous.MapWidth)
}

func (g *Game) centerOnMapTile(x, y int) {
	g.xoff = x - 3
	g.yoff = y - 3
	g.clampView()
}

func (g *Game) centerOnNextBattle() {
	if g.world == nil || len(g.world.Peeps) == 0 {
		return
	}
	for tries := 0; tries < len(g.world.Peeps); tries++ {
		g.viewFight = (g.viewFight + 1) % len(g.world.Peeps)
		peep := g.world.Peeps[g.viewFight]
		if peep.Population > 0 && peep.Flags&populous.InBattle != 0 {
			g.centerOnMapPos(peep.AtPos)
			return
		}
	}
}

func (g *Game) centerOnNextOwnPeep(headedOnly bool) {
	if g.world == nil || len(g.world.Peeps) == 0 {
		return
	}
	for tries := 0; tries < len(g.world.Peeps); tries++ {
		g.viewPeople = (g.viewPeople + 1) % len(g.world.Peeps)
		peep := g.world.Peeps[g.viewPeople]
		if peep.Population <= 0 || int(peep.Player) != g.player {
			continue
		}
		if headedOnly && !peepLooksLikeKnight(peep) {
			continue
		}
		if !headedOnly && peep.Flags != populous.InTown {
			continue
		}
		g.centerOnMapPos(peep.AtPos)
		return
	}
}

func (g *Game) scrollView(dx, dy int) {
	g.xoff += dx
	g.yoff += dy
	g.clampView()
}

func (g *Game) clampView() {
	g.xoff = clampInt(g.xoff, 0, populous.MapWidth-8)
	g.yoff = clampInt(g.yoff, 0, populous.MapHeight-8)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func wrapInt(value, size int) int {
	if size <= 0 {
		return 0
	}
	value %= size
	if value < 0 {
		value += size
	}
	return value
}

func (g *Game) drawTitle(screen *ebiten.Image) {
	if img := g.images["load"]; img != nil {
		screen.DrawImage(img, nil)
	} else if img := g.images["demo"]; img != nil {
		screen.DrawImage(img, nil)
	}
	g.drawPanel(screen)
	code := "GENESIS"
	levelLine := "No level.dat"
	if len(g.bundle.Levels) > 0 {
		lvl := g.bundle.Levels[g.levelIndex]
		code = lvl.Code
		levelLine = fmt.Sprintf("WORLD %03d  %s", lvl.Number, lvl.Code)
	}
	input := g.titleCode
	if input == "" {
		input = code
	}
	g.drawTitleMenu(screen)
	ebitenutil.DebugPrintAt(screen, "POPULOUS", 8, 204)
	ebitenutil.DebugPrintAt(screen, levelLine, 86, 204)
	ebitenutil.DebugPrintAt(screen, "CODE "+input, 8, 216)
	help := "UP/DOWN MENU  TYPE CODE  <-/-> WORLD"
	if g.titleMessage != "" {
		help = g.titleMessage
	}
	ebitenutil.DebugPrintAt(screen, help, 8, 228)
}

func (g *Game) drawTitleMenu(screen *ebiten.Image) {
	const menuX = 224
	const menuY = 132
	const menuWidth = 88
	const lineHeight = 12
	ebitenutil.DrawRect(screen, menuX-4, menuY-6, menuWidth+8, float64(len(titleMenuItems)*lineHeight+8), color.RGBA{R: 8, G: 8, B: 8, A: 210})
	for i, item := range titleMenuItems {
		y := menuY + i*lineHeight
		if i == g.titleMenuCursor && g.titleCode == "" {
			ebitenutil.DrawRect(screen, menuX-2, float64(y-1), menuWidth+4, 9, color.RGBA{R: 72, G: 58, B: 18, A: 255})
		}
		prefix := " "
		if i == g.titleMenuCursor && g.titleCode == "" {
			prefix = ">"
		}
		ebitenutil.DebugPrintAt(screen, prefix+item.label, menuX, y)
	}
}

func (g *Game) drawSetup(screen *ebiten.Image) {
	if g.setupReturn == StateTitle {
		g.drawTitle(screen)
	} else {
		g.drawGame(screen)
	}

	panelX := 16.0
	panelY := 16.0
	panelW := 288.0
	panelH := 176.0
	ebitenutil.DrawRect(screen, panelX, panelY, panelW, panelH, color.RGBA{R: 8, G: 8, B: 8, A: 238})
	frame := color.RGBA{R: 214, G: 196, B: 128, A: 255}
	ebitenutil.DrawRect(screen, panelX, panelY, panelW, 2, frame)
	ebitenutil.DrawRect(screen, panelX, panelY+panelH-2, panelW, 2, frame)
	ebitenutil.DrawRect(screen, panelX, panelY, 2, panelH, frame)
	ebitenutil.DrawRect(screen, panelX+panelW-2, panelY, 2, panelH, frame)

	ebitenutil.DebugPrintAt(screen, "GAME SETUP", 128, 32)
	for i, item := range setupItems {
		x := 16 + item.x
		y := 16 + item.y
		if i == g.setupCursor {
			ebitenutil.DrawRect(screen, float64(x-3), float64(y-1), float64(item.width+20), 9, color.RGBA{R: 72, G: 58, B: 18, A: 255})
		}
		text := g.setupLine(item.action)
		if i == g.setupCursor {
			text = ">" + text
		} else {
			text = " " + text
		}
		ebitenutil.DebugPrintAt(screen, text, x, y)
	}

	g.drawPanel(screen)
	message := "UP/DOWN SELECT  ENTER ACTION  ESC PLAY"
	if g.setupReturn == StateTitle {
		message = "UP/DOWN SELECT  ENTER ACTION  ESC TITLE"
	}
	ebitenutil.DebugPrintAt(screen, message, 6, 204)
	ebitenutil.DebugPrintAt(screen, "T TITLE SCREEN  F2 OPTIONS", 6, 216)
	if g.setupMessage != "" {
		ebitenutil.DebugPrintAt(screen, g.setupMessage, 6, 228)
	} else {
		ebitenutil.DebugPrintAt(screen, "GOOD/EVIL AND PPC VS PPC ACTIVE", 6, 228)
	}
}

func (g *Game) setupLine(action int) string {
	switch action {
	case setupPlay:
		return checkedText(true, "PLAY GAME")
	case setupPaint:
		return checkedText(g.paintMap, "PAINT MAP")
	case setupGood:
		return checkedText(g.player == populous.GodPlayer, "GOOD")
	case setupEvil:
		return checkedText(g.player == populous.DevilPlayer, "EVIL")
	case setupHuman:
		return checkedText(g.humanControlsPlayer() && g.computerControlled[g.opponent()], "HUMAN VS PPC")
	case setupPPC:
		return checkedText(g.computerControlled[populous.GodPlayer] && g.computerControlled[populous.DevilPlayer], "PPC VS PPC")
	case setupConquest:
		return checkedText(true, "CONQUEST")
	case setupCustom:
		return checkedText(false, "CUSTOM GAME")
	case setupOptions:
		return checkedText(false, "GAME OPTIONS")
	case setupSave:
		return checkedText(false, "SAVE A GAME")
	case setupLoad:
		return checkedText(false, "LOAD A GAME")
	case setupNextMap:
		return checkedText(false, "MOVE TO NEXT MAP")
	case setupRestart:
		return checkedText(false, "RESTART THIS MAP")
	case setupSurrender:
		return checkedText(false, "SURRENDER THIS GAME")
	default:
		return ""
	}
}

func checkedText(checked bool, text string) string {
	if checked {
		return "[X] " + text
	}
	return "[ ] " + text
}

func (g *Game) drawOptions(screen *ebiten.Image) {
	if img := g.images["load"]; img != nil {
		screen.DrawImage(img, nil)
	} else if img := g.images["demo"]; img != nil {
		screen.DrawImage(img, nil)
	}
	ebitenutil.DrawRect(screen, 8, 8, 304, 188, color.RGBA{R: 8, G: 8, B: 8, A: 238})
	frame := color.RGBA{R: 214, G: 196, B: 128, A: 255}
	ebitenutil.DrawRect(screen, 8, 8, 304, 2, frame)
	ebitenutil.DrawRect(screen, 8, 194, 304, 2, frame)
	ebitenutil.DrawRect(screen, 8, 8, 2, 188, frame)
	ebitenutil.DrawRect(screen, 310, 8, 2, 188, frame)
	ebitenutil.DebugPrintAt(screen, "WORLD TO CONQUER", 96, 18)
	ebitenutil.DebugPrintAt(screen, "                 YOU    HIM", 142, 32)

	for row := 0; row < optionRowCount; row++ {
		y := 42 + row*10
		if row == g.optionCursor {
			ebitenutil.DrawRect(screen, 14, float64(y-1), 292, 9, color.RGBA{R: 72, G: 58, B: 18, A: 255})
		}
		prefix := " "
		if row == g.optionCursor {
			prefix = ">"
		}
		ebitenutil.DebugPrintAt(screen, prefix+g.optionLine(row), 16, y)
	}

	g.drawPanel(screen)
	ebitenutil.DebugPrintAt(screen, "OPTIONS: UP/DOWN SELECT  LEFT/RIGHT ADJUST", 6, 204)
	ebitenutil.DebugPrintAt(screen, "ENTER/ESC TITLE  LEFT CLICK +  RIGHT CLICK -", 6, 216)
	ebitenutil.DebugPrintAt(screen, "POWER ROWS: LEFT YOU  RIGHT HIM  SPACE BOTH", 6, 228)
}

func (g *Game) optionLine(row int) string {
	if len(g.bundle.Levels) == 0 {
		return "NO LEVEL.DAT"
	}
	level := g.bundle.Levels[g.levelIndex]
	switch row {
	case optionRowWorld:
		return fmt.Sprintf("BATTLE NUMBER IS   %03d  %s", level.Number, level.Code)
	case optionRowTerrain:
		return fmt.Sprintf("WORLDS LANDSCAPE IS %-18s", terrainName(level.Terrain))
	case optionRowReaction:
		return fmt.Sprintf("HIS REACTIONS ARE   %-10s %2d", speedName(level.EnemyReactionSpeed), level.EnemyReactionSpeed)
	case optionRowRating:
		return fmt.Sprintf("HIS RATING IS       %-10s %2d", ratingName(level.EnemyRating), level.EnemyRating)
	case optionRowBuild:
		return fmt.Sprintf("LAND                %-20s", buildModeNames[buildModeIndex(level.GameMode)])
	case optionRowSwamps:
		text := "SHALLOW"
		if level.GameMode&populous.GameSwampRemain != 0 {
			text = "BOTTOMLESS"
		}
		return fmt.Sprintf("THE SWAMPS ARE      %-20s", text)
	case optionRowWater:
		text := "HARMFUL"
		if level.GameMode&populous.GameWaterFatal != 0 {
			text = "FATAL"
		}
		return fmt.Sprintf("WATER IS            %-20s", text)
	case optionRowPopulationYou:
		return fmt.Sprintf("POPULATION YOU      %3d", level.PlayerPopulation)
	case optionRowPopulationHim:
		return fmt.Sprintf("POPULATION HIM             %3d", level.EnemyPopulation)
	default:
		power := row - optionRowFirstPower
		if power < 0 || power >= len(optionPowerNames) {
			return ""
		}
		bit := byte(1 << power)
		return fmt.Sprintf("%-19s %-3s    %-3s", optionPowerNames[power], yesNo(level.PlayerPowers&bit != 0), yesNo(level.EnemyPowers&bit != 0))
	}
}

func terrainName(value byte) string {
	index := int(value)
	if index < 0 || index >= len(terrainNames) {
		index = 0
	}
	return terrainNames[index]
}

func speedName(value byte) string {
	index := (10 - int(value)) / 2
	return speedNames[clampInt(index, 0, len(speedNames)-1)]
}

func ratingName(value byte) string {
	index := (10 - int(value)) / 2
	return ratingNames[clampInt(index, 0, len(ratingNames)-1)]
}

func yesNo(ok bool) string {
	if ok {
		return "YES"
	}
	return "NO"
}

func (g *Game) drawGame(screen *ebiten.Image) {
	if img := g.images["qaz"]; img != nil {
		screen.DrawImage(img, nil)
	}
	if g.world != nil {
		g.drawWorld(screen)
		g.drawMiniMap(screen)
		g.drawInterfaceGauges(screen)
	}
	if g.atlasView {
		g.drawAtlasPreview(screen)
	}
	g.drawPanel(screen)

	line := "No level.dat"
	if g.tutorialActive && g.world != nil {
		lvl := g.world.Level
		line = fmt.Sprintf("%s  LAND %d  POP %d/%d", lvl.Code, lvl.Terrain, lvl.PlayerPopulation, lvl.EnemyPopulation)
	} else if len(g.bundle.Levels) > 0 {
		lvl := g.bundle.Levels[g.levelIndex]
		line = fmt.Sprintf("LV %03d %s  LAND %d  POP %d/%d", lvl.Number, lvl.Code, lvl.Terrain, lvl.PlayerPopulation, lvl.EnemyPopulation)
	}
	ebitenutil.DebugPrintAt(screen, line, 6, 204)
	ebitenutil.DebugPrintAt(screen, "ICONS ACTIVE  L/R: RAISE/LOWER  1 SET 2 JOIN 3 FIGHT", 6, 216)
	ebitenutil.DebugPrintAt(screen, g.statusLine(), 6, 228)
	if g.tutorialActive {
		g.drawTutorialOverlay(screen)
	}

	for i, warning := range g.bundle.Warnings {
		if i >= 2 {
			break
		}
		ebitenutil.DebugPrintAt(screen, warning, 6, 8+i*12)
	}
}

func (g *Game) drawEndScreen(screen *ebiten.Image) {
	panelX := 16.0
	panelY := 16.0
	panelW := 288.0
	panelH := 166.0
	ebitenutil.DrawRect(screen, panelX, panelY, panelW, panelH, color.RGBA{R: 12, G: 12, B: 12, A: 232})
	frame := color.RGBA{R: 214, G: 196, B: 128, A: 255}
	ebitenutil.DrawRect(screen, panelX, panelY, panelW, 2, frame)
	ebitenutil.DrawRect(screen, panelX, panelY+panelH-2, panelW, 2, frame)
	ebitenutil.DrawRect(screen, panelX, panelY, 2, panelH, frame)
	ebitenutil.DrawRect(screen, panelX+panelW-2, panelY, 2, panelH, frame)

	title := "GAME WON"
	if g.endLost {
		title = "GAME LOST"
	} else if g.tutorialActive {
		title = "TUTORIAL WON"
	}
	ebitenutil.DebugPrintAt(screen, title, 120, 28)
	ebitenutil.DebugPrintAt(screen, "                   YOU  HIM", 32, 46)

	you := g.endSummary[g.player]
	him := g.endSummary[g.opponent()]
	lines := []string{
		fmt.Sprintf("BATTLES WON        %4d %4d", you.BattlesWon, him.BattlesWon),
		fmt.Sprintf("NUMBER OF KNIGHTS  %4d %4d", you.Knights, him.Knights),
		fmt.Sprintf("NUMBER OF TOWNS    %4d %4d", you.Towns, him.Towns),
		fmt.Sprintf("NUMBER OF CASTLES  %4d %4d", you.Castles, him.Castles),
		fmt.Sprintf("YOUR SCORE         %6d", g.endScore),
	}
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, 32, 62+i*14)
	}
	footer := "ENTER: LORD"
	if g.endLost {
		footer = "ENTER: RESTART LEVEL"
	} else if g.tutorialActive {
		footer = "ENTER: TITLE"
	} else if len(g.bundle.Levels) > 0 {
		next := g.bundle.Levels[clampInt(g.endNextLevel, 0, len(g.bundle.Levels)-1)]
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("NEXT WORLD %03d  %s", next.Number, next.Code), 74, 142)
	}
	ebitenutil.DebugPrintAt(screen, footer, 74, 156)
	ebitenutil.DebugPrintAt(screen, "ESC: TITLE", 112, 170)
}

func (g *Game) drawLordScreen(screen *ebiten.Image) {
	if img := g.images["lord"]; img != nil {
		screen.DrawImage(img, nil)
	} else {
		drawRequester(screen, 8, 8, 304, 192)
		ebitenutil.DebugPrintAt(screen, "MESSAGE FROM YOUR LORD", 72, 88)
	}

	g.drawLordMouth(screen)
	for i, line := range g.lordTextLines() {
		ebitenutil.DebugPrintAt(screen, line, 8, 178+i*8)
	}

	g.drawPanel(screen)
	next := g.levelCode(g.endNextLevel)
	if g.conquestCompleted() {
		ebitenutil.DebugPrintAt(screen, "CONQUEST COMPLETE  TRY "+next, 8, 204)
	} else {
		ebitenutil.DebugPrintAt(screen, "NEXT WORLD "+next, 8, 204)
	}
	ebitenutil.DebugPrintAt(screen, "ENTER/SPACE/CLICK CONTINUE", 8, 216)
	ebitenutil.DebugPrintAt(screen, "ESC TITLE", 8, 228)
}

func (g *Game) lordTextLines() []string {
	next := g.levelCode(g.endNextLevel)
	if g.conquestCompleted() {
		return []string{
			"WELL DONE YOU HAVE CONQUERED EVIL",
			"THE BATTLE IS OVER BUT TRY " + next,
		}
	}
	rankIndex := clampInt((g.endNextLevel*5)/250, 0, len(conquestRankNames)-1)
	return []string{
		"WELL DONE " + conquestRankNames[rankIndex] + " YOU CONQUERED",
		g.conqueredLevelCode() + " NOW BATTLE AT " + next,
	}
}

func (g *Game) conqueredLevelCode() string {
	if g.levelIndex == 0 {
		return "GENESIS"
	}
	return g.levelCode(g.levelIndex)
}

func (g *Game) levelCode(index int) string {
	if len(g.bundle.Levels) == 0 {
		return "UNKNOWN"
	}
	index = clampInt(index, 0, len(g.bundle.Levels)-1)
	return g.bundle.Levels[index].Code
}

func (g *Game) drawLordMouth(screen *ebiten.Image) {
	if g.mouths == nil {
		return
	}
	g.drawLordMouthFrame(screen, 9*16, 135, g.lordMouthFrame())
	if g.conquestCompleted() && (g.tick/12)%24 == 16 {
		g.drawLordMouthFrame(screen, 8*16, 75, 5)
	}
}

func (g *Game) lordMouthFrame() int {
	phase := (g.tick / 5) % 8
	if phase > 4 {
		phase = 8 - phase
	}
	return phase
}

func (g *Game) drawLordMouthFrame(screen *ebiten.Image, x, y, frame int) {
	frame = clampInt(frame, 0, populous.MouthFrames-1)
	src := image.Rect(frame*populous.MouthWidth, 0, (frame+1)*populous.MouthWidth, populous.MouthHeight)
	if src.Max.X > g.mouths.Bounds().Dx() || src.Max.Y > g.mouths.Bounds().Dy() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(g.mouths.SubImage(src).(*ebiten.Image), op)
}

func (g *Game) drawTutorialOverlay(screen *ebiten.Image) {
	if !g.tutorialPaused {
		ebitenutil.DebugPrintAt(screen, "TUTORIAL: H HELP  ESC SETUP", 6, 8)
		return
	}
	panelX := 24.0
	panelY := 44.0
	panelW := 272.0
	panelH := 112.0
	drawRequester(screen, panelX, panelY, panelW, panelH)
	lines := []string{
		"TUTORIAL",
		"Original seed 27068, fatal water.",
		"Left/right click raises/lowers land.",
		"Use minimap or arrows to move view.",
		"M or icon 6,2 places the papal magnet.",
		"Build towns, gain mana, then try powers.",
		"ENTER/SPACE/CLICK starts the lesson.",
	}
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, 40, 58+i*13)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	switch g.helpReturn {
	case StateGame:
		g.drawGame(screen)
	case StateSetup:
		g.drawSetup(screen)
	case StateOptions:
		g.drawOptions(screen)
	default:
		g.drawTitle(screen)
	}
	drawRequester(screen, 16, 16, 288, 176)
	lines := []string{
		"HELP",
		"Title: choose Tutorial, Conquest, Custom or Setup.",
		"Code entry accepts original world names (GENESIS).",
		"Game: left/right mouse sculpts where your people are.",
		"Minimap and interface arrows move the viewport.",
		"Icons: flood, armageddon, volcano, quake, knight, swamp.",
		"0,3 toggles music. 0,4 toggles effects/voices.",
		"ESC opens setup. F toggles fullscreen. TAB atlas.",
		"ENTER/SPACE/CLICK closes help.",
	}
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, 28, 30+i*16)
	}
}

func (g *Game) drawInterfaceGauges(screen *ebiten.Image) {
	g.drawManaGauge(screen)
	g.drawPopulationGauges(screen)
}

func (g *Game) drawManaGauge(screen *ebiten.Image) {
	if g.sprites == nil || g.world == nil {
		return
	}
	x, y := manaGaugePosition(g.world.Magnets[g.player].Mana)
	g.drawSpriteFrame(screen, x, y, populous.ManaSprite)
}

func manaGaugePosition(mana int) (int, int) {
	level := 0
	for level < len(manaGaugeValues) && mana > manaGaugeValues[level] {
		level++
	}
	if level > 9 {
		return 311, 87
	}
	if level < 1 {
		level = 1
	}
	previous := manaGaugeValues[level-1]
	next := manaGaugeValues[level]
	step := 0
	if next > previous {
		step = ((mana - previous) * 8) / (next - previous)
	}
	step = clampInt(step, 0, 8)
	return 48 + 128 - 16 + (level-1)*16 + step*2, 8 + (level-1)*8 + step
}

func (g *Game) drawPopulationGauges(screen *ebiten.Image) {
	if g.world == nil {
		return
	}
	drawInterfaceBar(screen, 32, 31, 32, populationGaugeHeight(g.world.PlayerPopulation(populous.GodPlayer)), 15)
	drawInterfaceBar(screen, 39, 31, 32, populationGaugeHeight(g.world.PlayerPopulation(populous.DevilPlayer)), 8)
}

func populationGaugeHeight(population int) int {
	if population <= 0 {
		return 0
	}
	return clampInt((population*31)/50000+1, 0, 32)
}

func drawInterfaceBar(screen *ebiten.Image, xPos, yPos, maxHeight, currentHeight, colorIndex int) {
	left := float64(xPos*8 + 2)
	width := 4.0
	top := float64(yPos - maxHeight)
	height := float64(maxHeight + 1)
	ebitenutil.DrawRect(screen, left, top, width, height, populous.Palette(2, 0))
	if currentHeight <= 0 {
		return
	}
	fillHeight := clampInt(currentHeight+1, 0, maxHeight)
	fillTop := float64(yPos - fillHeight + 1)
	ebitenutil.DrawRect(screen, left, fillTop, width, float64(fillHeight), populous.Palette(colorIndex, 0))
}

func (g *Game) statusLine() string {
	mode := "SCULPT"
	if g.mode == ModeMagnet {
		mode = "MAGNET"
	} else if g.mode == ModeSwamp {
		mode = "SWAMP"
	}
	if g.paintMap {
		mode = "PAINT-" + mode
	}
	mana := 0
	magnet := 0
	tendency := "SETTLE"
	if g.world != nil {
		playerMagnet := g.world.Magnets[g.player]
		mana = playerMagnet.Mana
		magnet = playerMagnet.GoTo
		switch playerMagnet.Flags {
		case populous.MagnetMode:
			tendency = "MAGNET"
		case populous.JoinMode:
			tendency = "JOIN"
		case populous.FightMode:
			tendency = "FIGHT"
		}
	}
	side := "GOOD"
	if g.player == populous.DevilPlayer {
		side = "EVIL"
	}
	control := "HUMAN"
	if !g.humanControlsPlayer() {
		control = "PPC"
	}
	if g.tutorialActive {
		side = "TUTORIAL"
	}
	audioState := "M/E"
	if g.sound != nil {
		if !g.sound.MusicEnabled() && !g.sound.EffectsEnabled() {
			audioState = "--"
		} else if !g.sound.MusicEnabled() {
			audioState = "-/E"
		} else if !g.sound.EffectsEnabled() {
			audioState = "M/-"
		}
	}
	return fmt.Sprintf("%s/%s %s %s TEND %s MANA %d MAG %02d,%02d", side, control, mode, audioState, tendency, mana, magnet%populous.MapWidth, magnet/populous.MapWidth)
}

func (g *Game) playerCanSculptView(player byte) bool {
	if g.world == nil {
		return false
	}
	return g.world.HasBuildPresence(int(player), g.xoff, g.yoff, 8, 8)
}

func (g *Game) drawAtlasPreview(screen *ebiten.Image) {
	if len(g.lands) == 0 {
		return
	}
	tileW := populous.BlockWidth
	tileH := populous.BlockHeight
	cols := logicalWidth / tileW
	land := 0
	if g.world != nil {
		land = g.world.Terrain
	}
	if land < 0 || land >= len(g.lands) {
		land = 0
	}
	for i := 0; i < populous.BlocksPerLand; i++ {
		src := image.Rect(0, i*tileH, tileW, i*tileH+tileH)
		if src.Max.Y > g.lands[land].Bounds().Dy() {
			return
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64((i%cols)*tileW), float64((i/cols)*tileH))
		screen.DrawImage(g.lands[land].SubImage(src).(*ebiten.Image), op)
	}
	if g.sprites != nil {
		for i := 0; i < 20; i++ {
			src := image.Rect(0, i*populous.SpriteHeight, populous.SpriteWidth, i*populous.SpriteHeight+populous.SpriteHeight)
			if src.Max.Y > g.sprites.Bounds().Dy() {
				break
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Reset()
			op.GeoM.Translate(float64(i*populous.SpriteWidth), 168)
			screen.DrawImage(g.sprites.SubImage(src).(*ebiten.Image), op)
		}
	}
	if g.bigSprites != nil {
		for i := 0; i < 10; i++ {
			src := image.Rect(0, i*populous.BigSpriteHeight, populous.BigSpriteWidth, i*populous.BigSpriteHeight+populous.BigSpriteHeight)
			if src.Max.Y > g.bigSprites.Bounds().Dy() {
				break
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(i*populous.BigSpriteWidth), 184)
			screen.DrawImage(g.bigSprites.SubImage(src).(*ebiten.Image), op)
		}
	}
}

func (g *Game) drawMiniMap(screen *ebiten.Image) {
	if g.world == nil {
		return
	}
	if g.miniMap == nil {
		g.miniMap = ebiten.NewImage(miniMapWidth, miniMapHeight)
		g.miniPixels = make([]byte, miniMapWidth*miniMapHeight*4)
	}
	for i := range g.miniPixels {
		g.miniPixels[i] = 0
	}

	for y := 0; y < populous.MapHeight; y++ {
		for x := 0; x < populous.MapWidth; x++ {
			pos := x + y*populous.MapWidth
			putMiniPixel(g.miniPixels, x, y, g.miniMapColor(pos))
		}
	}
	for _, peep := range g.world.Peeps {
		if peep.Population <= 0 || peep.AtPos < 0 || peep.AtPos >= populous.MapWidth*populous.MapHeight {
			continue
		}
		colorIndex := 8
		if peep.Player == populous.GodPlayer {
			colorIndex = 15
		}
		x := peep.AtPos % populous.MapWidth
		y := peep.AtPos / populous.MapWidth
		putMiniPixel(g.miniPixels, x, y, populous.Palette(colorIndex, 0))
	}
	if magnet := g.world.Magnets[g.player].GoTo; magnet >= 0 && magnet < populous.MapWidth*populous.MapHeight {
		putMiniPixel(g.miniPixels, magnet%populous.MapWidth, magnet/populous.MapWidth, populous.Palette(10, 0))
	}
	g.drawMiniMapViewport()

	g.miniMap.ReplacePixels(g.miniPixels)
	screen.DrawImage(g.miniMap, nil)
}

func (g *Game) miniMapColor(pos int) color.RGBA {
	block := int(g.world.MapBlk[pos])
	overlay := int(g.world.MapBk2[pos])
	if overlay >= populous.FirstTown && overlay <= populous.CityWall2 {
		if block == populous.FarmBlock+populous.DevilPlayer {
			return populous.Palette(1, 0)
		}
		return populous.Palette(5, 0)
	}
	if overlay >= populous.TreeBlock && overlay <= populous.TreeBlock+2 {
		return populous.Palette(11, 0)
	}

	switch block {
	case populous.FarmBlock + populous.GodPlayer:
		return populous.Palette(5, 0)
	case populous.FarmBlock + populous.DevilPlayer:
		return populous.Palette(1, 0)
	case populous.RockBlock, populous.RockBlock + 1, populous.RockBlock + 2:
		return populous.Palette(3, 0)
	case populous.SwampBlock:
		return populous.Palette(2, 0)
	case populous.BadLand:
		return populous.Palette(8, 0)
	case populous.WaterBlock:
		return populous.Palette(g.mapColorIndex(0), 0)
	default:
		return populous.Palette(g.mapColorIndex(block), 0)
	}
}

func (g *Game) mapColorIndex(block int) int {
	if g.world == nil {
		return 0
	}
	index := 25
	if block >= 0 && block < 16 {
		index = int(g.world.Rules.MapColor[block])
	} else if block >= 16 && block < 32 {
		index = int(g.world.Rules.MapColor[block-16])
	}
	if index == 0 && block != populous.WaterBlock {
		index = int(g.world.Rules.MapColor[populous.FlatBlock])
	}
	if index == 25 {
		index = int(g.world.Rules.MapColor[populous.FlatBlock])
	}
	return index
}

func (g *Game) drawMiniMapViewport() {
	view := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	x2 := g.xoff + 7
	y2 := g.yoff + 7
	for x := g.xoff; x <= x2; x++ {
		putMiniPixel(g.miniPixels, x, g.yoff, view)
		putMiniPixel(g.miniPixels, x, y2, view)
	}
	for y := g.yoff; y <= y2; y++ {
		putMiniPixel(g.miniPixels, g.xoff, y, view)
		putMiniPixel(g.miniPixels, x2, y, view)
	}
}

func putMiniPixel(pixels []byte, mapX, mapY int, c color.RGBA) {
	screenX := 64 + mapX - mapY
	screenY := (mapX + mapY) >> 1
	if screenX < 0 || screenX >= miniMapWidth || screenY < 0 || screenY >= miniMapHeight {
		return
	}
	offset := (screenY*miniMapWidth + screenX) * 4
	pixels[offset+0] = c.R
	pixels[offset+1] = c.G
	pixels[offset+2] = c.B
	pixels[offset+3] = c.A
}

func (g *Game) drawWorld(screen *ebiten.Image) {
	land := g.world.Terrain
	if land < 0 || land >= len(g.lands) {
		return
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pos := (g.xoff + x) + (g.yoff+y)*populous.MapWidth
			block := int(g.world.MapBlk[pos])
			if block == populous.WaterBlock && g.tick%2 == 0 {
				block = 16
			}
			g.drawBlock(screen, g.lands[land], x, y, int(g.world.MapAlt[pos])<<3, block)
			if overlay := int(g.world.MapBk2[pos]); overlay != 0 {
				g.drawBlock(screen, g.lands[land], x, y, (int(g.world.MapAlt[pos])<<3)+8, overlay)
			}
		}
	}
	g.drawSideWalls(screen)
	g.drawVisiblePeeps(screen)
	g.drawHoverCursor(screen)
}

func (g *Game) drawBlock(screen *ebiten.Image, atlas *ebiten.Image, x, y, z, block int) {
	if block < 0 || block >= populous.BlocksPerLand {
		return
	}
	dstX, dstY := blockScreenPosition(x, y, z)
	src := image.Rect(0, block*populous.BlockHeight, populous.BlockWidth, (block+1)*populous.BlockHeight)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(dstX), float64(dstY))
	screen.DrawImage(atlas.SubImage(src).(*ebiten.Image), op)
}

func blockScreenPosition(x, y, z int) (int, int) {
	screenOrigin := (8*8*40 + 20 + 2) * 4
	screenX := x << 3
	screenY := y << 3
	screenPos := screenOrigin + screenX - screenY
	screenPos = screenX*160 + screenPos
	screenPos = screenY*160 + screenPos
	screenPos -= 160 * z

	dstX := (screenPos % 160) * 2
	dstY := screenPos / 160
	return dstX, dstY
}

func (g *Game) drawSideWalls(screen *ebiten.Image) {
	if g.sprites == nil {
		return
	}
	for i := 0; i < 8; i++ {
		leftPos := (g.yoff+i)*populous.MapWidth + g.xoff + 7
		for alt := int(g.world.MapAlt[leftPos]); alt > 0; alt-- {
			g.drawFixedSprite(screen, 8, i, alt-1, populous.SideWall1)
		}

		rightPos := (g.yoff+7)*populous.MapWidth + g.xoff + i
		for alt := int(g.world.MapAlt[rightPos]); alt > 0; alt-- {
			g.drawFixedSprite(screen, i+1, 8, alt, populous.SideWall2)
		}
	}
}

func (g *Game) drawHoverCursor(screen *ebiten.Image) {
	if g.sprites == nil || !g.hoverOK {
		return
	}
	pos := g.hoverX + g.hoverY*populous.MapWidth
	if pos < 0 || pos >= populous.MapWidth*populous.MapHeight {
		return
	}
	topLeftX, topLeftY := blockScreenPosition(g.hoverLocalX, g.hoverLocalY, int(g.world.MapAlt[pos])<<3)
	g.drawSpriteFrame(screen, topLeftX+8, topLeftY, populous.CrosshairSprite)
}

func (g *Game) drawVisiblePeeps(screen *ebiten.Image) {
	if g.sprites == nil {
		return
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pos := (g.xoff + x) + (g.yoff+y)*populous.MapWidth
			index := int(g.world.MapWho[pos])
			if index == 0 || index > len(g.world.Peeps) {
				continue
			}
			peep := g.world.Peeps[index-1]
			if peep.Population <= 0 {
				continue
			}
			frame, dx, dy := g.peepSpriteFrame(pos, peep)
			g.drawPeepSprite(screen, x, y, int(g.world.MapAlt[pos])<<3, frame, dx, dy)
		}
	}
}

func (g *Game) peepSpriteFrame(sourcePos int, peep populous.Peep) (frame, dx, dy int) {
	switch {
	case peep.Flags == populous.InTown:
		return populous.FlagSprite + (g.tick & 1) + int(peep.Player)*2, 0, 0
	case peep.Flags&populous.InRuin != 0:
		return populous.FireSprite + (g.tick & 3), 0, 0
	case peep.Flags&populous.InEffect != 0:
		if peepLooksLikeKnight(peep) {
			return peep.Frame + populous.KnightWinSprite - populous.VictorySprite, 0, 0
		}
		return peep.Frame, 0, 0
	case peep.Flags&populous.InBattle != 0:
		return g.battleSpriteFrame(peep), 0, 0
	case peep.Flags&(populous.WaitForMe|populous.IAmWaiting) != 0:
		frame := peep.Frame
		if peepLooksLikeKnight(peep) {
			frame = populous.KnightWaitSprite + (g.tick & 1)
		}
		if peep.Player == populous.DevilPlayer {
			frame += 2
		}
		return frame, 0, 0
	case peep.Flags&populous.InWater != 0:
		frame := populous.FirstWaterSprite + (g.tick & 3)
		if peepLooksLikeKnight(peep) {
			frame = populous.FirstKnightWater + (g.tick & 3)
		}
		if peep.Player == populous.DevilPlayer {
			frame += 4
		}
		return frame, 0, 0
	default:
		stepX, stepY, base := peepMotion(peep.Direction)
		frame := base + (g.tick & 1)
		if peep.Player == populous.DevilPlayer {
			frame += populous.BadPeople
		}
		if peepLooksLikeKnight(peep) {
			frame += populous.KnightPeople
		}
		dx = peep.Frame * stepX
		dy = peep.Frame * stepY
		if target := sourcePos + peep.Direction; target >= 0 && target < populous.MapWidth*populous.MapHeight {
			dy += (int(g.world.MapAlt[sourcePos]) - int(g.world.MapAlt[target])) * peep.Frame
		}
		if int(g.world.MapBlk[sourcePos]) != populous.FlatBlock {
			dy += 4
		}
		return frame, dx, dy
	}
}

func (g *Game) battleSpriteFrame(peep populous.Peep) int {
	if !peepLooksLikeKnight(peep) {
		return peep.Frame
	}
	opponentIsKnight := false
	if peep.BattlePopulation >= 0 && peep.BattlePopulation < len(g.world.Peeps) {
		opponentIsKnight = peepLooksLikeKnight(g.world.Peeps[peep.BattlePopulation])
	}
	offset := peep.Frame - populous.BattleFirstFrame
	if opponentIsKnight {
		return populous.BlueVsRedSprite + offset
	}
	if peep.Player == populous.GodPlayer {
		return populous.BlueVsPeepSprite + offset
	}
	return populous.RedVsPeepSprite + offset
}

func peepLooksLikeKnight(peep populous.Peep) bool {
	return peep.Status == populous.KnightStatus || peep.HeadFor != 0
}

func peepMotion(direction int) (stepX, stepY, baseFrame int) {
	switch direction {
	case -65:
		return 0, -2, 0
	case -64:
		return 2, -1, 2
	case -63:
		return 4, 0, 4
	case 1:
		return 2, 1, 6
	case 65:
		return 0, 2, 8
	case 64:
		return -2, 1, 10
	case 63:
		return -4, 0, 12
	case -1:
		return -2, -1, 14
	default:
		return 0, 0, 8
	}
}

func (g *Game) drawPeepSprite(screen *ebiten.Image, x, y, z, frame, dx, dy int) {
	xPos := x << 3
	yPos := y << 3
	r5 := xPos + yPos - z
	r4 := (xPos << 1) - (yPos << 1)
	dstX := r4 + 320 - 128 - 8 + dx
	dstY := r5 + 64 + dy
	g.drawSpriteFrame(screen, dstX, dstY, frame)
}

func (g *Game) drawFixedSprite(screen *ebiten.Image, x, y, z, frame int) {
	screenOrigin := (8*8*40 + 20 + 2) * 4
	screenX := x << 3
	screenY := y << 3
	screenPos := screenOrigin + screenX - screenY
	screenPos = screenX*160 + screenPos
	screenPos = screenY*160 + screenPos
	screenPos -= 1280 * z

	dstX := (screenPos % 160) * 2
	dstY := screenPos / 160
	g.drawSpriteFrame(screen, dstX, dstY, frame)
}

func (g *Game) drawSpriteFrame(screen *ebiten.Image, dstX, dstY, frame int) {
	if frame < 0 || frame*populous.SpriteHeight >= g.sprites.Bounds().Dy() {
		return
	}
	src := image.Rect(0, frame*populous.SpriteHeight, populous.SpriteWidth, (frame+1)*populous.SpriteHeight)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(dstX), float64(dstY))
	screen.DrawImage(g.sprites.SubImage(src).(*ebiten.Image), op)
}

func (g *Game) drawPanel(screen *ebiten.Image) {
	panel := ebiten.NewImage(populous.ScreenWidth, populous.UIStripe)
	panel.Fill(color.RGBA{8, 8, 8, 245})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, populous.ScreenHeight)
	screen.DrawImage(panel, op)
}

func drawRequester(screen *ebiten.Image, x, y, width, height float64) {
	ebitenutil.DrawRect(screen, x, y, width, height, color.RGBA{R: 8, G: 8, B: 8, A: 238})
	frame := color.RGBA{R: 214, G: 196, B: 128, A: 255}
	ebitenutil.DrawRect(screen, x, y, width, 2, frame)
	ebitenutil.DrawRect(screen, x, y+height-2, width, 2, frame)
	ebitenutil.DrawRect(screen, x, y, 2, height, frame)
	ebitenutil.DrawRect(screen, x+width-2, y, 2, height, frame)
}
