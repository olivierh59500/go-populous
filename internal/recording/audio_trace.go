package recording

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// AudioTraceEvent describes an actual source sound played by the application.
// UnixNano is anchored to the opening wall clock and advances monotonically.
// Voice distinguishes overlapping instances of the same sound.
type AudioTraceEvent struct {
	Kind       string  `json:"kind"`
	UnixNano   int64   `json:"unix_ns"`
	Version    int     `json:"version,omitempty"`
	SampleRate int     `json:"sample_rate,omitempty"`
	Voice      uint64  `json:"voice,omitempty"`
	Sound      int     `json:"sound,omitempty"`
	Volume     float64 `json:"volume,omitempty"`
}

// AudioTrace records only application sound events, never a microphone or an
// operating-system audio stream. Each event is written directly to the file so
// a running Android application's trace can be copied with adb run-as.
type AudioTrace struct {
	mu     sync.Mutex
	file   *os.File
	writer *json.Encoder
	origin time.Time
	voice  uint64
	err    error
}

// NewAudioTrace creates a new private JSONL trace and never replaces a file.
func NewAudioTrace(path string) (*AudioTrace, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("create audio trace: %w", err)
	}
	t := &AudioTrace{file: file, writer: json.NewEncoder(file), origin: time.Now()}
	t.writeLocked(AudioTraceEvent{Kind: "start", Version: 1, SampleRate: SampleRate})
	if t.err != nil {
		_ = file.Close()
		return nil, t.err
	}
	return t, nil
}

// Play records a sound at its source and returns its unique playback instance.
// A nil or failed trace is a no-op so capture cannot interrupt normal audio.
func (t *AudioTrace) Play(sound int, volume float64) uint64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file == nil || t.err != nil {
		return 0
	}
	t.voice++
	t.writeLocked(AudioTraceEvent{Kind: "play", Voice: t.voice, Sound: sound, Volume: volume})
	return t.voice
}

// Stop records an explicit close, including the live player's voice limit.
// Natural sample endings need no event: the original PCM defines their length.
func (t *AudioTrace) Stop(voice uint64) {
	if t == nil || voice == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writeLocked(AudioTraceEvent{Kind: "stop", Voice: voice})
}

// Close flushes the trace and is safe to call repeatedly or on a nil receiver.
func (t *AudioTrace) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.file == nil {
		return t.err
	}
	t.writeLocked(AudioTraceEvent{Kind: "end"})
	t.err = errors.Join(t.err, t.file.Sync(), t.file.Close())
	t.file = nil
	return t.err
}

func (t *AudioTrace) writeLocked(event AudioTraceEvent) {
	if t.file == nil || t.err != nil {
		return
	}
	event.UnixNano = t.origin.Add(time.Since(t.origin)).UnixNano()
	if err := t.writer.Encode(event); err != nil {
		t.err = fmt.Errorf("write audio trace: %w", err)
	}
}
