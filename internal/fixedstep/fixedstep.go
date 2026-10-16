// Package fixedstep schedules fixed-rate simulation steps from a configurable
// update loop without relying on wall-clock time.
package fixedstep

// Scheduler converts update-loop calls into fixed-rate simulation steps.
// It uses integer arithmetic, so it does not accumulate floating-point drift.
type Scheduler struct {
	stepRate    int
	updateRate  int
	accumulator int
}

// New returns a Scheduler that emits stepRate simulation steps over every
// updateRate calls to Advance. Non-positive rates are clamped to one.
func New(stepRate, updateRate int) *Scheduler {
	return &Scheduler{
		stepRate:   positiveRate(stepRate),
		updateRate: positiveRate(updateRate),
	}
}

// SetUpdateRate changes the number of Advance calls expected per scheduling
// interval. It resets the partial interval so an old rate cannot affect the
// phase of the new one. A non-positive rate is clamped to one.
func (scheduler *Scheduler) SetUpdateRate(updateRate int) {
	scheduler.updateRate = positiveRate(updateRate)
	scheduler.accumulator = 0
}

// Advance records one update-loop call and returns the number of simulation
// steps due. The result can exceed one when stepRate is greater than
// updateRate.
func (scheduler *Scheduler) Advance() int {
	// Split the rate into whole and fractional parts before accumulating. This
	// avoids overflowing when callers use unusually large positive rates.
	steps := scheduler.stepRate / scheduler.updateRate
	remainder := scheduler.stepRate % scheduler.updateRate
	if scheduler.accumulator >= scheduler.updateRate-remainder {
		steps++
		scheduler.accumulator -= scheduler.updateRate - remainder
	} else {
		scheduler.accumulator += remainder
	}
	return steps
}

func positiveRate(rate int) int {
	if rate < 1 {
		return 1
	}
	return rate
}
