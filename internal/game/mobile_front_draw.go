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

// drawMobileFront renders only the touch menu's supplied presentation state.
// Demo frames use their own renderer and never pass through this function.
func (g *Game) drawMobileFront(screen *ebiten.Image, layout mobileui.FrontLayout, state mobileui.FrontState) {
	screen.Fill(mobileHUDBackground)
	if background := g.images["load"]; background != nil {
		op := &ebiten.DrawImageOptions{}
		scale := float64(layout.Height) / float64(background.Bounds().Dy())
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate((float64(layout.Width)-float64(background.Bounds().Dx())*scale)/2, 0)
		op.ColorScale.Scale(0.18, 0.22, 0.25, 1)
		screen.DrawImage(background, op)
	}
	panel := image.Rect(layout.HeaderRect.Min.X-4, 4, layout.HeaderRect.Max.X+4, layout.Height-4)
	drawMobileRoundedRect(screen, panel, 6, color.RGBA{R: 17, G: 26, B: 36, A: 246})
	vector.DrawFilledRect(screen, float32(layout.HeaderRect.Min.X), 4, float32(layout.HeaderRect.Dx()), 2, mobileHUDAccent, false)

	title := state.Title
	if title == "" {
		title = "POPULOUS"
	}
	ebitenutil.DebugPrintAt(screen, title, layout.HeaderRect.Min.X+2, layout.HeaderRect.Min.Y-1)
	subtitle := state.Subtitle
	if state.Status != "" && layout.Page != mobileui.FrontHome {
		subtitle = state.Status
	}
	if subtitle != "" {
		ebitenutil.DebugPrintAt(screen, mobileTruncate(subtitle, max(0, (layout.HeaderRect.Dx()-4)/6)), layout.HeaderRect.Min.X+2, layout.HeaderRect.Min.Y+14)
	}
	if layout.PageCount > 1 {
		page := fmt.Sprintf("%d / %d", layout.PageIndex+1, layout.PageCount)
		ebitenutil.DebugPrintAt(screen, page, layout.HeaderRect.Max.X-len(page)*6-2, layout.HeaderRect.Min.Y-1)
	}

	switch layout.Page {
	case mobileui.FrontConquest, mobileui.FrontMultiplayer:
		g.drawMobileFrontWorld(screen, layout, state)
	case mobileui.FrontOptions:
		for _, row := range layout.Rows {
			if row.Index < 0 || row.Index >= len(state.Items) {
				continue
			}
			item := state.Items[row.Index]
			drawMobileRoundedRect(screen, row.Rect, 3, mobileHUDPanel)
			textRight := row.Rect.Max.X
			for _, b := range layout.Buttons {
				if b.Index == row.Index && b.Rect.Overlaps(row.Rect) {
					textRight = min(textRight, b.Rect.Min.X-4)
				}
			}
			width := max(0, (textRight-row.Rect.Min.X-8)/6)
			ebitenutil.DebugPrintAt(screen, mobileTruncate(item.Label, width), row.Rect.Min.X+7, row.Rect.Min.Y+1)
			value := item.Value
			if item.Detail != "" {
				if value != "" {
					value += "  "
				}
				value += item.Detail
			}
			ebitenutil.DebugPrintAt(screen, mobileTruncate(value, width), row.Rect.Min.X+7, row.Rect.Min.Y+16)
		}
	case mobileui.FrontHelp:
		drawMobileRoundedRect(screen, layout.ContentRect, 4, mobileHUDPanel)
		width := max(1, (layout.ContentRect.Dx()-20)/6)
		y := layout.ContentRect.Min.Y + 5
		for _, paragraph := range state.HelpLines {
			if y+16 > layout.ContentRect.Max.Y {
				break
			}
			for _, line := range mobileWrap(paragraph, width, max(1, (layout.ContentRect.Max.Y-y)/16)) {
				ebitenutil.DebugPrintAt(screen, line, layout.ContentRect.Min.X+10, y)
				y += 16
			}
		}
	}

	for _, button := range layout.Buttons {
		drawMobileFrontButton(screen, layout, button, state)
	}
	if layout.Page == mobileui.FrontHome {
		status := state.Status
		if status == "" {
			status = "CHOOSE A GAME OR CONTINUE YOUR SAVE"
		}
		ebitenutil.DebugPrintAt(screen, mobileTruncate(status, max(0, layout.FooterRect.Dx()/6)), layout.FooterRect.Min.X, layout.FooterRect.Min.Y)
		if state.IdleEnabled {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("AI DEMO IN %ds  -  TOUCH A CHOICE TO PLAY", max(0, state.IdleSeconds)), layout.FooterRect.Min.X, layout.FooterRect.Min.Y+16)
		}
	}
}

