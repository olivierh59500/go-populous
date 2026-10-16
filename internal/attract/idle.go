// Package attract manages the unattended AI demonstration independently of
// windowing, input devices, saved games, and multiplayer sessions.
package attract

const (
	TicksPerSecond   = 8
	DefaultIdleTicks = 30 * TicksPerSecond
)

// Idle measures inactivity in simulation ticks. A nonpositive limit disables
// the timer. Call Tick only while the menu is eligible for a demonstration.
type Idle struct {
	LimitTicks   int
	ElapsedTicks int
}

func NewIdle(limitTicks int) *Idle {
	return &Idle{LimitTicks: limitTicks}
}

// Tick resets on any activity; otherwise it reports when the limit is reached.
// The elapsed count saturates so a menu left unattended cannot overflow it.
func (idle *Idle) Tick(activity bool) bool {
	if activity || idle.LimitTicks <= 0 {
		idle.Reset()
		return false
	}
	if idle.ElapsedTicks < idle.LimitTicks {
		idle.ElapsedTicks++
	}
	return idle.ElapsedTicks >= idle.LimitTicks
}

func (idle *Idle) Reset() {
	idle.ElapsedTicks = 0
}
