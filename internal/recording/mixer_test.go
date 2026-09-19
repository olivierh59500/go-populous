package recording

import (
	"encoding/binary"
	"testing"

	"go-populous/internal/populous"
)

func stereoPCM(left, right int16, frames int) []byte {
	pcm := make([]byte, frames*4)
	for at := 0; at < len(pcm); at += 4 {
		binary.LittleEndian.PutUint16(pcm[at:], uint16(left))
		binary.LittleEndian.PutUint16(pcm[at+2:], uint16(right))
	}
	return pcm
}

func sampleAt(pcm []byte, frame, channel int) int16 {
	return int16(binary.LittleEndian.Uint16(pcm[frame*4+channel*2:]))
}

func TestMixerSimulationDurationDoesNotDrift(t *testing.T) {
	m := newMixer(nil)
	var samples int
	for frame := 0; frame < 8*16; frame++ {
		pcm := m.frame(nil, 0, 0)
		samples += len(pcm) / 4
		want := (frame + 1) * SampleRate / FramesPerSecond
		if samples != want {
			t.Fatalf("after %d frames: %d audio samples, want %d", frame+1, samples, want)
		}
	}
	if samples != SampleRate*16 {
		t.Fatalf("16 seconds contains %d audio samples", samples)
	}
}

func TestMixerEffectsStartAtFrameBoundaryAndClip(t *testing.T) {
	m := newMixer(nil)
	m.pcm[0] = stereoPCM(30000, -30000, 2)
	first := m.frame(nil, 0, 0)
	for _, b := range first {
		if b != 0 {
			t.Fatal("audio before the first event is not silent")
		}
	}
	pcm := m.frame([]int{0, 0, -1, 999}, 0, 0)
	if left, right := sampleAt(pcm, 0, 0), sampleAt(pcm, 0, 1); left != 32767 || right != -32768 {
		t.Fatalf("overlapping effects = (%d, %d), want clipped signed stereo", left, right)
	}
	if sampleAt(pcm, 2, 0) != 0 || sampleAt(pcm, 2, 1) != 0 {
		t.Fatal("completed effects leaked into following samples")
	}
	pcm = m.frame([]int{0}, 0, 0)
	if left, right := sampleAt(pcm, 0, 0), sampleAt(pcm, 0, 1); left != 21000 || right != -21000 {
		t.Fatalf("effect volume = (%d, %d), want (21000, -21000)", left, right)
	}
}

func testMusicBank() *populous.SoundBank {
	bank := &populous.SoundBank{
		Patches:        []populous.SoundPatch{{Period: 357, Length: 1, Volume: 64, Sample: 0}},
		Samples:        [][]byte{{64}},
		Sequence:       []populous.SoundSequence{{Measure: 0, Times: 1}},
		MeasureLengths: []int{1},
		Measures:       make([]byte, 256),
	}
	bank.Measures[0] = 65
	return bank
}

func collectFrames(m *mixer, count int, events []int, player, opponent int) []byte {
	var pcm []byte
	for frame := 0; frame < count; frame++ {
		pcm = append(pcm, m.frame(events, player, opponent)...)
		events = nil
	}
	return pcm
}

func TestMixerMusicCadenceAndEffectDelay(t *testing.T) {
	m := newMixer(testMusicBank())
	m.pcm[0] = stereoPCM(10000, -10000, 1)
	pcm := collectFrames(m, 2, nil, 0, 0)
	firstBeat := audioTickSamples * populous.MusicTempo
	if sampleAt(pcm, firstBeat-1, 0) != 0 || sampleAt(pcm, firstBeat, 0) != 3500 || sampleAt(pcm, firstBeat, 1) != -3500 {
		t.Fatal("music should start at 200 ms, with 35% volume")
	}

	m = newMixer(testMusicBank())
	m.pcm[0] = stereoPCM(10000, -10000, 1)
	pcm = collectFrames(m, 8, []int{0}, 0, 0)
	if sampleAt(pcm, 0, 0) != 7000 {
		t.Fatal("the effect should start immediately at 70% volume")
	}
	delayedBeat := audioTickSamples * populous.MusicEffectDelayTicks
	for i := 1; i < delayedBeat; i++ {
		if sampleAt(pcm, i, 0) != 0 {
			t.Fatalf("music interrupted effect delay at sample %d", i)
		}
	}
	if sampleAt(pcm, delayedBeat, 0) != 3500 {
		t.Fatal("music did not resume after the original 40 audio ticks")
	}
}

func TestMixerHeartbeatUsesPopulationAndResets(t *testing.T) {
	m := newMixer(nil)
	m.pcm[populous.TuneHeart1] = stereoPCM(10000, 10000, 1)
	m.pcm[populous.TuneHeart2] = stereoPCM(20000, 20000, 1)
	pcm := collectFrames(m, 8, nil, 100, 100)
	if sampleAt(pcm, 32*audioTickSamples, 0) != 14000 {
		t.Fatal("secondary heartbeat should play at tick 32")
	}
	if sampleAt(pcm, 48*audioTickSamples, 0) != 7000 {
		t.Fatal("primary heartbeat should play at tick 48")
	}
	pcm = m.frame(nil, 0, 100)
	for _, b := range pcm {
		if b != 0 {
			t.Fatal("heartbeat should stop when the observed camp has no population")
		}
	}
	if m.heartbeat != 0 {
		t.Fatal("heartbeat tempo did not reset")
	}
}

func TestMixerRendersGameSoundBank(t *testing.T) {
	bank := testMusicBank()
	bank.Sequence = nil
	m := newMixer(bank)
	got := m.frame([]int{0}, 0, 0)
	want := bank.RenderPCM(0, SampleRate)
	for i := 0; i < len(want)/4; i++ {
		for ch := 0; ch < 2; ch++ {
			if sampleAt(got, i, ch) != int16(int(sampleAt(want, i, ch))*effectVolume/100) {
				t.Fatalf("rendered patch differs from game audio at sample %d channel %d", i, ch)
			}
		}
	}
}