func (g *Game) drawMobileFrontWorld(screen *ebiten.Image, layout mobileui.FrontLayout, state mobileui.FrontState) {
	r := image.Rect(layout.ContentRect.Min.X, layout.ContentRect.Min.Y, layout.ContentRect.Max.X, layout.ContentRect.Min.Y+52)
	drawMobileRoundedRect(screen, r, 4, mobileHUDPanel)
	for i, line := range []string{state.WorldTitle, state.WorldCode, state.WorldDetail} {
		if line == "" {
			continue
		}
		line = mobileTruncate(line, max(0, (r.Dx()-16)/6))
		ebitenutil.DebugPrintAt(screen, line, r.Min.X+(r.Dx()-len(line)*6)/2, r.Min.Y+2+i*16)
	}
}

func drawMobileFrontButton(screen *ebiten.Image, layout mobileui.FrontLayout, button mobileui.FrontButton, state mobileui.FrontState) {
	r := button.Rect
	item := mobileui.FrontItem{Enabled: true}
	usesItem := false
	switch button.Action {
	case mobileui.FrontActionTutorial, mobileui.FrontActionConquest, mobileui.FrontActionMultiplayer, mobileui.FrontActionCustom,
		mobileui.FrontActionSetup, mobileui.FrontActionPreferences, mobileui.FrontActionLoad,
		mobileui.FrontActionHelp, mobileui.FrontActionDemo, mobileui.FrontActionSetupItem,
		mobileui.FrontActionDecrease, mobileui.FrontActionIncrease,
		mobileui.FrontActionTogglePlayerPower, mobileui.FrontActionToggleEnemyPower,
		mobileui.FrontActionToggleWide, mobileui.FrontActionTogglePad, mobileui.FrontActionToggleScope:
		usesItem = true
		if button.Index >= 0 && button.Index < len(state.Items) {
			item = state.Items[button.Index]
		}
	case mobileui.FrontActionStart:
		usesItem = true
		item.Enabled = state.CanStart
	case mobileui.FrontActionHostBluetooth:
		usesItem = true
		item.Enabled = state.CanStart && state.CanBluetooth
	case mobileui.FrontActionJoinBluetooth:
		usesItem = true
		item.Enabled = state.CanBluetooth
	}
	fill, ink := mobileHUDButton, mobileHUDInk
	selected := item.Selected
	if button.Action == mobileui.FrontActionTogglePlayerPower {
		selected = item.PlayerPower
	} else if button.Action == mobileui.FrontActionToggleEnemyPower {
		selected = item.EnemyPower
	}
	if selected || button.Action == mobileui.FrontActionStart {
		fill, ink = mobileHUDSelected, mobileHUDAccent
	}
	if usesItem && !item.Enabled {
		ink = mobileHUDMuted
	}
	drawMobileRoundedRect(screen, r, 4, fill)
	if selected {
		vector.DrawFilledRect(screen, float32(r.Min.X+6), float32(r.Max.Y-3), float32(r.Dx()-12), 2, mobileHUDAccent, false)
	}

	label := item.Label
	switch button.Action {
	case mobileui.FrontActionBack:
		label = "BACK"
	case mobileui.FrontActionStart:
		label = "START CONQUEST"
		if layout.Page == mobileui.FrontOptions {
			label = "PLAY"
		}
	case mobileui.FrontActionHostBluetooth:
		label = "HOST / CREATE"
	case mobileui.FrontActionJoinBluetooth:
		label = "JOIN"
	case mobileui.FrontActionPrevWorld:
		label = fmt.Sprintf("- %d", button.Index)
	case mobileui.FrontActionNextWorld:
		label = fmt.Sprintf("+ %d", button.Index)
	case mobileui.FrontActionPrevPage:
		label = "< PREVIOUS"
	case mobileui.FrontActionNextPage:
		label = "NEXT >"
	case mobileui.FrontActionDecrease:
		label = "-"
	case mobileui.FrontActionIncrease:
		label = "+"
	}
	if button.Action == mobileui.FrontActionTogglePlayerPower || button.Action == mobileui.FrontActionToggleEnemyPower {
		name, value := "BLUE", "OFF"
		if button.Action == mobileui.FrontActionToggleEnemyPower {
			name = "RED"
		}
		if selected {
			value = "ON"
		}
		ebitenutil.DebugPrintAt(screen, name, r.Min.X+(r.Dx()-len(name)*6)/2, r.Min.Y)
		ebitenutil.DebugPrintAt(screen, value, r.Min.X+(r.Dx()-len(value)*6)/2, r.Min.Y+15)
	} else if layout.Page == mobileui.FrontHome {
		drawMobileActionIcon(screen, mobileFrontIcon(button.Action), float32(r.Min.X+17), float32(r.Min.Y+r.Dy()/2), ink)
		textX, textY := r.Min.X+34, r.Min.Y+(r.Dy()-16)/2
		if item.Detail != "" && r.Dx() >= 200 {
			textY = r.Min.Y + 2
			ebitenutil.DebugPrintAt(screen, mobileTruncate(item.Detail, max(0, (r.Max.X-textX-5)/6)), textX, textY+16)
		}
		ebitenutil.DebugPrintAt(screen, mobileTruncate(label, max(0, (r.Max.X-textX-5)/6)), textX, textY)
	} else if button.Action == mobileui.FrontActionSetupItem || button.Action == mobileui.FrontActionToggleWide || button.Action == mobileui.FrontActionTogglePad || button.Action == mobileui.FrontActionToggleScope {
		y := r.Min.Y + (r.Dy()-16)/2
		if item.Value != "" || item.Detail != "" {
			y = r.Min.Y + 3
			value := strings.TrimSpace(item.Value)
			if item.Detail != "" {
				if value != "" {
					value += " - "
				}
				value += item.Detail
			}
			ebitenutil.DebugPrintAt(screen, mobileTruncate(value, max(0, (r.Dx()-14)/6)), r.Min.X+7, y+16)
		}
		ebitenutil.DebugPrintAt(screen, mobileTruncate(label, max(0, (r.Dx()-14)/6)), r.Min.X+7, y)
	} else {
		label = mobileTruncate(label, max(0, (r.Dx()-8)/6))
		ebitenutil.DebugPrintAt(screen, label, r.Min.X+(r.Dx()-len(label)*6)/2, r.Min.Y+(r.Dy()-16)/2)
	}
	if usesItem && !item.Enabled {
		drawMobileRoundedRect(screen, r, 4, color.RGBA{R: 17, G: 26, B: 36, A: 125})
	}
}

func mobileFrontIcon(action mobileui.FrontAction) mobileui.Action {
	switch action {
	case mobileui.FrontActionTutorial:
		return mobileui.ActionHelp
	case mobileui.FrontActionConquest:
		return mobileui.ActionLeader
	case mobileui.FrontActionMultiplayer:
		return mobileui.ActionJoin
	case mobileui.FrontActionCustom:
		return mobileui.ActionRaise
	case mobileui.FrontActionSetup:
		return mobileui.ActionMenu
	case mobileui.FrontActionPreferences:
		return mobileui.ActionSettings
	case mobileui.FrontActionLoad:
		return mobileui.ActionLoad
	case mobileui.FrontActionHelp:
		return mobileui.ActionInspect
	case mobileui.FrontActionDemo:
		return mobileui.ActionResume
	default:
		return mobileui.ActionNone
	}
}
