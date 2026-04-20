package populous

import (
	"encoding/binary"
	"testing"
)

func TestDecodeAmigaSoundBankAndRenderPCM(t *testing.T) {
	data := make([]byte, 0)
	data = append(data, make([]byte, 128)...)
	data = binary.BigEndian.AppendUint16(data, 0)
	data = binary.BigEndian.AppendUint32(data, 0)
	data = binary.BigEndian.AppendUint16(data, 1)
	data = binary.BigEndian.AppendUint16(data, 357)
	data = binary.BigEndian.AppendUint16(data, 4)
	data = append(data, 64, 0)
	data = binary.BigEndian.AppendUint16(data, 0)
	data = binary.BigEndian.AppendUint16(data, 1)
	data = binary.BigEndian.AppendUint16(data, 4)
	data = binary.BigEndian.AppendUint16(data, 4)
	data = append(data, 0, 0)
	data = binary.BigEndian.AppendUint16(data, 0)
	data = binary.BigEndian.AppendUint32(data, 4)
	data = append(data, 0x00, 0x40, 0xff, 0x80)

	bank, err := DecodeAmigaSoundBank(data)
	if err != nil {
		t.Fatalf("DecodeAmigaSoundBank returned error: %v", err)
	}
	if !bank.HasSound(0) {
		t.Fatal("decoded bank does not expose sound 0")
	}
	if len(bank.MeasureLengths) != 64 {
		t.Fatalf("decoded measure length count = %d, want 64", len(bank.MeasureLengths))
	}
	if len(bank.Sequence) != 0 {
		t.Fatalf("decoded sequence count = %d, want 0", len(bank.Sequence))
	}
	pcm := bank.RenderPCM(0, 44100)
	if len(pcm) == 0 || len(pcm)%4 != 0 {
		t.Fatalf("pcm length = %d, want non-empty stereo frames", len(pcm))
	}
}

func TestInsertBankAtMapsWordSounds(t *testing.T) {
	base := &SoundBank{
		Patches: []SoundPatch{{Period: 357, Length: 4, Volume: 64, Sample: 0}},
		Samples: [][]byte{{0, 1, 2, 3}},
	}
	words := &SoundBank{
		Patches: []SoundPatch{{Period: 428, Length: 3, Volume: 50, Sample: 0}},
		Samples: [][]byte{{4, 5, 6}},
	}

	if err := base.InsertBankAt(words, WordSoundBase); err != nil {
		t.Fatalf("InsertBankAt returned error: %v", err)
	}
	if !base.HasSound(0) {
		t.Fatal("base sound disappeared after insert")
	}
	if !base.HasSound(WordSoundBase) {
		t.Fatal("inserted word sound is not playable")
	}
	if sample := base.Patches[WordSoundBase].Sample; sample != 1 {
		t.Fatalf("inserted sample index = %d, want 1", sample)
	}
}

func TestConquestVoiceSequence(t *testing.T) {
	if got := ConquestVoiceSequence(0, false); !sameInts(got, []int{104, 113}) {
		t.Fatalf("first conquest voice = %v, want [104 113]", got)
	}
	if got := ConquestVoiceSequence(300, false); !sameInts(got, []int{104, 105, 106}) {
		t.Fatalf("late conquest voice = %v, want [104 105 106]", got)
	}
	if got := ConquestVoiceSequence(0, true); !sameInts(got, []int{104, 105, 107, 109, 110}) {
		t.Fatalf("completed conquest voice = %v, want [104 105 107 109 110]", got)
	}
}

func TestHeartbeatTickMatchesOriginalTempo(t *testing.T) {
	tempoNow, sounds := HeartbeatTick(100, 100, 0)
	if tempoNow != 1 || len(sounds) != 0 {
		t.Fatalf("first heartbeat tick = tempo %d sounds %v, want tempo 1 no sound", tempoNow, sounds)
	}

	tempoNow = 47
	tempoNow, sounds = HeartbeatTick(100, 100, tempoNow)
	if tempoNow != 0 || len(sounds) != 1 || sounds[0] != TuneHeart1 {
		t.Fatalf("main heartbeat = tempo %d sounds %v, want reset and TuneHeart1", tempoNow, sounds)
	}

	tempoNow = 31
	tempoNow, sounds = HeartbeatTick(100, 100, tempoNow)
	if tempoNow != 32 || len(sounds) != 1 || sounds[0] != TuneHeart2 {
		t.Fatalf("second heartbeat = tempo %d sounds %v, want TuneHeart2 at tempo-beat_two", tempoNow, sounds)
	}

	tempoNow, sounds = HeartbeatTick(0, 100, 12)
	if tempoNow != 0 || len(sounds) != 0 {
		t.Fatalf("zero population heartbeat = tempo %d sounds %v, want silence", tempoNow, sounds)
	}
}

