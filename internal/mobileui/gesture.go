// Package mobileui contains the platform-independent mobile interface rules.
package mobileui

// Point is a position or displacement in logical screen pixels.
type Point struct {
	X, Y float64
}

// Contact describes a pointer currently touching the screen. Started should be
// set on its first sample if the input backend can reuse an ID between samples.
type Contact struct {
	ID      int
	X, Y    float64
	Started bool
}

// Region determines which interactions can start at a screen position.
type Region int

const (
	RegionNone Region = iota
	RegionTerrain
	RegionUI
)

// Tap is committed only after the last finger leaves the screen. Start allows
// the caller to check that a UI button is the same at press and release.
type Tap struct {
	Point
	Start  Point
	Region Region
}

// GestureFrame contains events for one input sample, independently of the
// simulation rate. Callers must retain Taps until their next simulation step.
// Pan is a screen-space displacement in the direction of the fingers.
type GestureFrame struct {
	Taps       []Tap
	Pan        Point
	Panning    bool
	Preview    Point
	HasPreview bool
	// UIHeld is a still-valid single-finger UI press. A frontend may repeat
	// camera-pad motion while held, after checking that both points hit the
	// same control. It must not use this to commit terrain or power commands.
	UIHeld  bool
	UIStart Point
	UIPoint Point
}

type gestureContact struct {
	id     int
	start  Point
	point  Point
	region Region
}

// Gesture recognizes a single-finger tap or a two-finger terrain pan. Once a
// second finger arrives, no tap can be emitted until every finger has lifted.
// Contacts retain their initial region, so a pan can cross controls without
// activating them. Mixing UI and terrain fingers, or adding a third finger,
// cancels the whole gesture until all fingers lift.
//
// The zero value is ready to use. Update should run at the input polling rate,
// not at the slower world simulation rate.
type Gesture struct {
	// TapSlop is the maximum displacement allowed for a tap. Nonpositive values
	// select the default of 10 logical pixels. Exceeding it cancels a tap even
	// if the finger subsequently returns to its starting point.
	TapSlop float64

	contacts [2]gestureContact
	count    int
	active   bool
	canceled bool
	multiple bool
	panning  bool
	tapMoved bool
}

// Cancel discards the current gesture and captures input until an Update with
// no contacts. Use it after focus loss, a screen change, or a layout change so
// a finger held across the transition cannot activate the new screen.
func (g *Gesture) Cancel() {
	g.canceled = true
	g.panning = false
}

// Update consumes one complete snapshot of active contacts. Contact order is
// irrelevant. A nil classifier makes all positions noninteractive.
func (g *Gesture) Update(contacts []Contact, classify func(Point) Region) GestureFrame {
	var frame GestureFrame
	if len(contacts) == 0 {
		if g.active && !g.canceled && !g.multiple && !g.tapMoved && g.count == 1 {
			contact := g.contacts[0]
			if contact.region != RegionNone && regionAt(classify, contact.point) == contact.region {
				frame.Taps = []Tap{{Point: contact.point, Start: contact.start, Region: contact.region}}
			}
		}
		g.reset()
		return frame
	}
	if g.canceled {
		return frame
	}
	if len(contacts) > len(g.contacts) || duplicateContactIDs(contacts) {
		g.Cancel()
		return frame
	}

	previous := g.contacts
	previousCount := g.count
	wasActive := g.active
	wasPanning := g.panning
	g.active = true
	g.count = len(contacts)
	unchangedContacts := wasActive && previousCount == len(contacts)
	for i, contact := range contacts {
		point := Point{X: contact.X, Y: contact.Y}
		old, found := findGestureContact(previous[:previousCount], contact.ID)
		if found && !contact.Started {
			old.point = point
			g.contacts[i] = old
		} else {
			g.contacts[i] = gestureContact{
				id: contact.ID, start: point, point: point,
				region: regionAt(classify, point),
			}
			unchangedContacts = false
		}
	}

	// A release and replacement without an empty sample are ambiguous: never
	// reinterpret the replacement as a release of the old single-finger tap.
	if wasActive && previousCount == 1 && len(contacts) == 1 && !unchangedContacts && !g.multiple {
		g.Cancel()
		return frame
	}

	if len(contacts) == 2 {
		g.multiple = true
		if g.contacts[0].region != RegionTerrain || g.contacts[1].region != RegionTerrain {
			g.Cancel()
			return frame
		}
		g.panning = true
		frame.Panning = true
		// Reset the centroid whenever the finger set changes. This avoids a
		// jump when the second finger lands, lifts, or changes its reused ID.
		if wasPanning && unchangedContacts {
			frame.Pan = Point{
				X: (g.contacts[0].point.X + g.contacts[1].point.X - previous[0].point.X - previous[1].point.X) / 2,
				Y: (g.contacts[0].point.Y + g.contacts[1].point.Y - previous[0].point.Y - previous[1].point.Y) / 2,
			}
		}
		return frame
	}
	if g.multiple {
		frame.Panning = g.panning
		return frame
	}

	contact := g.contacts[0]
	dx, dy := contact.point.X-contact.start.X, contact.point.Y-contact.start.Y
	slop := g.TapSlop
	if slop <= 0 {
		slop = 10
	}
	if dx*dx+dy*dy > slop*slop || regionAt(classify, contact.point) != contact.region {
		g.tapMoved = true
	}
	if contact.region == RegionTerrain && !g.tapMoved {
		frame.Preview = contact.point
		frame.HasPreview = true
	}
	if contact.region == RegionUI && !g.tapMoved {
		frame.UIHeld = true
		frame.UIStart = contact.start
		frame.UIPoint = contact.point
	}
	return frame
}

func (g *Gesture) reset() {
	*g = Gesture{TapSlop: g.TapSlop}
}

func regionAt(classify func(Point) Region, point Point) Region {
	if classify == nil {
		return RegionNone
	}
	return classify(point)
}

func duplicateContactIDs(contacts []Contact) bool {
	return len(contacts) == 2 && contacts[0].ID == contacts[1].ID
}

func findGestureContact(contacts []gestureContact, id int) (gestureContact, bool) {
	for _, contact := range contacts {
		if contact.id == id {
			return contact, true
		}
	}
	return gestureContact{}, false
}
