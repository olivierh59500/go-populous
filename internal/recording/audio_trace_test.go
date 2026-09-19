package recording

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAudioTraceLiveFileAndConcurrentVoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app-audio.jsonl")
	trace, err := NewAudioTrace(path)
	if err != nil {
		t.Fatal(err)
	}
	defer trace.Close()
	if _, err := NewAudioTrace(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing trace may be overwritten: %v", err)
	}
	var workers sync.WaitGroup
	for sound := 0; sound < 16; sound++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			voice := trace.Play(sound, 0.7)
			trace.Stop(voice)
		}()
	}
	workers.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &audioTraceReader{scanner: bufio.NewScanner(bytes.NewReader(data))}
	start, err := r.next()
	if err != nil || start.Kind != "start" || start.Version != 1 || start.SampleRate != SampleRate {
		t.Fatalf("missing visible header: %+v, %v", start, err)
	}
	playing, stopped := map[uint64]bool{}, map[uint64]bool{}
	for {
		event, err := r.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch event.Kind {
		case "play":
			if event.Voice == 0 || playing[event.Voice] || event.Volume != 0.7 {
				t.Fatalf("invalid concurrent voice: %+v", event)
			}
			playing[event.Voice] = true
		case "stop":
			if !playing[event.Voice] || stopped[event.Voice] {
				t.Fatalf("stop precedes or duplicates a start: %+v", event)
			}
			stopped[event.Voice] = true
		default:
			t.Fatalf("unexpected event before close: %+v", event)
		}
	}
	if len(playing) != 16 || len(stopped) != 16 {
		t.Fatalf("live trace lost voices: %d starts, %d stops", len(playing), len(stopped))
	}
	if err := trace.Close(); err != nil {
		t.Fatal(err)
	}
	if err := trace.Close(); err != nil || trace.Play(0, 1) != 0 {
		t.Fatal("closed capture should be idempotent and disabled")
	}
	final, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(final[len(data):], []byte(`"kind":"end"`)) {
		t.Fatalf("missing end marker: %s, %v", final, err)
	}
	var disabled *AudioTrace
	if disabled.Play(0, 1) != 0 || disabled.Close() != nil {
		t.Fatal("disabled capture must be a no-op")
	}
	disabled.Stop(1)
}

const traceEpoch = int64(1700000000000000000)

func traceBytes(t *testing.T, events ...AudioTraceEvent) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := json.NewEncoder(&data)
	for _, event := range events {
		if err := writer.Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	return data.Bytes()
}

func sampleTimestamp(frame int64) int64 {
	return traceEpoch + (frame*int64(time.Second)+SampleRate/2)/SampleRate
}

