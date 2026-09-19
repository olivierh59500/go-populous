// Package mobileui describes the mobile game's controls without depending on
// the renderer or the simulation. Drawing and touch routing share this layout.
package mobileui

import "image"

type Action uint8

const (
	ActionNone Action = iota
	ActionRaise
	ActionLower
	ActionMagnet
	ActionInspect
	ActionPowers
	ActionPeople
	ActionLeader
	ActionMenu
	ActionQuake
	ActionSwamp
	ActionKnight
	ActionVolcano
	ActionFlood
	ActionArmageddon
	ActionSettle
	ActionJoin
	ActionFight
	ActionFollow
	ActionResume
	ActionSave
	ActionLoad
	ActionSettings
	ActionHelp
	ActionTitle
	ActionToggleWide
	ActionTogglePad
	ActionToggleBuildScope
	ActionBack
	ActionConfirm
	ActionCancel
	ActionPanUp
	ActionPanDown
	ActionPanLeft
	ActionPanRight
	ActionCount
)

type Overlay uint8

const (
	OverlayNone Overlay = iota
	OverlayPowers
	OverlayPeople
	OverlayMenu
	OverlaySettings
	OverlayConfirm
)

type Button struct {
	Rect   image.Rectangle
	Action Action
}

type HUDLayout struct {
	Width, Height int
	Overlay       Overlay
	MapRect       image.Rectangle
	MiniMapRect   image.Rectangle
	ToolbarRect   image.Rectangle
	PanelRect     image.Rectangle
	DPadRect      image.Rectangle
	Buttons       []Button
}

// PowerState uses the simulation's current prices and permissions. A disabled
// power is still inspectable; the input handler decides whether it can be used.
type PowerState struct {
	Cost    int
	Enabled bool
}

type HUDState struct {
	Selected          Action
	Mana, Population  int
	Powers            [ActionCount]PowerState
	Wide              bool
	ShowPad           bool
	VisibleBuildScope bool
	Online            bool
	Status            string
	Confirmation      string
}

// NewHUDLayout keeps the controls large enough at the original 320-pixel width
// and uses the additional landscape width for terrain rather than larger bars.
func NewHUDLayout(width, height int, overlay Overlay, showPad bool) HUDLayout {
	width = max(320, width)
	height = max(240, height)
	l := HUDLayout{
		Width: width, Height: height, Overlay: overlay,
		MapRect:     image.Rect(0, 24, width, height-44),
		MiniMapRect: image.Rect(4, 28, 68, 60),
		ToolbarRect: image.Rect(0, height-44, width, height),
	}
	if overlay == OverlayNone {
		for i, action := range [...]Action{ActionRaise, ActionLower, ActionMagnet, ActionInspect, ActionPowers, ActionPeople, ActionLeader, ActionMenu} {
			l.Buttons = append(l.Buttons, Button{
				Rect: image.Rect(i*width/8+1, height-43, (i+1)*width/8-1, height-1), Action: action,
			})
		}
		if showPad {
			x, y := 6, l.MapRect.Max.Y-80
			l.DPadRect = image.Rect(x, y, x+78, y+78)
			for _, b := range []Button{
				{image.Rect(x+26, y, x+52, y+26), ActionPanUp},
				{image.Rect(x+26, y+52, x+52, y+78), ActionPanDown},
				{image.Rect(x, y+26, x+26, y+52), ActionPanLeft},
				{image.Rect(x+52, y+26, x+78, y+52), ActionPanRight},
			} {
				l.Buttons = append(l.Buttons, b)
			}
		}
		return l
	}

	panelWidth, panelHeight := min(width-16, 380), 164
	if overlay == OverlayPeople || overlay == OverlayConfirm {
		panelHeight = 132
	}
	x, y := (width-panelWidth)/2, 24+(height-68-panelHeight)/2
	l.PanelRect = image.Rect(x, y, x+panelWidth, y+panelHeight)
	var actions []Action
	switch overlay {
	case OverlayPowers:
		actions = []Action{ActionQuake, ActionSwamp, ActionKnight, ActionVolcano, ActionFlood, ActionArmageddon}
	case OverlayPeople:
		actions = []Action{ActionSettle, ActionJoin, ActionFight, ActionFollow}
	case OverlayMenu:
		actions = []Action{ActionResume, ActionSave, ActionLoad, ActionSettings, ActionHelp, ActionTitle}
	case OverlaySettings:
		// Full-width settings leave room for both the current setting and its
		// label on the narrowest display.
		for i, action := range [...]Action{ActionToggleWide, ActionTogglePad, ActionToggleBuildScope} {
			l.Buttons = append(l.Buttons, Button{image.Rect(x+8, y+23+i*35, x+panelWidth-8, y+55+i*35), action})
		}
	case OverlayConfirm:
		l.Buttons = append(l.Buttons,
			Button{image.Rect(x+8, y+panelHeight-42, x+panelWidth/2-3, y+panelHeight-8), ActionCancel},
			Button{image.Rect(x+panelWidth/2+3, y+panelHeight-42, x+panelWidth-8, y+panelHeight-8), ActionConfirm},
		)
		return l
	}
	for i, action := range actions {
		col, row := i%2, i/2
		left := x + 8 + col*(panelWidth-10)/2
		l.Buttons = append(l.Buttons, Button{image.Rect(left, y+23+row*35, left+(panelWidth-22)/2, y+55+row*35), action})
	}
	l.Buttons = append(l.Buttons, Button{image.Rect(x+8, y+panelHeight-29, x+panelWidth-8, y+panelHeight-5), ActionBack})
	return l
}

func (l HUDLayout) Hit(x, y int) Action {
	p := image.Pt(x, y)
	for _, button := range l.Buttons {
		if p.In(button.Rect) {
			return button.Action
		}
	}
	return ActionNone
}

// IsTerrainPoint excludes all overlays, including empty spaces between their
// buttons. Touching UI padding must never terraform the world behind it.
func (l HUDLayout) IsTerrainPoint(x, y int) bool {
	p := image.Pt(x, y)
	return l.Overlay == OverlayNone && p.In(l.MapRect) && !p.In(l.MiniMapRect) && !p.In(l.DPadRect)
}

func (l HUDLayout) IsMiniMapPoint(x, y int) bool {
	return l.Overlay == OverlayNone && image.Pt(x, y).In(l.MiniMapRect)
}
