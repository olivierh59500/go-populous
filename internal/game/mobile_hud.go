package game

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"go-populous/internal/mobileui"
)

var (
	mobileHUDBackground = color.RGBA{R: 17, G: 26, B: 36, A: 255}
	mobileHUDPanel      = color.RGBA{R: 27, G: 40, B: 53, A: 255}
	mobileHUDButton     = color.RGBA{R: 42, G: 58, B: 72, A: 255}
	mobileHUDInk        = color.RGBA{R: 229, G: 235, B: 239, A: 255}
	mobileHUDMuted      = color.RGBA{R: 137, G: 157, B: 171, A: 255}
	mobileHUDAccent     = color.RGBA{R: 255, G: 202, B: 91, A: 255}
	mobileHUDSelected   = color.RGBA{R: 65, G: 85, B: 93, A: 255}
)

// drawMobileHUD only draws presentation state supplied by the input/simulation
// adapter. In particular, opening a panel cannot change the world or its mana.
func (g *Game) drawMobileHUD(screen *ebiten.Image, layout mobileui.HUDLayout, state mobileui.HUDState) {
	vector.DrawFilledRect(screen, 0, 0, float32(layout.Width), 24, mobileHUDBackground, false)
	vector.DrawFilledRect(screen, 0, float32(layout.ToolbarRect.Min.Y), float32(layout.Width), 44, mobileHUDBackground, false)
	ebitenutil.DebugPrintAt(screen, "MANA "+mobileAmount(state.Mana), 6, 4)
	ebitenutil.DebugPrintAt(screen, "PEOPLE "+mobileAmount(state.Population), 111, 4)
	selectedLabel := mobileActionLabel(state.Selected)
	if selectedLabel != "" {
		ebitenutil.DebugPrintAt(screen, selectedLabel, layout.Width-6-len(selectedLabel)*6, 4)
	}
	if layout.Overlay == mobileui.OverlayNone {
		// The mini-map itself is rendered by the terrain adapter below this
		// frame, so both are guaranteed to use the same projection.
		mini := layout.MiniMapRect
		vector.StrokeRect(screen, float32(mini.Min.X-1), float32(mini.Min.Y-1), float32(mini.Dx()+2), float32(mini.Dy()+2), 1, mobileHUDAccent, false)
		if state.Status != "" {
			x := 6
			if state.ShowPad {
				x = layout.DPadRect.Max.X + 6
			}
			message := mobileTruncate(state.Status, max(0, (layout.Width-x-6)/6))
			vector.DrawFilledRect(screen, float32(x-2), float32(layout.MapRect.Max.Y-18), float32(len(message)*6+4), 16, color.RGBA{R: 9, G: 15, B: 22, A: 210}, false)
			ebitenutil.DebugPrintAt(screen, message, x, layout.MapRect.Max.Y-18)
		}
	} else {
		vector.DrawFilledRect(screen, 0, 24, float32(layout.Width), float32(layout.Height-24), color.RGBA{A: 155}, false)
		drawMobileRoundedRect(screen, layout.PanelRect, 6, mobileHUDPanel)
		title := map[mobileui.Overlay]string{
			mobileui.OverlayPowers: "POWERS", mobileui.OverlayPeople: "PEOPLE'S BEHAVIOUR",
			mobileui.OverlayMenu: "GAME MENU", mobileui.OverlaySettings: "TOUCH SETTINGS", mobileui.OverlayConfirm: "CONFIRM POWER",
		}[layout.Overlay]
		ebitenutil.DebugPrintAt(screen, title, layout.PanelRect.Min.X+10, layout.PanelRect.Min.Y+3)
		if layout.Overlay == mobileui.OverlayConfirm {
			for i, line := range mobileWrap(state.Confirmation, (layout.PanelRect.Dx()-24)/6, 3) {
				ebitenutil.DebugPrintAt(screen, line, layout.PanelRect.Min.X+12, layout.PanelRect.Min.Y+29+i*16)
			}
		}
	}
	for _, button := range layout.Buttons {
		drawMobileHUDButton(screen, button, layout.Overlay, state)
	}
	if layout.Overlay != mobileui.OverlayNone && state.Status != "" {
		ebitenutil.DebugPrintAt(screen, mobileTruncate(state.Status, (layout.Width-12)/6), 6, layout.Height-20)
	}
}