func renderTraceTestPCM(t *testing.T, source map[int][]byte, start int64, frames int64, events ...AudioTraceEvent) []byte {
	t.Helper()
	r := &audioTraceReader{scanner: bufio.NewScanner(bytes.NewReader(traceBytes(t, events...)))}
	m := traceRenderer{pcm: func(sound int) []byte { return source[sound] }}
	var output bytes.Buffer
	if err := m.render(&output, r, start, frames); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestAudioTraceSampleTimingVolumeStopAndNaturalEnding(t *testing.T) {
	pcm := renderTraceTestPCM(t, map[int][]byte{0: stereoPCM(10000, -10000, 4)}, traceEpoch, 9,
		AudioTraceEvent{Kind: "play", UnixNano: sampleTimestamp(2), Voice: 1, Volume: 0.35},
		AudioTraceEvent{Kind: "play", UnixNano: sampleTimestamp(3), Voice: 2, Volume: 0.7},
		AudioTraceEvent{Kind: "stop", UnixNano: sampleTimestamp(4), Voice: 1},
	)
	want := []int16{0, 0, 3500, 10500, 7000, 7000, 7000, 0, 0}
	for frame, expected := range want {
		if left, right := sampleAt(pcm, frame, 0), sampleAt(pcm, frame, 1); left != expected || right != -expected {
			t.Fatalf("sample %d = (%d,%d), want (%d,%d)", frame, left, right, expected, -expected)
		}
	}
}

func TestAudioTraceTrimPreservesActiveTailsAndStops(t *testing.T) {
	pcm := renderTraceTestPCM(t, map[int][]byte{0: stereoPCM(12000, -12000, 8)}, sampleTimestamp(4), 5,
		AudioTraceEvent{Kind: "play", UnixNano: sampleTimestamp(0), Voice: 1, Volume: 0.5},
		AudioTraceEvent{Kind: "play", UnixNano: sampleTimestamp(1), Voice: 2, Volume: 0.25},
		AudioTraceEvent{Kind: "stop", UnixNano: sampleTimestamp(2), Voice: 2},
		AudioTraceEvent{Kind: "stop", UnixNano: sampleTimestamp(6), Voice: 1},
	)
	for frame, expected := range []int16{6000, 6000, 0, 0, 0} {
		if left := sampleAt(pcm, frame, 0); left != expected {
			t.Fatalf("trimmed sample %d = %d, want %d", frame, left, expected)
		}
	}
}

func TestAudioTraceClipsMixAndEndSilencesVoices(t *testing.T) {
	pcm := renderTraceTestPCM(t, map[int][]byte{0: stereoPCM(30000, -30000, 8)}, traceEpoch, 4,
		AudioTraceEvent{Kind: "play", UnixNano: traceEpoch, Voice: 1, Volume: 1},
		AudioTraceEvent{Kind: "play", UnixNano: traceEpoch, Voice: 2, Volume: 1},
		AudioTraceEvent{Kind: "end", UnixNano: sampleTimestamp(2)},
	)
	for frame := range 2 {
		if sampleAt(pcm, frame, 0) != 32767 || sampleAt(pcm, frame, 1) != -32768 {
			t.Fatalf("overlapping voices did not saturate correctly: %v", pcm)
		}
	}
	if sampleAt(pcm, 2, 0) != 0 || sampleAt(pcm, 3, 0) != 0 {
		t.Fatal("closed capture leaked voices beyond its end")
	}
}

func TestAudioTraceWAVPreservesBankAndHasExactDuration(t *testing.T) {
	bank := testMusicBank()
	before, err := json.Marshal(bank)
	if err != nil {
		t.Fatal(err)
	}
	input := traceBytes(t,
		AudioTraceEvent{Kind: "start", UnixNano: traceEpoch, Version: 1, SampleRate: SampleRate},
		AudioTraceEvent{Kind: "play", UnixNano: traceEpoch + int64(20*time.Millisecond), Voice: 1, Volume: 0.5},
	)
	var wav bytes.Buffer
	if err := RenderAudioTraceWAV(&wav, bytes.NewReader(input), bank, AudioTraceRenderConfig{Duration: time.Second / 8}); err != nil {
		t.Fatal(err)
	}
	got := wav.Bytes()
	if string(got[:4]) != "RIFF" || string(got[8:16]) != "WAVEfmt " || binary.LittleEndian.Uint32(got[24:]) != SampleRate {
		t.Fatal("invalid stereo PCM WAV header")
	}
	if want := 44 + int(durationSamples(int64(time.Second/8)))*4; len(got) != want || int(binary.LittleEndian.Uint32(got[40:])) != want-44 {
		t.Fatalf("WAV size = %d, want %d", len(got), want)
	}
	sound := bank.RenderPCM(0, SampleRate)
	start := SampleRate / 50
	for at := 0; at < len(sound)/4; at++ {
		if sampleAt(got[44:], start+at, 0) != int16(float64(sampleAt(sound, at, 0))*0.5) {
			t.Fatalf("source sound differs at sample %d", at)
		}
	}
	after, _ := json.Marshal(bank)
	if !bytes.Equal(before, after) {
		t.Fatal("audio reconstruction modified the game sound bank")
	}
}

func TestAudioTraceStreamingChunksRemainBounded(t *testing.T) {
	input := traceBytes(t, AudioTraceEvent{Kind: "start", UnixNano: traceEpoch, Version: 1, SampleRate: SampleRate})
	output := &audioCountingWriter{}
	if err := RenderAudioTraceWAV(output, bytes.NewReader(input), nil, AudioTraceRenderConfig{Duration: 20 * time.Second}); err != nil {
		t.Fatal(err)
	}
	if output.bytes != 44+20*SampleRate*4 || output.maxWrite > 2048*4 {
		t.Fatalf("unbounded or incomplete streaming: %+v", output)
	}
}

type audioCountingWriter struct{ bytes, maxWrite int }

func (w *audioCountingWriter) Write(p []byte) (int, error) {
	w.bytes += len(p)
	w.maxWrite = max(w.maxWrite, len(p))
	return len(p), nil
}

func TestAudioTraceRejectsInvalidAndOutOfOrderEvents(t *testing.T) {
	header := AudioTraceEvent{Kind: "start", UnixNano: traceEpoch, Version: 1, SampleRate: SampleRate}
	for _, events := range [][]AudioTraceEvent{
		{{Kind: "play", UnixNano: traceEpoch, Voice: 1, Volume: 1.5}},
		{{Kind: "play", UnixNano: traceEpoch, Voice: 1, Sound: 999, Volume: 1}},
		{{Kind: "play", UnixNano: traceEpoch, Voice: 1, Volume: 1}, {Kind: "stop", UnixNano: traceEpoch - 1, Voice: 1}},
		{{Kind: "play", UnixNano: traceEpoch, Voice: 1, Volume: 1}, {Kind: "play", UnixNano: traceEpoch, Voice: 1, Volume: 1}},
		{{Kind: "stop", UnixNano: traceEpoch}},
		{{Kind: "unexpected", UnixNano: traceEpoch}},
	} {
		input := traceBytes(t, append([]AudioTraceEvent{header}, events...)...)
		if err := RenderAudioTraceWAV(io.Discard, bytes.NewReader(input), testMusicBank(), AudioTraceRenderConfig{Duration: time.Second}); err == nil {
			t.Fatalf("accepted invalid events: %+v", events)
		}
	}
	for _, duration := range []time.Duration{0, -time.Second, 7 * time.Hour} {
		if err := RenderAudioTraceWAV(io.Discard, bytes.NewReader(traceBytes(t, header)), nil, AudioTraceRenderConfig{Duration: duration}); err == nil {
			t.Fatalf("accepted invalid WAV duration %s", duration)
		}
	}
	if err := RenderAudioTraceWAV(io.Discard, strings.NewReader("{}\n"), nil, AudioTraceRenderConfig{Duration: time.Second}); err == nil {
		t.Fatal("accepted missing header")
	}
}

func TestAudioTraceManyFinishedSoundsDoNotKeepVoices(t *testing.T) {
	m := traceRenderer{pcm: func(int) []byte { return stereoPCM(1000, -1000, 1) }}
	for at := int64(0); at < 1000; at++ {
		if err := m.apply(AudioTraceEvent{Kind: "play", Voice: uint64(at + 1), Volume: 1}, at); err != nil {
			t.Fatal(err)
		}
		if len(m.voices) != 1 {
			t.Fatalf("expired voices accumulated at %d", at)
		}
	}
	if !reflect.DeepEqual(m.voices[0].pcm, stereoPCM(1000, -1000, 1)) {
		t.Fatal("voice PCM mutated while discarding finished sounds")
	}
}
