// Package touchui contains the platform-independent geometry for Populous'
// wide-screen touch controls. It deliberately has no Ebitengine dependency so
// its layout and multitouch rules can be tested on headless builders.
package touchui

const (
	SceneWidth  = 320
	SceneHeight = 240
	MaxWidth    = 640
	MinSideBand = 56
)

// Point is a logical touch position sampled during one update.
type Point struct {
	X    int
	Y    int
	Just bool
}

// Contact identifies a currently active pointer. IDs only need to be stable
// from press to release; the game maps Ebitengine touch IDs to int values.
type Contact struct {
	ID int
	X  int
	Y  int
}

// Latch retains newly pressed contacts until the next simulation step. This
// lets a frontend poll input faster than the simulation without losing a short
// tap that is released between two simulation ticks.
type Latch struct {
	pending []Contact
	step    []Point
	matched []bool
}

// Observe records contacts newly pressed during the current input poll. An
// already pending contact follows its latest active position before it is
// consumed.
func (latch *Latch) Observe(active []Contact, justPressedIDs []int) {
	for _, contact := range active {
		if intInSlice(contact.ID, justPressedIDs) {
			continue
		}
		if index := lastContactIndex(latch.pending, contact.ID); index >= 0 {
			latch.pending[index] = contact
		}
	}
	for _, id := range justPressedIDs {
		if contact, ok := contactByID(active, id); ok {
			latch.pending = append(latch.pending, contact)
		}
	}
}

func intInSlice(value int, values []int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// Consume returns the contact state for one simulation step. Active contacts
// remain present on every step, while newly pressed contacts carry Just once.
// A contact released since Observe is emitted once at its last known position.
// The returned slice is owned by the latch and is valid until the next Consume.
func (latch *Latch) Consume(active []Contact) []Point {
	latch.step = latch.step[:0]
	if cap(latch.matched) < len(latch.pending) {
		latch.matched = make([]bool, len(latch.pending))
	} else {
		latch.matched = latch.matched[:len(latch.pending)]
		clear(latch.matched)
	}
	for _, contact := range active {
		pendingIndex := lastUnmatchedContactIndex(latch.pending, latch.matched, contact.ID)
		if pendingIndex >= 0 {
			latch.matched[pendingIndex] = true
		}
		latch.step = append(latch.step, Point{X: contact.X, Y: contact.Y, Just: pendingIndex >= 0})
	}
	for index, pending := range latch.pending {
		if latch.matched[index] {
			continue
		}
		latch.step = append(latch.step, Point{X: pending.X, Y: pending.Y, Just: true})
	}
	latch.pending = latch.pending[:0]
	return latch.step
}

// Pending reports the number of new contacts waiting for a simulation step.
func (latch *Latch) Pending() int {
	return len(latch.pending)
}

func contactByID(contacts []Contact, id int) (Contact, bool) {
	for _, contact := range contacts {
		if contact.ID == id {
			return contact, true
		}
	}
	return Contact{}, false
}

func lastContactIndex(contacts []Contact, id int) int {
	for index := len(contacts) - 1; index >= 0; index-- {
		if contacts[index].ID == id {
			return index
		}
	}
	return -1
}

func lastUnmatchedContactIndex(contacts []Contact, matched []bool, id int) int {
	for index := len(contacts) - 1; index >= 0; index-- {
		if !matched[index] && contacts[index].ID == id {
			return index
		}
	}
	return -1
}

// LogicalWidth keeps the original 320x240 scene intact and adds logical pixels
// horizontally to match wide displays.
func LogicalWidth(outsideWidth, outsideHeight int) int {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		return SceneWidth
	}
	width := (outsideWidth*SceneHeight + outsideHeight - 1) / outsideHeight
	if width < SceneWidth {
		return SceneWidth
	}
	if width > MaxWidth {
		return MaxWidth
	}
	return width
}

// Button is a circular hit target in logical coordinates.
type Button struct {
	X      int
	Y      int
	Radius int
}

// Contains reports whether a logical position falls inside the button.
func (button Button) Contains(x, y int) bool {
	dx := x - button.X
	dy := y - button.Y
	return dx*dx+dy*dy <= button.Radius*button.Radius
}

// Layout describes the fixed scene and the controls in its side bands.
type Layout struct {
	Width      int
	SceneX     int
	Enabled    bool
	DPad       Button
	Raise      Button
	Lower      Button
	Menu       Button
	SceneWidth int
}