func drawMobileHUDButton(screen *ebiten.Image, button mobileui.Button, overlay mobileui.Overlay, state mobileui.HUDState) {
	r, action := button.Rect, button.Action
	fill, ink := mobileHUDButton, mobileHUDInk
	selected := action == state.Selected || action == mobileui.ActionPowers && state.Selected >= mobileui.ActionQuake && state.Selected <= mobileui.ActionArmageddon
	if selected {
		fill, ink = mobileHUDSelected, mobileHUDAccent
	}
	isPower := action >= mobileui.ActionQuake && action <= mobileui.ActionArmageddon
	if isPower && !state.Powers[action].Enabled || action == mobileui.ActionToggleBuildScope && state.Online {
		ink = mobileHUDMuted
	}
	drawMobileRoundedRect(screen, r, 3, fill)
	if selected {
		vector.DrawFilledRect(screen, float32(r.Min.X+5), float32(r.Max.Y-3), float32(r.Dx()-10), 2, mobileHUDAccent, false)
	}
	if action >= mobileui.ActionPanUp && action <= mobileui.ActionPanRight {
		drawMobileActionIcon(screen, action, float32(r.Min.X+r.Dx()/2), float32(r.Min.Y+r.Dy()/2), ink)
		return
	}
	if overlay == mobileui.OverlayNone {
		labels := map[mobileui.Action]string{
			mobileui.ActionRaise: "RAISE", mobileui.ActionLower: "LOWER", mobileui.ActionMagnet: "FLAG", mobileui.ActionInspect: "LOOK",
			mobileui.ActionPowers: "POWER", mobileui.ActionPeople: "TRIBE", mobileui.ActionLeader: "LEAD", mobileui.ActionMenu: "MENU",
		}
		label := labels[action]
		drawMobileActionIcon(screen, action, float32(r.Min.X+r.Dx()/2), float32(r.Min.Y+13), ink)
		ebitenutil.DebugPrintAt(screen, label, r.Min.X+(r.Dx()-len(label)*6)/2, r.Min.Y+25)
		return
	}
	if action >= mobileui.ActionToggleWide && action <= mobileui.ActionToggleBuildScope {
		label, value := "WIDE TERRAIN", "OFF"
		switch action {
		case mobileui.ActionToggleWide:
			if state.Wide {
				value = "ON"
			}
		case mobileui.ActionTogglePad:
			label = "VIRTUAL PAD"
			if state.ShowPad {
				value = "ON"
			}
		case mobileui.ActionToggleBuildScope:
			label, value = "BUILD AREA", "LOCAL 8x8"
			if state.VisibleBuildScope {
				value = "VISIBLE MAP"
			}
			if state.Online {
				value = "LOCKED ONLINE"
			}
		}
		ebitenutil.DebugPrintAt(screen, label, r.Min.X+9, r.Min.Y+8)
		ebitenutil.DebugPrintAt(screen, value, r.Max.X-9-len(value)*6, r.Min.Y+8)
		return
	}
	if action == mobileui.ActionBack || action == mobileui.ActionConfirm || action == mobileui.ActionCancel {
		label := mobileActionLabel(action)
		ebitenutil.DebugPrintAt(screen, label, r.Min.X+(r.Dx()-len(label)*6)/2, r.Min.Y+(r.Dy()-16)/2)
		return
	}
	drawMobileActionIcon(screen, action, float32(r.Min.X+17), float32(r.Min.Y+r.Dy()/2), ink)
	y := r.Min.Y + 8
	if isPower {
		y = r.Min.Y
		cost := "MANA " + mobileAmount(state.Powers[action].Cost)
		if !state.Powers[action].Enabled && state.Mana >= state.Powers[action].Cost {
			cost = "UNAVAILABLE"
		}
		ebitenutil.DebugPrintAt(screen, cost, r.Min.X+33, r.Min.Y+15)
	}
	ebitenutil.DebugPrintAt(screen, mobileActionLabel(action), r.Min.X+33, y)
}

