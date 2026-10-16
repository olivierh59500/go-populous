package game

import (
	"context"
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"go-populous/internal/recording"
)

// DemoRecordingConfig exports one complete match at the normal simulation
// speed on the video's timeline. Encoding can run faster than real time.
type DemoRecordingConfig struct {
	Path          string
	Width, Height int
	MaxDuration   time.Duration   // zero means no limit; reaching a limit is an error, never a win
	Context       context.Context // optional cancellation for command-line interrupts
	Progress      func(string)    // optional status at start, each simulated minute, and victory
}

type demoRecording struct {
	encoder  *recording.Recorder
	pixels   []byte
	events   []int
	pops     [2]int
	pending  bool
	done     bool
	err      error
	maxTicks int
	context  context.Context
	progress func(string)
}

// StartDemoRecording captures the application's pixels and synthesizes its
// own PCM soundtrack. No audio device, microphone, or system mixer is opened.
// Call before RunGame, after choosing the demo world/seed.
func (g *Game) StartDemoRecording(config DemoRecordingConfig) error {
	if g.recording != nil || g.audioInitialized || g.multiplayerEnabled() || g.state != StateTitle {
		return fmt.Errorf("start demo recording: requires a fresh offline title screen")
	}
	if config.MaxDuration < 0 {
		return fmt.Errorf("recording duration cannot be negative")
	}
	encoder, err := recording.New(recording.Config{
		Path: config.Path, Width: demoWidth, Height: demoHeight,
		OutputWidth: config.Width, OutputHeight: config.Height,
	}, g.soundBank)
	if err != nil {
		return err
	}
	g.StartDemo()
	g.demo.session.LimitTicks = 0
	maxTicks := int(config.MaxDuration / (time.Second / simulationTPS))
	if config.MaxDuration%(time.Second/simulationTPS) != 0 {
		maxTicks++
	}
	g.recording = &demoRecording{
		encoder:  encoder,
		pixels:   make([]byte, demoWidth*demoHeight*4),
		pending:  true,
		pops:     g.demo.session.World.PlayerPopulations(),
		maxTicks: maxTicks,
		context:  config.Context,
		progress: config.Progress,
	}
	g.reportDemoRecordingProgress()
	g.audioInitialized = true
	g.soundBank = nil
	return nil
}

func (g *Game) updateDemoRecording() error {
	r := g.recording
	if r.err != nil {
		return r.err
	}
	if r.done {
		return ebiten.Termination
	}
	if r.context != nil && r.context.Err() != nil {
		return fmt.Errorf("demo recording canceled: %w", r.context.Err())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return fmt.Errorf("demo recording canceled before completion")
	}
	// Ebiten can call Update multiple times before Draw. This gate ensures
	// every simulated step is rendered and encoded exactly once.
	if r.pending {
		return nil
	}
	session := g.demo.session
	if session.ShouldRestart() {
		if session.Winner < 0 {
			return fmt.Errorf("demo ended without a winner: %s", session.Outcome)
		}
		if err := r.encoder.Close(); err != nil {
			r.err = err
			return err
		}
		r.done = true
		return ebiten.Termination
	}
	if r.maxTicks > 0 && session.ElapsedTicks >= r.maxTicks && !session.Finished {
		return fmt.Errorf("no winner after %s of simulation; recording canceled (increase -record-max-duration or use 0)", time.Duration(r.maxTicks)*time.Second/simulationTPS)
	}
	g.tick++
	wasFinished := session.Finished
	session.Tick()
	r.events = append(r.events[:0], session.World.DrainSoundEvents()...)
	r.pops = session.World.PlayerPopulations()
	if session.Finished {
		r.pops = [2]int{} // no heartbeat over the final result
	}
	r.pending = true
	if !wasFinished && (session.Finished || session.ElapsedTicks%(60*simulationTPS) == 0) {
		g.reportDemoRecordingProgress()
	}
	return nil
}

func (g *Game) reportDemoRecordingProgress() {
	if g.recording.progress == nil {
		return
	}
	session := g.demo.session
	level := session.World.Level
	seconds := session.ElapsedTicks / simulationTPS
	g.recording.progress(fmt.Sprintf("World %03d %s, %02d:%02d, population %v %s", level.Number, level.Code, seconds/60, seconds%60, session.World.PlayerPopulations(), session.Outcome))
}

func (g *Game) captureDemoRecordingFrame() {
	r := g.recording
	if !r.pending || r.err != nil || r.done {
		return
	}
	g.demo.frame.ReadPixels(r.pixels)
	player := g.demo.session.AdvancedPlayer
	r.err = r.encoder.WriteFrame(r.pixels, r.events, r.pops[player], r.pops[player^1])
	r.pending = false
}

func (g *Game) closeDemoRecording() error {
	if g.recording == nil || g.recording.done {
		return nil
	}
	g.recording.encoder.Abort()
	if g.recording.err != nil {
		return g.recording.err
	}
	return fmt.Errorf("demo recording interrupted before a complete result")
}