// NewLayout builds the shared drawing and hit-test geometry.
func NewLayout(width int) Layout {
	if width < SceneWidth {
		width = SceneWidth
	}
	sceneX := (width - SceneWidth) / 2
	leftBand := sceneX
	rightBand := width - sceneX - SceneWidth
	band := minInt(leftBand, rightBand)
	layout := Layout{
		Width:      width,
		SceneX:     sceneX,
		SceneWidth: SceneWidth,
		Enabled:    band >= MinSideBand,
	}
	if !layout.Enabled {
		return layout
	}

	leftX := leftBand / 2
	rightX := sceneX + SceneWidth + rightBand/2
	dPadRadius := clampInt(band/2-7, 22, 43)
	actionRadius := clampInt(band/2-10, 21, 34)
	menuRadius := clampInt(band/2-16, 18, 25)
	layout.DPad = Button{X: leftX, Y: 174, Radius: dPadRadius}
	layout.Raise = Button{X: rightX, Y: 124, Radius: actionRadius}
	layout.Lower = Button{X: rightX, Y: 194, Radius: actionRadius}
	layout.Menu = Button{X: rightX, Y: 34, Radius: menuRadius}
	return layout
}

// ScenePosition converts a wide-screen logical position into original scene
// coordinates.
func (layout Layout) ScenePosition(x, y int) (int, int, bool) {
	localX := x - layout.SceneX
	if localX < 0 || localX >= layout.SceneWidth || y < 0 || y >= SceneHeight {
		return 0, 0, false
	}
	return localX, y, true
}

// DPadDirection returns a cardinal or diagonal direction. The center is a dead
// zone so resting a thumb cannot move the camera accidentally.
func (layout Layout) DPadDirection(x, y int) (int, int, bool) {
	if !layout.Enabled || !layout.DPad.Contains(x, y) {
		return 0, 0, false
	}
	dx := x - layout.DPad.X
	dy := y - layout.DPad.Y
	deadZone := maxInt(7, layout.DPad.Radius/4)
	if dx*dx+dy*dy < deadZone*deadZone {
		return 0, 0, false
	}

	xDirection := 0
	yDirection := 0
	if absInt(dx)*2 >= absInt(dy) {
		if dx < 0 {
			xDirection = -1
		} else {
			xDirection = 1
		}
	}
	if absInt(dy)*2 >= absInt(dx) {
		if dy < 0 {
			yDirection = -1
		} else {
			yDirection = 1
		}
	}
	return xDirection, yDirection, xDirection != 0 || yDirection != 0
}

// Controls is the combined state of every current contact. Boolean actions are
// ORed, which makes a map contact plus a held LOWER modifier reliable.
type Controls struct {
	DX        int
	DY        int
	DPadJust  bool
	Raise     bool
	RaiseJust bool
	Lower     bool
	LowerJust bool
	Menu      bool
	MenuJust  bool
	AnyJust   bool
	SceneJust bool
}

// Evaluate combines all current contacts without depending on their order.
func Evaluate(layout Layout, points []Point) Controls {
	var controls Controls
	for _, point := range points {
		if point.Just {
			controls.AnyJust = true
		}
		if _, _, ok := layout.ScenePosition(point.X, point.Y); ok {
			controls.SceneJust = controls.SceneJust || point.Just
			continue
		}
		if !layout.Enabled {
			continue
		}
		if dx, dy, ok := layout.DPadDirection(point.X, point.Y); ok {
			controls.DX += dx
			controls.DY += dy
			controls.DPadJust = controls.DPadJust || point.Just
			continue
		}
		if layout.Raise.Contains(point.X, point.Y) {
			controls.Raise = true
			controls.RaiseJust = controls.RaiseJust || point.Just
			continue
		}
		if layout.Lower.Contains(point.X, point.Y) {
			controls.Lower = true
			controls.LowerJust = controls.LowerJust || point.Just
			continue
		}
		if layout.Menu.Contains(point.X, point.Y) {
			controls.Menu = true
			controls.MenuJust = controls.MenuJust || point.Just
		}
	}
	controls.DX = clampInt(controls.DX, -1, 1)
	controls.DY = clampInt(controls.DY, -1, 1)
	return controls
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
