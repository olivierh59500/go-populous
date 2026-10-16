package populous

import (
	"encoding/binary"
	"fmt"
	"math"
)

const amigaAudioClock = 3579545.0
const MusicTempo = 10
const MusicEffectDelayTicks = MusicTempo * 4
const WordSoundBase = 104

var conquestWords = [...]int{
	9, -1,
	5, -1,
	3, -1,
	7, -1,
	1, 8,
	2, -1,
	1, 2,
	9, 6,
	1, 6,
	3, 6,
}

var endConquestWords = [...]int{0, 1, 3, 5, 6}

type SoundPatch struct {
	Period int
	Length int
	Volume int
	Sample int
	Repeat int
}

type SoundSequence struct {
	Measure int
	Times   int
}

type MusicState struct {
	SequenceIndex int
	Beat          int
	Time          int
	Now           int
	Tempo         int
}

type SoundBank struct {
	MeasureLengths []int
	Sequence       []SoundSequence
	Measures       []byte
	Patches        []SoundPatch
	Samples        [][]byte
}

func (s *MusicState) Delay(ticks int) {
	if ticks > s.Tempo {
		s.Tempo = ticks
	}
}

func HeartbeatTick(playerPopulation, opponentPopulation, tempoNow int) (int, []int) {
	if playerPopulation <= 0 {
		return 0, nil
	}
	tempo := 50 - (opponentPopulation/playerPopulation)*2
	if tempo < 7 {
		tempo = 7
	}
	if tempoNow > tempo {
		tempoNow = tempo
	}
	tempoNow++
	if tempoNow >= tempo {
		return 0, []int{TuneHeart1}
	}
	if tempoNow == tempo-tempo/3 {
		return tempoNow, []int{TuneHeart2}
	}
	return tempoNow, nil
}

func DecodeAmigaSoundBank(data []byte) (*SoundBank, error) {
	const measlnSize = 128
	if len(data) < measlnSize+2 {
		return nil, fmt.Errorf("sound bank too short: %d bytes", len(data))
	}

	measureLengths := make([]int, 64)
	for i := range measureLengths {
		measureLengths[i] = int(binary.BigEndian.Uint16(data[i*2:]))
	}
	offset := measlnSize
	seqLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if seqLen < 0 || seqLen > 128 {
		return nil, fmt.Errorf("invalid sound sequence length %d", seqLen)
	}
	if len(data) < offset+seqLen*4+4 {
		return nil, fmt.Errorf("sound bank sequence table truncated")
	}
	sequence := make([]SoundSequence, seqLen)
	for i := range sequence {
		entry := data[offset+i*4:]
		sequence[i] = SoundSequence{
			Measure: int(binary.BigEndian.Uint16(entry[0:2])),
			Times:   int(binary.BigEndian.Uint16(entry[2:4])),
		}
	}
	offset += seqLen * 4

	measureLength := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4
	if measureLength < 0 || len(data) < offset+measureLength+2 {
		return nil, fmt.Errorf("sound bank measures truncated")
	}
	measures := append([]byte(nil), data[offset:offset+measureLength]...)
	offset += measureLength

	noSounds := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if noSounds < 0 || noSounds > 512 {
		return nil, fmt.Errorf("invalid sound patch count %d", noSounds)
	}
	if len(data) < offset+noSounds*8+2 {
		return nil, fmt.Errorf("sound patch table truncated")
	}
	patches := make([]SoundPatch, noSounds)
	for i := range patches {
		entry := data[offset+i*8:]
		patches[i] = SoundPatch{
			Period: int(binary.BigEndian.Uint16(entry[0:2])),
			Length: int(binary.BigEndian.Uint16(entry[2:4])),
			Volume: int(entry[4]),
			Sample: int(entry[5]),
			Repeat: int(binary.BigEndian.Uint16(entry[6:8])),
		}
	}
	offset += noSounds * 8

	sampleCount := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if sampleCount < 0 || sampleCount > 512 {
		return nil, fmt.Errorf("invalid sample count %d", sampleCount)
	}
	if len(data) < offset+sampleCount*8+4 {
		return nil, fmt.Errorf("sample table truncated")
	}
	sampleLengths := make([]int, sampleCount)
	for i := range sampleLengths {
		sampleLengths[i] = int(binary.BigEndian.Uint16(data[offset+i*8:]))
	}
	offset += sampleCount * 8

	allSampleLength := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4
	if allSampleLength < 0 || len(data) < offset+allSampleLength {
		return nil, fmt.Errorf("sample data truncated")
	}
	allSamples := data[offset : offset+allSampleLength]
	samples := make([][]byte, sampleCount)
	cursor := 0
	for i, length := range sampleLengths {
		if length < 0 || cursor+length > len(allSamples) {
			return nil, fmt.Errorf("sample %d length %d exceeds sample data", i, length)
		}
		samples[i] = append([]byte(nil), allSamples[cursor:cursor+length]...)
		cursor += length
	}

	return &SoundBank{
		MeasureLengths: measureLengths,
		Sequence:       sequence,
		Measures:       measures,
		Patches:        patches,
		Samples:        samples,
	}, nil
}

