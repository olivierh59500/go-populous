package mobileui

import "image"

// FrontPage identifies a touch-friendly menu. Demo rendering has its own path
// and does not use these menus or their geometry.
type FrontPage uint8

const (
	FrontHome FrontPage = iota
	FrontConquest
	FrontMultiplayer
	FrontSetup
	FrontOptions
	FrontPreferences
	FrontHelp
)

type FrontAction uint8

const (
	FrontActionNone FrontAction = iota
	FrontActionTutorial
	FrontActionConquest
	FrontActionMultiplayer
	FrontActionCustom
	FrontActionSetup
	FrontActionPreferences
	FrontActionLoad
	FrontActionHelp
	FrontActionDemo
	FrontActionBack
	FrontActionStart
	FrontActionPrevWorld
	FrontActionNextWorld
	FrontActionPrevPage
	FrontActionNextPage
	FrontActionSetupItem
	FrontActionDecrease
	FrontActionIncrease
	FrontActionTogglePlayerPower
	FrontActionToggleEnemyPower
	FrontActionToggleWide
	FrontActionTogglePad
	FrontActionToggleScope
	FrontActionHostBluetooth
	FrontActionJoinBluetooth
)

type FrontButton struct {
	Rect   image.Rectangle
	Action FrontAction
	// Index identifies an absolute item, or a world-step magnitude (1 or 10).
	Index int
}

type FrontRow struct {
	Rect  image.Rectangle
	Index int
}

// FrontItem and FrontState contain presentation data only. The game owns the
// settings, permissions and menu transitions represented by these values.
type FrontItem struct {
	Label, Value, Detail    string
	Enabled, Selected       bool
	PlayerPower, EnemyPower bool
}

type FrontState struct {
	Title, Subtitle, Status             string
	WorldTitle, WorldDetail, WorldCode  string
	Items                               []FrontItem
	HelpLines                           []string
	IdleSeconds                         int
	IdleEnabled, CanStart, CanBluetooth bool
}

type FrontLayout struct {
	Width, Height                       int
	Page                                FrontPage
	PageIndex, PageCount                int
	FirstItem, LastItem                 int // LastItem is exclusive.
	HeaderRect, ContentRect, FooterRect image.Rectangle
	Buttons                             []FrontButton
	Rows                                []FrontRow
}

