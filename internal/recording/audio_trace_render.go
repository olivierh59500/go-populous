package recording

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"go-populous/internal/populous"
)

// AudioTraceRenderConfig selects the recorded wall-clock interval to render.
// A zero StartUnixNano starts at the trace header. Duration must be positive.
type AudioTraceRenderConfig struct {
	StartUnixNano int64
	Duration      time.Duration
}

// RenderAudioTraceWAV reconstructs the actual source sounds into stereo PCM WAV.
// It retains sounds already playing when the selected video interval starts.
// Input is streamed and output uses fixed-size chunks, so memory does not grow
// with the recording duration. The original sound bank is never modified.
func RenderAudioTraceWAV(output io.Writer, input io.Reader, bank *populous.SoundBank, config AudioTraceRenderConfig) error {
	frames := durationSamples(int64(config.Duration))
	if config.Duration <= 0 || frames <= 0 || frames > (math.MaxUint32-36)/4 {
		return errors.New("audio trace duration must fit a nonempty PCM WAV (less than 7 hours)")
	}
	r := &audioTraceReader{scanner: bufio.NewScanner(input)}
	r.scanner.Buffer(make([]byte, 4096), 64*1024)
	header, err := r.next()
	if err != nil {
		return fmt.Errorf("audio trace header: %w", err)
	}
	if header.Kind != "start" || header.Version != 1 || header.SampleRate != SampleRate {
		return errors.New("unsupported audio trace header")
	}
	if config.StartUnixNano == 0 {
		config.StartUnixNano = header.UnixNano
	}
	if config.StartUnixNano < 0 {
		return errors.New("audio trace start timestamp must be positive")
	}
	if err := writeWAVHeader(output, frames); err != nil {
		return fmt.Errorf("write audio trace WAV header: %w", err)
	}
	pcm := make(map[int][]byte)
	renderer := traceRenderer{pcm: func(sound int) []byte {
		if cached, ok := pcm[sound]; ok {
			return cached
		}
		var rendered []byte
		if bank != nil {
			rendered = bank.RenderPCM(sound, SampleRate)
		}
		pcm[sound] = rendered
		return rendered
	}}
	return renderer.render(output, r, config.StartUnixNano, frames)
}

type audioTraceReader struct {
	scanner *bufio.Scanner
	last    int64
	line    int
}

func (r *audioTraceReader) next() (AudioTraceEvent, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return AudioTraceEvent{}, fmt.Errorf("read audio trace: %w", err)
		}
		return AudioTraceEvent{}, io.EOF
	}
	r.line++
	var event AudioTraceEvent
	if err := json.Unmarshal(r.scanner.Bytes(), &event); err != nil {
		return event, fmt.Errorf("audio trace line %d: %w", r.line, err)
	}
	if event.UnixNano <= 0 || event.UnixNano < r.last {
		return event, fmt.Errorf("audio trace line %d: timestamps must increase", r.line)
	}
	r.last = event.UnixNano
	return event, nil
}

type traceVoice struct {
	id     uint64
	pcm    []byte
	start  int64
	volume float64
}

type traceRenderer struct {
	pcm    func(int) []byte
	voices []traceVoice
}

func (m *traceRenderer) render(output io.Writer, r *audioTraceReader, start, frames int64) error {
	event, err := r.next()
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	buffer := make([]byte, 2048*4)
	for at := int64(0); at < frames; {
		// Consume earlier events without mixing discarded time. A voice's start
		// offset preserves its remaining tail at the beginning of the clip.
		for err == nil && durationSamples(event.UnixNano-start) <= at {
			if applyErr := m.apply(event, durationSamples(event.UnixNano-start)); applyErr != nil {
				return fmt.Errorf("audio trace line %d: %w", r.line, applyErr)
			}
			event, err = r.next()
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
		}
		end := min(frames, at+int64(len(buffer)/4))
		if err == nil {
			end = min(end, durationSamples(event.UnixNano-start))
		}
		chunk := buffer[:int(end-at)*4]
		m.mix(chunk, at)
		if err := writeAll(output, chunk); err != nil {
			return fmt.Errorf("write audio trace WAV samples: %w", err)
		}
		at = end
	}
	return nil
}

func (m *traceRenderer) apply(event AudioTraceEvent, at int64) error {
	m.discardFinished(at)
	switch event.Kind {
	case "play":
		if event.Voice == 0 || event.Volume < 0 || event.Volume > 1 || math.IsNaN(event.Volume) || math.IsInf(event.Volume, 0) {
			return errors.New("invalid audio play event")
		}
		for _, voice := range m.voices {
			if voice.id == event.Voice {
				return errors.New("duplicate active audio voice")
			}
		}
		pcm := m.pcm(event.Sound)
		if len(pcm) == 0 || len(pcm)%4 != 0 {
			return fmt.Errorf("sound %d is missing or not stereo PCM", event.Sound)
		}
		// The live player may briefly start its 33rd voice before closing the
		// oldest. Keep that ordering, with a bound for malformed input.
		if len(m.voices) >= 64 {
			return errors.New("too many simultaneous audio trace voices")
		}
		m.voices = append(m.voices, traceVoice{id: event.Voice, pcm: pcm, start: at, volume: event.Volume})
	case "stop":
		if event.Voice == 0 {
			return errors.New("invalid audio stop event")
		}
		for i, voice := range m.voices {
			if voice.id == event.Voice {
				m.voices = append(m.voices[:i], m.voices[i+1:]...)
				break
			}
		}
	case "end":
		m.voices = m.voices[:0]
	default:
		return fmt.Errorf("unknown audio event %q", event.Kind)
	}
	return nil
}

func (m *traceRenderer) discardFinished(at int64) {
	kept := m.voices[:0]
	for _, voice := range m.voices {
		if at < voice.start+int64(len(voice.pcm)/4) {
			kept = append(kept, voice)
		}
	}
	m.voices = kept
}

func (m *traceRenderer) mix(output []byte, start int64) {
	for offset := 0; offset < len(output); offset += 4 {
		at := start + int64(offset/4)
		var left, right float64
		for _, voice := range m.voices {
			pos := (at - voice.start) * 4
			if pos < 0 || pos+3 >= int64(len(voice.pcm)) {
				continue
			}
			left += float64(int16(binary.LittleEndian.Uint16(voice.pcm[pos:]))) * voice.volume
			right += float64(int16(binary.LittleEndian.Uint16(voice.pcm[pos+2:]))) * voice.volume
		}
		binary.LittleEndian.PutUint16(output[offset:], uint16(clampSample(int(left))))
		binary.LittleEndian.PutUint16(output[offset+2:], uint16(clampSample(int(right))))
	}
	m.discardFinished(start + int64(len(output)/4))
}

// Split the seconds and remainder to avoid overflowing for long recordings.
func durationSamples(nanos int64) int64 {
	seconds, remainder := nanos/int64(time.Second), nanos%int64(time.Second)
	adjust := int64(time.Second) / 2
	if remainder < 0 {
		adjust = -adjust
	}
	return seconds*SampleRate + (remainder*SampleRate+adjust)/int64(time.Second)
}

func writeWAVHeader(output io.Writer, frames int64) error {
	var header [44]byte
	copy(header[0:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(frames*4)+36)
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 2)
	binary.LittleEndian.PutUint32(header[24:], SampleRate)
	binary.LittleEndian.PutUint32(header[28:], SampleRate*4)
	binary.LittleEndian.PutUint16(header[32:], 4)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(frames*4))
	return writeAll(output, header[:])
}
