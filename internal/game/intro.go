package game

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type introPresentation struct {
	duration    time.Duration
	ticks       int
	updateRate  int
	waitRelease bool
	keys        []ebiten.Key
	touches     []ebiten.TouchID
}

// SetIntroDuration shows the original title illustration before the menu.
// Platforms opt in before RunGame; desktop and demo playback are unchanged.
// A non-positive duration disables the introduction.
func (g *Game) SetIntroDuration(duration time.Duration) {
	if g.state != StateTitle && g.state != StateIntro {
		return
	}
	if duration <= 0 {
		g.intro = nil
		if g.state == StateIntro {
			g.state = StateTitle
		}
		return
	}
	g.intro = &introPresentation{duration: duration}
	g.state = StateIntro
	g.frameCacheValid = false
	if g.demoIdle != nil {
		g.demoIdle.Reset()
	}
}

// updateIntro runs at the frontend input rate, before any menu samples input.
// A skip opens the menu immediately but swallows the whole contact/key press,
// including its release, so it cannot also choose the item underneath it.
func (g *Game) updateIntro() bool {
	i := g.intro
	if i == nil || g.state != StateIntro && !i.waitRelease {
		return false
	}
	i.keys = inpututil.AppendPressedKeys(i.keys[:0])
	i.touches = ebiten.AppendTouchIDs(i.touches[:0])
	held := len(i.keys) > 0 || len(i.touches) > 0
	for _, button := range [...]ebiten.MouseButton{ebiten.MouseButtonLeft, ebiten.MouseButtonRight, ebiten.MouseButtonMiddle} {
		held = held || ebiten.IsMouseButtonPressed(button)
	}
	if i.waitRelease {
		if !held {
			g.clearIntroInput()
			g.intro = nil
		}
		return true
	}
	if i.updateRate == 0 {
		i.updateRate = max(1, ebiten.TPS())
	}
	i.ticks++
	if !held && time.Duration(i.ticks)*time.Second/time.Duration(i.updateRate) < i.duration {
		return true
	}
	g.state = StateTitle
	g.frameCacheValid = false
	g.clearIntroInput()
	if held {
		i.waitRelease = true
	} else {
		g.intro = nil
	}
	return true
}

func (g *Game) clearIntroInput() {
	g.cancelMobileInput()
	g.touch = touchInputState{}
	g.demoInput = demoInputState{}
	if g.demoIdle != nil {
		g.demoIdle.Reset()
	}
}

func (g *Game) drawIntro(screen *ebiten.Image) {
	screen.Fill(color.Black)
	art := g.images["load"]
	if art == nil {
		art = g.images["demo"]
	}
	if art == nil {
		return
	}
	bounds := screen.Bounds()
	w, h := float64(art.Bounds().Dx()), float64(art.Bounds().Dy())
	scale := math.Min(float64(bounds.Dx())/w, float64(bounds.Dy())/h)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((float64(bounds.Dx())-w*scale)/2, (float64(bounds.Dy())-h*scale)/2)
	screen.DrawImage(art, op)
}
