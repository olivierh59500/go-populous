package recording

import (
	"encoding/binary"

	"go-populous/internal/populous"
)

const (
	SampleRate       = 44100
	FramesPerSecond  = 8
	audioTickSamples = SampleRate / 50
	maxVoices        = 32
	effectVolume     = 70
	musicVolume      = 35
)

type voice struct {
	pcm    []byte
	pos    int
	volume int
}

// mixer uses the simulation clock, never the audio device or the wall clock.
// One simulation frame alternates between 5512 and 5513 stereo sample frames.
type mixer struct {
	bank      *populous.SoundBank
	pcm       map[int][]byte
	voices    []voice
	music     populous.MusicState
	heartbeat int
	samples   int64
	frames    int64
	nextTick  int64
	output    []byte
}

func newMixer(bank *populous.SoundBank) *mixer {
	m := &mixer{
		bank:     bank,
		pcm:      make(map[int][]byte),
		voices:   make([]voice, 0, maxVoices),
		nextTick: audioTickSamples,
		output:   make([]byte, (SampleRate/FramesPerSecond+1)*4),
	}
	if bank != nil {
		for id := range bank.Patches {
			if pcm := bank.RenderPCM(id, SampleRate); len(pcm) != 0 {
				m.pcm[id] = pcm
			}
		}
	}
	return m
}

func (m *mixer) play(id, volume int) bool {
	pcm := m.pcm[id]
	if len(pcm) == 0 {
		return false
	}
	if len(m.voices) == maxVoices {
		copy(m.voices, m.voices[1:])
		m.voices = m.voices[:maxVoices-1]
	}
	m.voices = append(m.voices, voice{pcm: pcm, volume: volume})
	return true
}

// frame returns borrowed PCM storage, valid until the next call.
func (m *mixer) frame(events []int, playerPopulation, opponentPopulation int) []byte {
	for _, id := range events {
		if m.play(id, effectVolume) && m.bank != nil {
			m.music.Delay(populous.MusicEffectDelayTicks)
		}
	}
	if playerPopulation <= 0 {
		m.heartbeat = 0
	}
	m.frames++
	end := m.frames * SampleRate / FramesPerSecond
	output := m.output[:int(end-m.samples)*4]
	clear(output)
	pos := 0
	for m.samples < end {
		if m.samples == m.nextTick {
			for _, id := range m.bank.MusicTick(&m.music) {
				m.play(id, musicVolume)
			}
			var sounds []int
			m.heartbeat, sounds = populous.HeartbeatTick(playerPopulation, opponentPopulation, m.heartbeat)
			for _, id := range sounds {
				m.play(id, effectVolume)
			}
			m.nextTick += audioTickSamples
		}
		stop := end
		if m.nextTick < stop {
			stop = m.nextTick
		}
		size := int(stop-m.samples) * 4
		m.mix(output[pos : pos+size])
		m.samples = stop
		pos += size
	}
	return output
}

func (m *mixer) mix(output []byte) {
	for at := 0; at < len(output); at += 4 {
		var left, right int
		for i := range m.voices {
			v := &m.voices[i]
			if v.pos >= len(v.pcm) {
				continue
			}
			left += int(int16(binary.LittleEndian.Uint16(v.pcm[v.pos:]))) * v.volume
			right += int(int16(binary.LittleEndian.Uint16(v.pcm[v.pos+2:]))) * v.volume
			v.pos += 4
		}
		binary.LittleEndian.PutUint16(output[at:], uint16(clampSample(left/100)))
		binary.LittleEndian.PutUint16(output[at+2:], uint16(clampSample(right/100)))
	}
	kept := m.voices[:0]
	for _, v := range m.voices {
		if v.pos < len(v.pcm) {
			kept = append(kept, v)
		}
	}
	m.voices = kept
}

func clampSample(value int) int16 {
	if value > 32767 {
		return 32767
	}
	if value < -32768 {
		return -32768
	}
	return int16(value)
}
