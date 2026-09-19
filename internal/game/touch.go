package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"go-populous/internal/populous"
	"go-populous/internal/touchui"
)

type touchInputState struct {
	ids            []ebiten.TouchID
	justIDs        []ebiten.TouchID
	justContactIDs []int
	contacts       []touchui.Contact
	points         []touchui.Point
	stepPoints     []touchui.Point
	latch          touchui.Latch
}

func (g *Game) updateTouchInput() {
	g.touch.update()
	if g.mobileMenuActive() {
		g.updateMobileFrontInput()
		return
	}
	if g.mobileSceneActive() {
		g.updateMobileInput()
		return
	}
	layout := g.touchLayout()
	if layout.Enabled && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if _, _, inScene := layout.ScenePosition(x, y); !inScene {
			const mouseContactID = -1
			g.touch.contacts = append(g.touch.contacts, touchui.Contact{ID: mouseContactID, X: x, Y: y})
			just := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
			g.touch.points = append(g.touch.points, touchui.Point{X: x, Y: y, Just: just})
			if just {
				g.touch.justContactIDs = append(g.touch.justContactIDs, mouseContactID)
			}
		}
	}
	g.touch.latch.Observe(g.touch.contacts, g.touch.justContactIDs)
}

func (state *touchInputState) update() {
	state.ids = ebiten.AppendTouchIDs(state.ids[:0])
	state.justIDs = inpututil.AppendJustPressedTouchIDs(state.justIDs[:0])
	state.justContactIDs = state.justContactIDs[:0]
	state.contacts = state.contacts[:0]
	state.points = state.points[:0]
	for _, id := range state.ids {
		x, y := ebiten.TouchPosition(id)
		contactID := int(id)
		just := touchIDInSlice(id, state.justIDs)
		state.contacts = append(state.contacts, touchui.Contact{ID: contactID, X: x, Y: y})
		state.points = append(state.points, touchui.Point{
			X:    x,
			Y:    y,
			Just: just,
		})
		if just {
			state.justContactIDs = append(state.justContactIDs, contactID)
		}
	}
}

func (state *touchInputState) prepareStep() {
	state.stepPoints = state.latch.Consume(state.contacts)
}