func drawMobileRoundedRect(screen *ebiten.Image, rect image.Rectangle, radius float32, fill color.Color) {
	x, y, w, h := float32(rect.Min.X), float32(rect.Min.Y), float32(rect.Dx()), float32(rect.Dy())
	vector.DrawFilledRect(screen, x+radius, y, w-2*radius, h, fill, false)
	vector.DrawFilledRect(screen, x, y+radius, w, h-2*radius, fill, false)
	for _, corner := range [][2]float32{{x + radius, y + radius}, {x + w - radius, y + radius}, {x + radius, y + h - radius}, {x + w - radius, y + h - radius}} {
		vector.DrawFilledCircle(screen, corner[0], corner[1], radius, fill, true)
	}
}

func drawMobileActionIcon(screen *ebiten.Image, action mobileui.Action, x, y float32, ink color.Color) {
	line := func(x1, y1, x2, y2 float32) {
		vector.StrokeLine(screen, x+x1, y+y1, x+x2, y+y2, 1.7, ink, true)
	}
	circle := func(cx, cy, radius float32) { vector.StrokeCircle(screen, x+cx, y+cy, radius, 1.5, ink, true) }
	switch action {
	case mobileui.ActionRaise, mobileui.ActionLower:
		line(-9, 6, 0, 1)
		line(0, 1, 9, 6)
		line(9, 6, 0, 10)
		line(0, 10, -9, 6)
		if action == mobileui.ActionRaise {
			line(0, 0, 0, -9)
			line(-4, -5, 0, -9)
			line(4, -5, 0, -9)
		} else {
			line(0, -9, 0, 0)
			line(-4, -4, 0, 0)
			line(4, -4, 0, 0)
		}
	case mobileui.ActionMagnet, mobileui.ActionFollow:
		line(-6, 9, -6, -10)
		line(-6, -9, 8, -7)
		line(8, -7, 5, -1)
		line(5, -1, -6, -3)
	case mobileui.ActionInspect:
		circle(-2, -2, 6)
		line(3, 3, 9, 9)
	case mobileui.ActionPowers, mobileui.ActionQuake:
		line(4, -10, -5, 1)
		line(-5, 1, 2, 1)
		line(2, 1, -4, 11)
	case mobileui.ActionPeople, mobileui.ActionJoin:
		circle(-5, -4, 3)
		circle(5, -4, 3)
		line(-5, 0, -5, 8)
		line(5, 0, 5, 8)
		line(-9, 4, -1, 4)
		line(1, 4, 9, 4)
	case mobileui.ActionLeader, mobileui.ActionKnight:
		line(-9, -5, -7, 5)
		line(-7, 5, 7, 5)
		line(7, 5, 9, -5)
		line(9, -5, 3, -1)
		line(3, -1, 0, -8)
		line(0, -8, -3, -1)
		line(-3, -1, -9, -5)
		line(-7, 8, 7, 8)
	case mobileui.ActionMenu:
		for _, d := range []float32{-6, 0, 6} {
			line(-8, d, 8, d)
		}
	case mobileui.ActionSwamp, mobileui.ActionFlood:
		for _, d := range []float32{-5, 1, 7} {
			line(-9, d, -4, d-2)
			line(-4, d-2, 1, d)
			line(1, d, 6, d-2)
			line(6, d-2, 10, d)
		}
		if action == mobileui.ActionSwamp {
			line(0, -4, 0, -10)
		}
	case mobileui.ActionVolcano:
		line(-10, 8, -3, -3)
		line(-3, -3, 3, -3)
		line(3, -3, 10, 8)
		line(-10, 8, 10, 8)
		line(0, -6, 0, -11)
		line(-4, -7, -7, -10)
		line(4, -7, 7, -10)
	case mobileui.ActionArmageddon, mobileui.ActionFight:
		line(-8, -8, 8, 8)
		line(8, -8, -8, 8)
		line(4, 8, 8, 4)
		line(-4, 8, -8, 4)
		line(-8, -8, -3, -7)
		line(8, -8, 3, -7)
	case mobileui.ActionSettle, mobileui.ActionTitle:
		line(-10, -1, 0, -9)
		line(0, -9, 10, -1)
		line(-7, -3, -7, 9)
		line(-7, 9, 7, 9)
		line(7, 9, 7, -3)
		line(0, 9, 0, 2)
	case mobileui.ActionSave, mobileui.ActionLoad:
		line(-8, -2, -8, 8)
		line(-8, 8, 8, 8)
		line(8, 8, 8, -2)
		if action == mobileui.ActionSave {
			line(0, -10, 0, 3)
			line(-4, -1, 0, 3)
			line(4, -1, 0, 3)
		} else {
			line(0, 3, 0, -10)
			line(-4, -6, 0, -10)
			line(4, -6, 0, -10)
		}
	case mobileui.ActionResume:
		line(-5, -8, 8, 0)
		line(8, 0, -5, 8)
		line(-5, 8, -5, -8)
	case mobileui.ActionSettings:
		circle(0, 0, 6)
		circle(0, 0, 2)
		line(0, -10, 0, -6)
		line(0, 6, 0, 10)
		line(-10, 0, -6, 0)
		line(6, 0, 10, 0)
	case mobileui.ActionHelp:
		circle(0, 0, 10)
		line(-3, -4, -1, -6)
		line(-1, -6, 3, -4)
		line(3, -4, 0, 0)
		line(0, 0, 0, 2)
		vector.DrawFilledCircle(screen, x, y+6, 1, ink, true)
	case mobileui.ActionPanUp:
		line(-6, 3, 0, -4)
		line(0, -4, 6, 3)
	case mobileui.ActionPanDown:
		line(-6, -3, 0, 4)
		line(0, 4, 6, -3)
	case mobileui.ActionPanLeft:
		line(3, -6, -4, 0)
		line(-4, 0, 3, 6)
	case mobileui.ActionPanRight:
		line(-3, -6, 4, 0)
		line(4, 0, -3, 6)
	}
}

