package game

import (
	"fmt"

	"go-populous/internal/recording"
)

// SetAudioTracePath enables opt-in source audio capture before the first Update.
// The trace records only this game's sound events and never replaces a file.
// It has no effect on gameplay, the sound output, or the presentation.
func (g *Game) SetAudioTracePath(path string) error {
	if g == nil || g.audioInitialized || g.audioTrace != nil {
		return fmt.Errorf("audio capture must be configured once before the first update")
	}
	if path == "" {
		return nil
	}
	trace, err := recording.NewAudioTrace(path)
	if err != nil {
		return err
	}
	g.audioTrace = trace
	return nil
}