func (b *SoundBank) InsertBankAt(other *SoundBank, soundIDBase int) error {
	if b == nil {
		return fmt.Errorf("target sound bank is nil")
	}
	if other == nil {
		return fmt.Errorf("source sound bank is nil")
	}
	if soundIDBase < 0 {
		return fmt.Errorf("invalid sound id base %d", soundIDBase)
	}

	sampleBase := len(b.Samples)
	for _, sample := range other.Samples {
		b.Samples = append(b.Samples, append([]byte(nil), sample...))
	}
	requiredPatches := soundIDBase + len(other.Patches)
	if len(b.Patches) < requiredPatches {
		patches := make([]SoundPatch, requiredPatches)
		copy(patches, b.Patches)
		b.Patches = patches
	}
	for i, patch := range other.Patches {
		if patch.Sample >= 0 {
			patch.Sample += sampleBase
		}
		b.Patches[soundIDBase+i] = patch
	}
	return nil
}

func ConquestVoiceSequence(nextLevelIndex int, completed bool) []int {
	if completed {
		sequence := make([]int, 0, len(endConquestWords))
		for _, word := range endConquestWords {
			sequence = append(sequence, WordSoundBase+word)
		}
		return sequence
	}

	gotTo := clamp((nextLevelIndex*5)/250, 0, len(conquestWords)/2-1)
	sequence := []int{WordSoundBase}
	for i := 0; i < 2; i++ {
		word := conquestWords[gotTo*2+i]
		if word >= 0 {
			sequence = append(sequence, WordSoundBase+word)
		}
	}
	return sequence
}

func (b *SoundBank) MusicTick(state *MusicState) []int {
	if b == nil || state == nil || len(b.Sequence) == 0 || len(b.Measures) == 0 {
		return nil
	}
	tempo := state.Tempo
	if tempo <= 0 {
		tempo = MusicTempo
	}
	state.Now++
	if state.Now != tempo {
		return nil
	}
	state.Now = 0
	state.Tempo = MusicTempo
	return b.playMusicBeat(state)
}

func (b *SoundBank) playMusicBeat(state *MusicState) []int {
	if state.SequenceIndex < 0 || state.SequenceIndex >= len(b.Sequence) {
		state.SequenceIndex = 0
	}
	sequence := b.Sequence[state.SequenceIndex]
	measure := sequence.Measure
	beat := state.Beat
	var sounds []int
	if beat >= 0 && beat < 64 {
		offset := measure * 256
		if offset >= 0 && offset+192+beat < len(b.Measures) {
			for channel := 0; channel < 4; channel++ {
				id := int(b.Measures[offset+channel*64+beat]) - 65
				if b.HasSound(id) {
					sounds = append(sounds, id)
				}
			}
		}
	}

	state.Beat++
	length := 64
	if measure >= 0 && measure < len(b.MeasureLengths) && b.MeasureLengths[measure] > 0 {
		length = b.MeasureLengths[measure]
		if length > 64 {
			length = 64
		}
	}
	if state.Beat >= length {
		state.Beat = 0
		state.Time++
		times := sequence.Times
		if times <= 0 {
			times = 1
		}
		if state.Time >= times {
			state.Time = 0
			state.SequenceIndex++
			if state.SequenceIndex >= len(b.Sequence) {
				state.SequenceIndex = 0
			}
		}
	}
	return sounds
}

func (b *SoundBank) HasSound(id int) bool {
	if b == nil || id < 0 || id >= len(b.Patches) {
		return false
	}
	patch := b.Patches[id]
	return patch.Period > 0 && patch.Length > 0 && patch.Sample >= 0 && patch.Sample < len(b.Samples) && len(b.Samples[patch.Sample]) > 0
}

func (b *SoundBank) RenderPCM(id, sampleRate int) []byte {
	if sampleRate <= 0 || !b.HasSound(id) {
		return nil
	}
	patch := b.Patches[id]
	src := b.Samples[patch.Sample]
	srcLen := patch.Length
	if srcLen > len(src) {
		srcLen = len(src)
	}
	if srcLen <= 0 {
		return nil
	}

	sourceRate := amigaAudioClock / float64(patch.Period)
	outputSamples := int(math.Ceil(float64(srcLen) * float64(sampleRate) / sourceRate))
	if outputSamples <= 0 {
		return nil
	}

	volume := patch.Volume
	if volume < 0 {
		volume = 0
	}
	if volume > 64 {
		volume = 64
	}

	pcm := make([]byte, outputSamples*4)
	step := sourceRate / float64(sampleRate)
	fadeSamples := 32
	if outputSamples/8 < fadeSamples {
		fadeSamples = outputSamples / 8
	}
	for i := 0; i < outputSamples; i++ {
		sourcePosition := float64(i) * step
		sourceIndex := int(sourcePosition)
		if sourceIndex >= srcLen {
			sourceIndex = srcLen - 1
		}
		sample := float64(int8(src[sourceIndex]))
		if sourceIndex+1 < srcLen {
			next := float64(int8(src[sourceIndex+1]))
			fraction := sourcePosition - float64(sourceIndex)
			sample += (next - sample) * fraction
		}
		value := int(sample*256) * volume / 64
		if fadeSamples > 0 && i < fadeSamples {
			value = value * i / fadeSamples
		}
		if fadeSamples > 0 && i >= outputSamples-fadeSamples {
			value = value * (outputSamples - 1 - i) / fadeSamples
		}
		at := i * 4
		binary.LittleEndian.PutUint16(pcm[at:at+2], uint16(int16(value)))
		binary.LittleEndian.PutUint16(pcm[at+2:at+4], uint16(int16(value)))
	}
	return pcm
}