// NewFrontLayout shares hit geometry with the renderer. At the smallest
// supported size, every button is at least 32 logical pixels in both axes.
// Wider screens grow a centered panel; additional height grows its content.
// firstPowerRow optionally supplies the options model's first power row; the
// default is 9. Power rows expose separate player and opponent toggles.
func NewFrontLayout(width, height int, page FrontPage, pageIndex, itemCount int, firstPowerRow ...int) FrontLayout {
	width, height = max(320, width), max(240, height)
	powerRow := 9
	if len(firstPowerRow) > 0 {
		powerRow = max(0, firstPowerRow[0])
	}
	panelWidth := min(width-24, 640)
	x := (width - panelWidth) / 2
	l := FrontLayout{
		Width: width, Height: height, Page: page, PageCount: 1,
		HeaderRect:  image.Rect(x, 8, x+panelWidth, 34),
		ContentRect: image.Rect(x, 42, x+panelWidth, height-44),
		FooterRect:  image.Rect(x, height-38, x+panelWidth, height-4),
	}
	itemCount = max(0, itemCount)
	pageSize := 0
	switch page {
	case FrontSetup:
		pageSize = 6
	case FrontOptions:
		pageSize = 4
	case FrontHelp:
		pageSize = 1
	}
	if pageSize > 0 {
		if itemCount > 0 {
			l.PageCount = 1 + (itemCount-1)/pageSize
		}
		l.PageIndex = min(max(0, pageIndex), l.PageCount-1)
		l.FirstItem = l.PageIndex * pageSize
		l.LastItem = min(itemCount, l.FirstItem+pageSize)
	}
	addButton := func(rect image.Rectangle, action FrontAction, index int) {
		l.Buttons = append(l.Buttons, FrontButton{Rect: rect, Action: action, Index: index})
	}
	gridRect := func(col, row, cols, rows int) image.Rectangle {
		const gap = 6
		w := (l.ContentRect.Dx() - (cols-1)*gap) / cols
		h := (l.ContentRect.Dy() - (rows-1)*gap) / rows
		left, top := x+col*(w+gap), l.ContentRect.Min.Y+row*(h+gap)
		return image.Rect(left, top, left+w, top+h)
	}
	switch page {
	case FrontHome:
		for i, action := range [...]FrontAction{
			FrontActionTutorial, FrontActionConquest, FrontActionMultiplayer,
			FrontActionCustom, FrontActionSetup, FrontActionPreferences,
			FrontActionLoad, FrontActionHelp, FrontActionDemo,
		} {
			addButton(gridRect(i%3, i/3, 3, 3), action, i)
		}
		return l
	case FrontConquest, FrontMultiplayer:
		// Leave the top of the content area free for the world name, code
		// and difficulty before the four navigation targets.
		l.Rows = append(l.Rows, FrontRow{Rect: image.Rect(x, 42, x+panelWidth, 94)})
		buttonWidth := (panelWidth - 18) / 4
		for i, choice := range [...]struct {
			action FrontAction
			delta  int
		}{
			{FrontActionPrevWorld, 10}, {FrontActionPrevWorld, 1},
			{FrontActionNextWorld, 1}, {FrontActionNextWorld, 10},
		} {
			left := x + i*(buttonWidth+6)
			addButton(image.Rect(left, 100, left+buttonWidth, 140), choice.action, choice.delta)
		}
		playTop := l.ContentRect.Max.Y - 42
		if page == FrontMultiplayer {
			buttonWidth := (panelWidth - 6) / 2
			addButton(image.Rect(x, playTop, x+buttonWidth, l.ContentRect.Max.Y), FrontActionHostBluetooth, 0)
			addButton(image.Rect(x+buttonWidth+6, playTop, x+panelWidth, l.ContentRect.Max.Y), FrontActionJoinBluetooth, 0)
		} else {
			addButton(image.Rect(x, playTop, x+panelWidth, l.ContentRect.Max.Y), FrontActionStart, 0)
		}
	case FrontSetup:
		for index := l.FirstItem; index < l.LastItem; index++ {
			i := index - l.FirstItem
			r := gridRect(i%2, i/2, 2, 3)
			l.Rows = append(l.Rows, FrontRow{Rect: r, Index: index})
			addButton(r, FrontActionSetupItem, index)
		}
	case FrontOptions:
		controlWidth := min(64, panelWidth/7)
		for index := l.FirstItem; index < l.LastItem; index++ {
			r := gridRect(0, index-l.FirstItem, 1, 4)
			l.Rows = append(l.Rows, FrontRow{Rect: r, Index: index})
			leftAction, rightAction := FrontActionDecrease, FrontActionIncrease
			if index >= powerRow {
				leftAction, rightAction = FrontActionTogglePlayerPower, FrontActionToggleEnemyPower
			}
			addButton(image.Rect(r.Max.X-2*controlWidth-6, r.Min.Y, r.Max.X-controlWidth-6, r.Max.Y), leftAction, index)
			addButton(image.Rect(r.Max.X-controlWidth, r.Min.Y, r.Max.X, r.Max.Y), rightAction, index)
		}
	case FrontPreferences:
		for i, action := range [...]FrontAction{FrontActionToggleWide, FrontActionTogglePad, FrontActionToggleScope} {
			r := gridRect(0, i, 1, 3)
			l.Rows = append(l.Rows, FrontRow{Rect: r, Index: i})
			addButton(r, action, i)
		}
	case FrontHelp:
		l.Rows = append(l.Rows, FrontRow{Rect: l.ContentRect, Index: l.PageIndex})
	}
	footerColumns := 3
	if page == FrontOptions {
		footerColumns = 4
	}
	footerWidth := (panelWidth - 6*(footerColumns-1)) / footerColumns
	footerButton := func(col int, action FrontAction) {
		left := x + col*(footerWidth+6)
		addButton(image.Rect(left, l.FooterRect.Min.Y, left+footerWidth, l.FooterRect.Max.Y), action, 0)
	}
	footerButton(0, FrontActionBack)
	if l.PageIndex > 0 {
		footerButton(1, FrontActionPrevPage)
	}
	if l.PageIndex+1 < l.PageCount {
		footerButton(2, FrontActionNextPage)
	}
	if page == FrontOptions {
		footerButton(3, FrontActionStart)
	}
	return l
}

// Hit returns only a visible button. Labels, gutters and the surrounding
// background never activate a neighboring item.
func (l FrontLayout) Hit(x, y int) (FrontAction, int, bool) {
	p := image.Pt(x, y)
	for _, button := range l.Buttons {
		if p.In(button.Rect) {
			return button.Action, button.Index, true
		}
	}
	return FrontActionNone, 0, false
}
