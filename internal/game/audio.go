package game

import (
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"go-populous/internal/populous"
)

const audioSampleRate = 44100
const maxActiveSounds = 32
const minSoundLifetime = time.Second
const audioTickInterval = 20 * time.Millisecond
const effectVolume = 0.7
const musicVolume = 0.35

type activeSound struct {
	player    *audio.Player
	keepUntil time.Time
}

type soundPlayer struct {
	context    *audio.Context
	sampleRate int
	bank       *populous.SoundBank
	pcm        map[int][]byte

	mu       sync.Mutex
	active   []activeSound
	requests chan int

	musicState     populous.MusicState
	musicEnabled   bool
	effectsEnabled bool

	heartbeatPlayerPopulation   int
	heartbeatOpponentPopulation int
	heartbeatTempoNow           int
}

func newSoundPlayer(bank *populous.SoundBank) *soundPlayer {
	if bank == nil {
		return nil
	}
	context := audio.CurrentContext()
	sampleRate := audioSampleRate
	if context == nil {
		context = audio.NewContext(audioSampleRate)
	} else {
		sampleRate = context.SampleRate()
	}
	player := &soundPlayer{
		context:        context,
		sampleRate:     sampleRate,
		bank:           bank,
		pcm:            map[int][]byte{},
		requests:       make(chan int, 32),
		musicEnabled:   len(bank.Sequence) > 0 && len(bank.Measures) > 0,
		effectsEnabled: true,
	}
	for id := range bank.Patches {
		if pcm := bank.RenderPCM(id, sampleRate); len(pcm) > 0 {
			player.pcm[id] = pcm
		}
	}
	if len(player.pcm) == 0 {
		return nil
	}
	go player.run()
	return player
}

func (p *soundPlayer) Play(id int) {
	if p == nil || len(p.pcm[id]) == 0 {
		return
	}
	p.mu.Lock()
	enabled := p.effectsEnabled
	p.mu.Unlock()
	if !enabled {
		return
	}
	select {
	case p.requests <- id:
		p.delayMusicForEffect()
	default:
	}
}

func (p *soundPlayer) PlaySequence(ids []int) {
	if p == nil || len(ids) == 0 {
		return
	}
	sequence := append([]int(nil), ids...)
	go func() {
		for _, id := range sequence {
			p.Play(id)
			time.Sleep(p.soundDuration(id) + 120*time.Millisecond)
		}
	}()
}

func (p *soundPlayer) run() {
	ticker := time.NewTicker(audioTickInterval)
	defer ticker.Stop()
	for {
		select {
		case id := <-p.requests:
			p.playNow(id)
		case <-ticker.C:
			p.playTimedAudio()
		}
	}
}

func (p *soundPlayer) SetHeartbeat(playerPopulation, opponentPopulation int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.heartbeatPlayerPopulation = playerPopulation
	p.heartbeatOpponentPopulation = opponentPopulation
	if playerPopulation <= 0 {
		p.heartbeatTempoNow = 0
	}
	p.mu.Unlock()
}

func (p *soundPlayer) ResetHeartbeat() {
	p.SetHeartbeat(0, 0)
}

func (p *soundPlayer) SetMusicEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.musicEnabled = enabled
	if !enabled {
		p.musicState = populous.MusicState{}
	}
	p.mu.Unlock()
}

func (p *soundPlayer) MusicEnabled() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.musicEnabled
}

func (p *soundPlayer) SetEffectsEnabled(enabled bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.effectsEnabled = enabled
	if !enabled {
		p.heartbeatTempoNow = 0
	}
	p.mu.Unlock()
}

func (p *soundPlayer) EffectsEnabled() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.effectsEnabled
}

func (p *soundPlayer) delayMusicForEffect() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.musicEnabled {
		p.musicState.Delay(populous.MusicEffectDelayTicks)
	}
	p.mu.Unlock()
}

func (p *soundPlayer) playTimedAudio() {
	p.mu.Lock()
	effectsEnabled := p.effectsEnabled
	var musicSounds []int
	if p.musicEnabled && p.bank != nil {
		musicSounds = p.bank.MusicTick(&p.musicState)
	}
	var sounds []int
	if effectsEnabled {
		var tempoNow int
		tempoNow, sounds = populous.HeartbeatTick(
			p.heartbeatPlayerPopulation,
			p.heartbeatOpponentPopulation,
			p.heartbeatTempoNow,
		)
		p.heartbeatTempoNow = tempoNow
	}
	p.mu.Unlock()
	for _, sound := range musicSounds {
		p.playNowVolume(sound, musicVolume)
	}
	for _, sound := range sounds {
		p.playNow(sound)
	}
}

func (p *soundPlayer) soundDuration(id int) time.Duration {
	pcm := p.pcm[id]
	if len(pcm) == 0 || p.sampleRate <= 0 {
		return 250 * time.Millisecond
	}
	duration := time.Duration(len(pcm)/4) * time.Second / time.Duration(p.sampleRate)
	if duration <= 0 {
		return 250 * time.Millisecond
	}
	return duration
}

func (p *soundPlayer) playNow(id int) {
	p.playNowVolume(id, effectVolume)
}

func (p *soundPlayer) playNowVolume(id int, volume float64) {
	pcm := p.pcm[id]
	if len(pcm) == 0 {
		return
	}
	if volume < 0 {
		volume = 0
	}
	player := p.context.NewPlayerFromBytes(pcm)
	player.SetBufferSize(30 * time.Millisecond)
	player.SetVolume(volume)
	player.Play()

	p.mu.Lock()
	now := time.Now()
	kept := p.active[:0]
	for _, active := range p.active {
		if active.player.IsPlaying() || now.Before(active.keepUntil) {
			kept = append(kept, active)
			continue
		}
		_ = active.player.Close()
	}
	p.active = kept
	if len(p.active) >= maxActiveSounds {
		_ = p.active[0].player.Close()
		copy(p.active, p.active[1:])
		p.active = p.active[:len(p.active)-1]
	}
	duration := time.Duration(len(pcm)/4) * time.Second / time.Duration(p.sampleRate)
	if duration < minSoundLifetime {
		duration = minSoundLifetime
	}
	p.active = append(p.active, activeSound{player: player, keepUntil: now.Add(duration + 500*time.Millisecond)})
	p.mu.Unlock()
}

func (p *soundPlayer) Update() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.active) == 0 {
		return
	}
	now := time.Now()
	kept := p.active[:0]
	for _, active := range p.active {
		if active.player.IsPlaying() || now.Before(active.keepUntil) {
			kept = append(kept, active)
			continue
		}
		_ = active.player.Close()
	}
	p.active = kept
}