func TestMusicTickPlaysMeasureSequence(t *testing.T) {
	bank := &SoundBank{
		MeasureLengths: make([]int, 64),
		Sequence: []SoundSequence{
			{Measure: 1, Times: 1},
			{Measure: 2, Times: 1},
		},
		Measures: make([]byte, 3*256),
		Patches: []SoundPatch{
			{Period: 357, Length: 4, Volume: 64, Sample: 0},
			{Period: 357, Length: 4, Volume: 64, Sample: 0},
			{Period: 357, Length: 4, Volume: 64, Sample: 0},
		},
		Samples: [][]byte{{0, 1, 2, 3}},
	}
	bank.MeasureLengths[1] = 2
	bank.MeasureLengths[2] = 1
	bank.Measures[1*256+0] = 65
	bank.Measures[1*256+64] = 66
	bank.Measures[1*256+1] = 67
	bank.Measures[2*256+0] = 66

	state := &MusicState{Now: MusicTempo - 1}
	sounds := bank.MusicTick(state)
	if !sameInts(sounds, []int{0, 1}) || state.Beat != 1 || state.SequenceIndex != 0 {
		t.Fatalf("first music beat = sounds %v beat %d seq %d, want [0 1] beat 1 seq 0", sounds, state.Beat, state.SequenceIndex)
	}

	if sounds := bank.MusicTick(state); len(sounds) != 0 || state.Now != 1 {
		t.Fatalf("intermediate music tick = sounds %v now %d, want silence now 1", sounds, state.Now)
	}

	state.Now = MusicTempo - 1
	sounds = bank.MusicTick(state)
	if !sameInts(sounds, []int{2}) || state.Beat != 0 || state.SequenceIndex != 1 {
		t.Fatalf("second music beat = sounds %v beat %d seq %d, want [2] beat 0 seq 1", sounds, state.Beat, state.SequenceIndex)
	}

	state.Now = MusicTempo - 1
	sounds = bank.MusicTick(state)
	if !sameInts(sounds, []int{1}) || state.Beat != 0 || state.SequenceIndex != 0 {
		t.Fatalf("third music beat = sounds %v beat %d seq %d, want [1] beat 0 seq 0", sounds, state.Beat, state.SequenceIndex)
	}
}

func TestMusicTickRepeatsSequenceItemTimes(t *testing.T) {
	bank := &SoundBank{
		MeasureLengths: make([]int, 64),
		Sequence: []SoundSequence{
			{Measure: 0, Times: 2},
			{Measure: 1, Times: 1},
		},
		Measures: make([]byte, 2*256),
		Patches: []SoundPatch{
			{Period: 357, Length: 4, Volume: 64, Sample: 0},
			{Period: 357, Length: 4, Volume: 64, Sample: 0},
		},
		Samples: [][]byte{{0, 1, 2, 3}},
	}
	bank.MeasureLengths[0] = 1
	bank.MeasureLengths[1] = 1
	bank.Measures[0] = 65
	bank.Measures[256] = 66

	state := &MusicState{Now: MusicTempo - 1}
	sounds := bank.MusicTick(state)
	if !sameInts(sounds, []int{0}) || state.Time != 1 || state.SequenceIndex != 0 {
		t.Fatalf("first repeated beat = sounds %v time %d seq %d, want [0] time 1 seq 0", sounds, state.Time, state.SequenceIndex)
	}

	state.Now = MusicTempo - 1
	sounds = bank.MusicTick(state)
	if !sameInts(sounds, []int{0}) || state.Time != 0 || state.SequenceIndex != 1 {
		t.Fatalf("second repeated beat = sounds %v time %d seq %d, want [0] time 0 seq 1", sounds, state.Time, state.SequenceIndex)
	}
}

func sameInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