func touchIDInSlice(id ebiten.TouchID, ids []ebiten.TouchID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func touchLogicalWidth(outsideWidth, outsideHeight int) int {
	return touchui.LogicalWidth(outsideWidth, outsideHeight)
}

func (g *Game) touchLayout() touchui.Layout {
	width := g.layoutWidth
	if width < logicalWidth {
		width = logicalWidth
	}
	return touchui.NewLayout(width)
}

func (g *Game) cursorPosition() (int, int) {
	x, y := ebiten.CursorPosition()
	return x - g.touchLayout().SceneX, y
}

func (g *Game) pointerPosition() (int, int) {
	layout := g.touchLayout()
	for _, point := range g.touch.points {
		if x, y, ok := layout.ScenePosition(point.X, point.Y); ok {
			return x, y
		}
	}
	return g.cursorPosition()
}

// handleTouchInput handles touch-only transitions before keyboard and mouse
// input. It returns true when Update must stop because the current screen was
// left or an acknowledgement consumed the tick.
func (g *Game) handleTouchInput() bool {
	if len(g.touch.stepPoints) == 0 {
		return false
	}
	layout := g.touchLayout()
	controls := touchui.Evaluate(layout, g.touch.stepPoints)

	if controls.MenuJust {
		switch g.state {
		case StateTitle:
			g.openSetup(StateTitle)
		case StateSetup:
			g.returnFromSetup()
		case StateOptions:
			g.state = g.optionReturn
		case StateGame:
			if g.multiplayerEnabled() {
				g.stopMultiplayer()
				g.setLevel(g.levelIndex)
				g.state = StateTitle
			} else {
				g.openSetup(StateGame)
			}
		case StateEnd:
			g.returnToTitleFromTouch()
		case StateHelp:
			g.state = g.helpReturn
		case StateLord:
			g.setLevel(g.levelIndex)
			g.state = StateTitle
		}
		return true
	}

	switch g.state {
	case StateTitle:
		for _, point := range g.touch.stepPoints {
			if !point.Just {
				continue
			}
			x, y, ok := layout.ScenePosition(point.X, point.Y)
			if !ok {
				continue
			}
			if index, hit := titleMenuItemAt(x, y); hit {
				g.titleMenuCursor = index
				g.activateTitleMenu(index)
				return true
			}
		}
	case StateSetup:
		for _, point := range g.touch.stepPoints {
			if !point.Just {
				continue
			}
			x, y, ok := layout.ScenePosition(point.X, point.Y)
			if !ok {
				continue
			}
			if index, hit := setupItemAt(x, y); hit {
				g.setupCursor = index
				g.activateSetupItem(index)
				return true
			}
		}
	case StateOptions:
		for _, point := range g.touch.stepPoints {
			if !point.Just {
				continue
			}
			x, y, ok := layout.ScenePosition(point.X, point.Y)
			if !ok {
				continue
			}
			if row, hit := optionRowAt(x, y); hit {
				g.optionCursor = row
				if controls.Lower {
					g.adjustOption(-1)
				} else {
					g.adjustOption(1)
				}
				return true
			}
		}
		if controls.RaiseJust {
			g.adjustOption(1)
			return true
		}
		if controls.LowerJust {
			g.adjustOption(-1)
			return true
		}
	case StateGame:
		if g.tutorialPaused && controls.AnyJust {
			g.tutorialPaused = false
			g.updateHeartbeat()
			return true
		}
		g.handleGameTouches(layout, controls)
	case StateEnd:
		if controls.AnyJust {
			g.continueEndFromTouch()
			return true
		}
	case StateHelp:
		if controls.AnyJust {
			if g.helpReturn == StateHelp {
				g.helpReturn = StateTitle
			}
			g.state = g.helpReturn
			return true
		}
	case StateLord:
		if controls.AnyJust {
			g.startLordVoice()
			g.continueFromLord()
			return true
		}
	}
	return false
}

func (g *Game) continueEndFromTouch() {
	if g.multiplayerEnabled() {
		g.stopMultiplayer()
		g.setLevel(g.levelIndex)
		g.state = StateTitle
		return
	}
	if g.tutorialActive {
		if g.endLost {
			g.startTutorial()
		} else {
			g.tutorialActive = false
			g.tutorialPaused = false
			g.setLevel(g.levelIndex)
			g.state = StateTitle
		}
		return
	}
	if g.endLost {
		g.setLevel(g.levelIndex)
		g.state = StateGame
		return
	}
	g.openLordScreen()
}

func (g *Game) returnToTitleFromTouch() {
	if g.multiplayerEnabled() {
		g.stopMultiplayer()
	}
	g.tutorialActive = false
	g.tutorialPaused = false
	if g.mobileUI != nil {
		g.mobileUI.status = ""
	}
	g.setLevel(g.levelIndex)
	g.state = StateTitle
}

func (g *Game) handleGameTouches(layout touchui.Layout, controls touchui.Controls) {
	if g.world == nil {
		return
	}
	if controls.DX != 0 || controls.DY != 0 {
		g.scrollView(controls.DX, controls.DY)
	}
	if g.atlasView {
		return
	}

	rightAction := controls.Lower
	for _, point := range g.touch.stepPoints {
		x, y, inScene := layout.ScenePosition(point.X, point.Y)
		if !inScene {
			continue
		}

		iconX, iconY, icon := controlIconAt(x, y)
		if icon {
			if iconX >= 3 && iconX <= 5 && iconY >= 0 && iconY <= 2 {
				if point.Just || g.tick%4 == 0 {
					g.handleDirectionIcon(iconX, iconY)
				}
			} else if point.Just {
				g.handleActionIcon(iconX, iconY, rightAction)
			}
			continue
		}

		switch g.mode {
		case ModeMagnet, ModeSwamp:
			if !point.Just || !g.humanControlsPlayer() {
				continue
			}
			if mapX, mapY, ok := miniMapTileAt(x, y); ok {
				g.applyTargetPower(mapX, mapY)
				return
			}
			if mapX, mapY, _, _, ok := g.viewportTileAt(x, y); ok {
				g.applyTargetPower(mapX, mapY)
				return
			}
		case ModeInspect:
			if !point.Just {
				continue
			}
			if index, ok := g.visiblePeepAt(x, y); ok {
				if rightAction {
					g.setTemporaryViewedPeep(index)
				} else {
					g.setViewedPeep(index)
				}
				return
			}
		case ModeSculpt:
			if mapX, mapY, ok := miniMapTileAt(x, y); ok {
				g.centerOnMapTile(mapX, mapY)
				return
			}
			if !g.humanControlsPlayer() || (!point.Just && g.tick%4 != 0) {
				continue
			}
			if mapX, mapY, _, _, ok := g.viewportTileAt(x, y); ok {
				g.applyTouchSculpt(mapX, mapY, rightAction)
				return
			}
		}
	}
}

func (g *Game) applyTouchSculpt(mapX, mapY int, lower bool) {
	if !g.paintMap && !g.playerCanSculptView(byte(g.player)) {
		return
	}
	kind := populous.CommandRaise
	if g.paintMap {
		kind = populous.CommandPaintRaise
	}
	if lower {
		kind = populous.CommandLower
		if g.paintMap {
			kind = populous.CommandPaintLower
		}
	}
	g.issueCommand(populous.Command{Kind: kind, Player: g.player, X: mapX, Y: mapY})
}

func (g *Game) drawTouchControls(screen *ebiten.Image) {
	layout := g.touchLayout()
	if !layout.Enabled {
		return
	}
	controls := touchui.Evaluate(layout, g.touch.points)

	panel := color.RGBA{R: 15, G: 18, B: 24, A: 255}
	vector.DrawFilledRect(screen, 0, 0, float32(layout.SceneX), float32(logicalHeight), panel, false)
	rightX := layout.SceneX + logicalWidth
	vector.DrawFilledRect(screen, float32(rightX), 0, float32(layout.Width-rightX), float32(logicalHeight), panel, false)

	if g.state == StateGame {
		drawTouchButton(screen, layout.DPad, controls.DX != 0 || controls.DY != 0, "")
		drawDPadArrows(screen, layout.DPad)
	}

	menuLabel := "MENU"
	if g.state != StateTitle && g.state != StateGame {
		menuLabel = "BACK"
	}
	drawTouchButton(screen, layout.Menu, controls.Menu, menuLabel)

	if g.state == StateGame || g.state == StateOptions {
		raiseLabel := "RAISE"
		lowerLabel := "LOWER"
		if g.state == StateOptions {
			raiseLabel = "+"
			lowerLabel = "-"
		}
		drawTouchButton(screen, layout.Raise, controls.Raise, raiseLabel)
		drawTouchButton(screen, layout.Lower, controls.Lower, lowerLabel)
	}
}

func drawTouchButton(screen *ebiten.Image, button touchui.Button, pressed bool, label string) {
	fill := color.RGBA{R: 45, G: 54, B: 69, A: 235}
	stroke := color.RGBA{R: 148, G: 176, B: 211, A: 255}
	if pressed {
		fill = color.RGBA{R: 52, G: 126, B: 91, A: 255}
		stroke = color.RGBA{R: 186, G: 255, B: 215, A: 255}
	}
	vector.DrawFilledCircle(screen, float32(button.X), float32(button.Y), float32(button.Radius), fill, true)
	vector.StrokeCircle(screen, float32(button.X), float32(button.Y), float32(button.Radius), 1.5, stroke, true)
	if label != "" {
		x := button.X - len(label)*3
		ebitenutil.DebugPrintAt(screen, label, x, button.Y-4)
	}
}

func drawDPadArrows(screen *ebiten.Image, button touchui.Button) {
	ink := color.RGBA{R: 226, G: 232, B: 240, A: 255}
	offset := button.Radius * 9 / 16
	half := max(4, button.Radius/7)
	stroke := float32(max(2, button.Radius/16))
	vector.StrokeLine(screen, float32(button.X-half), float32(button.Y-offset+half), float32(button.X), float32(button.Y-offset), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X), float32(button.Y-offset), float32(button.X+half), float32(button.Y-offset+half), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X-half), float32(button.Y+offset-half), float32(button.X), float32(button.Y+offset), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X), float32(button.Y+offset), float32(button.X+half), float32(button.Y+offset-half), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X-offset+half), float32(button.Y-half), float32(button.X-offset), float32(button.Y), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X-offset), float32(button.Y), float32(button.X-offset+half), float32(button.Y+half), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X+offset-half), float32(button.Y-half), float32(button.X+offset), float32(button.Y), stroke, ink, true)
	vector.StrokeLine(screen, float32(button.X+offset), float32(button.Y), float32(button.X+offset-half), float32(button.Y+half), stroke, ink, true)
}