func mobileActionLabel(action mobileui.Action) string {
	switch action {
	case mobileui.ActionRaise:
		return "RAISE TERRAIN"
	case mobileui.ActionLower:
		return "LOWER TERRAIN"
	case mobileui.ActionMagnet:
		return "PLACE FLAG"
	case mobileui.ActionInspect:
		return "INSPECT"
	case mobileui.ActionQuake:
		return "EARTHQUAKE"
	case mobileui.ActionSwamp:
		return "SWAMP"
	case mobileui.ActionKnight:
		return "KNIGHT"
	case mobileui.ActionVolcano:
		return "VOLCANO"
	case mobileui.ActionFlood:
		return "FLOOD"
	case mobileui.ActionArmageddon:
		return "ARMAGEDDON"
	case mobileui.ActionSettle:
		return "SETTLE"
	case mobileui.ActionJoin:
		return "JOIN"
	case mobileui.ActionFight:
		return "FIGHT"
	case mobileui.ActionFollow:
		return "FOLLOW FLAG"
	case mobileui.ActionResume:
		return "RESUME"
	case mobileui.ActionSave:
		return "SAVE"
	case mobileui.ActionLoad:
		return "LOAD"
	case mobileui.ActionSettings:
		return "SETTINGS"
	case mobileui.ActionHelp:
		return "HELP"
	case mobileui.ActionTitle:
		return "TITLE MENU"
	case mobileui.ActionBack:
		return "BACK"
	case mobileui.ActionConfirm:
		return "CONFIRM"
	case mobileui.ActionCancel:
		return "CANCEL"
	default:
		return ""
	}
}

func mobileAmount(value int) string {
	if value >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(value)/1000000)
	}
	if value >= 10000 {
		return fmt.Sprintf("%.1fK", float64(value)/1000)
	}
	return fmt.Sprint(value)
}

func mobileTruncate(text string, width int) string {
	if len(text) <= width {
		return text
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	return text[:width-3] + "..."
}

func mobileWrap(text string, width, lines int) []string {
	var result []string
	current := ""
	words := strings.Fields(text)
	for i, word := range words {
		if current != "" && len(current)+1+len(word) > width {
			result = append(result, current)
			current = ""
			if len(result) == lines-1 {
				return append(result, mobileTruncate(strings.Join(words[i:], " "), width))
			}
		}
		if current != "" {
			current += " "
		}
		current += word
	}
	if current != "" {
		result = append(result, mobileTruncate(current, width))
	}
	return result
}
