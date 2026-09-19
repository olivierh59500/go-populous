//go:build frontmenucheck

package game

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"go-populous/internal/assets"
	"go-populous/internal/mobileui"
	"go-populous/internal/touchui"
)

// FrontMenuCheck runs opt-in integration checks in a real Ebitengine loop.
// All persistence goes to a fresh temporary directory, never the user's save.
type FrontMenuCheck struct {
	g              *Game
	frames, checks int
	err            error
	done           bool
	started        bool
	dir            string
}

func NewFrontMenuCheck(bundle *assets.Bundle) (*FrontMenuCheck, error) {
	dir, err := os.MkdirTemp("", "populous-front-check-")
	if err != nil {
		return nil, err
	}
	g := New(bundle)
	g.SetSavePath(filepath.Join(dir, "check.sav"))
	g.SetMobileUI(true)
	g.SetUpdateTPS(60)
	g.audioInitialized = true
	g.SetDemoSeed(1989)
	g.Layout(2424, 1080)
	return &FrontMenuCheck{g: g, dir: dir}, nil
}

func (c *FrontMenuCheck) Layout(_, _ int) (int, int) { return 539, 240 }

func (c *FrontMenuCheck) require(ok bool, description string) {
	c.checks++
	if !ok && c.err == nil {
		c.err = fmt.Errorf("front menu check %d: %s", c.checks, description)
	}
}

func (c *FrontMenuCheck) tap(action mobileui.FrontAction, index int) {
	g := c.g
	g.syncMobilePresentation()
	g.touch.contacts, g.touch.justContactIDs = nil, nil
	g.updateMobileFrontInput()
	for _, b := range g.mobileFrontLayout().Buttons {
		if b.Action != action || b.Index != index {
			continue
		}
		x, y := (b.Rect.Min.X+b.Rect.Max.X)/2, (b.Rect.Min.Y+b.Rect.Max.Y)/2
		oldState, oldWorld, oldPage := g.state, g.world, g.mobileFrontPage()
		g.touch.contacts = []touchui.Contact{{ID: 101, X: x, Y: y}}
		g.touch.justContactIDs = []int{101}
		g.updateMobileFrontInput()
		c.require(g.state == oldState && g.world == oldWorld && g.mobileFrontPage() == oldPage, fmt.Sprintf("action %d/%d activated on finger down", action, index))
		g.touch.contacts, g.touch.justContactIDs = nil, nil
		g.updateMobileFrontInput()
		return
	}
	c.require(false, fmt.Sprintf("action %d/%d has no visible hit target on page %d", action, index, g.mobileFrontPage()))
}

func (c *FrontMenuCheck) home() {
	g := c.g
	if g.state == StateDemo {
		g.stopDemo()
	}
	g.openMobileFront(mobileui.FrontHome)
	g.Layout(2424, 1080)
}

func (c *FrontMenuCheck) checkInput() {
	g := c.g
	for index, choice := range []struct {
		action mobileui.FrontAction
		page   mobileui.FrontPage
	}{
		{mobileui.FrontActionTutorial, mobileui.FrontHome},
		{mobileui.FrontActionConquest, mobileui.FrontConquest},
		{mobileui.FrontActionCustom, mobileui.FrontOptions},
		{mobileui.FrontActionSetup, mobileui.FrontSetup},
		{mobileui.FrontActionPreferences, mobileui.FrontPreferences},
		{mobileui.FrontActionLoad, mobileui.FrontHome},
		{mobileui.FrontActionHelp, mobileui.FrontHelp},
		{mobileui.FrontActionDemo, mobileui.FrontHome},
	} {
		c.home()
		c.tap(choice.action, index)
		switch choice.action {
		case mobileui.FrontActionTutorial:
			c.require(g.state == StateGame && g.tutorialActive && g.tutorialPaused, "tutorial starts paused")
			before := g.world.StateHash()
			g.syncMobilePresentation()
			g.updateMobileInput()
			c.require(g.tutorialPaused && len(g.mobileUI.pending) == 0 && before == g.world.StateHash(), "tutorial release leaked to terrain or dismissed instructions")
		case mobileui.FrontActionLoad:
			c.require(g.state == StateTitle && g.mobileUI.status == "NO SAVED GAME YET", "missing save reported on home")
		case mobileui.FrontActionDemo:
			c.require(g.state == StateDemo && g.demo.session != nil, "manual demo starts")
		default:
			c.require(g.mobileFrontPage() == choice.page, "home choice reaches correct page")
		}
	}
	c.home()
	c.tap(mobileui.FrontActionConquest, 1)
	g.setLevel(20)
	for _, v := range []struct {
		action     mobileui.FrontAction
		step, want int
	}{
		{mobileui.FrontActionPrevWorld, 1, 19}, {mobileui.FrontActionNextWorld, 1, 20},
		{mobileui.FrontActionPrevWorld, 10, 10}, {mobileui.FrontActionNextWorld, 10, 20},
	} {
		c.tap(v.action, v.step)
		c.require(g.levelIndex == v.want, "conquest world navigation")
	}
	c.tap(mobileui.FrontActionStart, 0)
	c.require(g.state == StateGame && !g.tutorialActive && g.levelIndex == 20, "conquest starts selected world")
	c.home()
	c.tap(mobileui.FrontActionCustom, 2)
	seen := make(map[int]bool)
	for {
		layout := g.mobileFrontLayout()
		for _, row := range layout.Rows {
			seen[row.Index] = true
			if row.Index < optionRowFirstPower {
				before := g.bundle.Levels[g.levelIndex]
				c.tap(mobileui.FrontActionIncrease, row.Index)
				c.require(g.bundle.Levels[g.levelIndex] != before || row.Index == optionRowReaction || row.Index == optionRowRating, "custom + changes level")
				c.tap(mobileui.FrontActionDecrease, row.Index)
			} else {
				before := g.bundle.Levels[g.levelIndex]
				bit := byte(1 << (row.Index - optionRowFirstPower))
				c.tap(mobileui.FrontActionTogglePlayerPower, row.Index)
				now := g.bundle.Levels[g.levelIndex]
				c.require(now.PlayerPowers == before.PlayerPowers^bit && now.EnemyPowers == before.EnemyPowers, "YOU power independent from AI")
				c.tap(mobileui.FrontActionToggleEnemyPower, row.Index)
				now = g.bundle.Levels[g.levelIndex]
				c.require(now.PlayerPowers == before.PlayerPowers^bit && now.EnemyPowers == before.EnemyPowers^bit, "AI power independent from YOU")
			}
		}
		if layout.PageIndex+1 == layout.PageCount {
			break
		}
		c.tap(mobileui.FrontActionNextPage, 0)
	}
	c.require(len(seen) == optionRowCount, "every custom row reachable")
	c.tap(mobileui.FrontActionStart, 0)
	c.require(g.state == StateGame, "custom starts")
	c.home()
	c.tap(mobileui.FrontActionSetup, 3)
	seen = make(map[int]bool)
	for {
		layout := g.mobileFrontLayout()
		for _, b := range layout.Buttons {
			if b.Action == mobileui.FrontActionSetupItem {
				seen[b.Index] = true
			}
		}
		if layout.PageIndex+1 == layout.PageCount {
			break
		}
		c.tap(mobileui.FrontActionNextPage, 0)
	}
	c.require(len(seen) == len(setupItems), "all 14 setup items reachable")
	c.tap(mobileui.FrontActionBack, 0)
	c.require(g.state == StateTitle, "setup back to home")
	c.tap(mobileui.FrontActionSetup, 3)
	c.tap(mobileui.FrontActionNextPage, 0)
	c.tap(mobileui.FrontActionSetupItem, setupSave)
	saved := g.world.StateHash()
	c.require(g.setupMessage == "SAVED TO "+saveFileName, "setup saves to private directory")
	c.tap(mobileui.FrontActionBack, 0)
	g.setLevel(45)
	c.tap(mobileui.FrontActionLoad, 5)
	c.require(g.state == StateGame && g.world.StateHash() == saved, "home loads the saved world")
	c.home()
	c.tap(mobileui.FrontActionPreferences, 4)
	before := g.mobileUI.prefs
	c.tap(mobileui.FrontActionToggleWide, 0)
	c.tap(mobileui.FrontActionTogglePad, 1)
	c.tap(mobileui.FrontActionToggleScope, 2)
	c.require(g.mobileUI.prefs.Wide != before.Wide && g.mobileUI.prefs.ShowPad != before.ShowPad && g.mobileUI.prefs.BuildScope != before.BuildScope, "all three preferences change")
	prefs, err := mobileui.LoadPreferences(g.mobilePreferencesPath())
	c.require(err == nil && prefs == g.mobileUI.prefs, "preferences saved in isolated directory")
	c.tap(mobileui.FrontActionBack, 0)
	c.tap(mobileui.FrontActionHelp, 6)
	for g.mobileFrontLayout().PageIndex+1 < g.mobileFrontLayout().PageCount {
		c.tap(mobileui.FrontActionNextPage, 0)
	}
	c.tap(mobileui.FrontActionBack, 0)
	c.require(g.state == StateTitle, "help back to home")
	g.state = StateGame
	g.openHelp(StateGame)
	c.tap(mobileui.FrontActionBack, 0)
	c.require(g.state == StateGame, "in-game help returns to game")
	c.home()
	g.SetDemoDelay(30 * time.Second)
	for range 239 {
		g.updateDemo()
	}
	c.require(g.state == StateTitle, "idle demo not early")
	g.updateDemo()
	c.require(g.state == StateDemo, "idle demo starts at 240 simulation ticks")
	c.home()
	for _, page := range []mobileui.FrontPage{mobileui.FrontSetup, mobileui.FrontOptions, mobileui.FrontHelp, mobileui.FrontConquest, mobileui.FrontPreferences} {
		g.openMobileFront(page)
		for range 250 {
			g.updateDemo()
		}
		c.require(g.state != StateDemo, "secondary menus never idle into demo")
	}
	c.home()
	g.StartDemo()
	g.syncMobilePresentation()
	g.demoInput.dismiss = true
	g.updateDemo()
	g.syncMobilePresentation()
	// The dismissing finger is still held exactly above TUTORIAL.
	b := g.mobileFrontLayout().Buttons[0]
	g.touch.contacts = []touchui.Contact{{ID: 51, X: b.Rect.Min.X + 12, Y: b.Rect.Min.Y + 12}}
	g.touch.justContactIDs = nil
	g.updateMobileFrontInput()
	g.touch.contacts = nil
	g.updateMobileFrontInput()
	c.require(g.state == StateTitle, "demo dismiss contact cannot activate home on release")
	c.home()
	// One actual scheduler run confirms the frontend can remain at 60 Hz.
	g.SetDemoDelay(0)
	beforeTick := g.tick
	for range 60 {
		if err := g.Update(); err != nil {
			c.require(false, err.Error())
		}
	}
	c.require(g.tick-beforeTick == 8, "60 frontend updates retain 8 simulation ticks")
	g.SetDemoDelay(30 * time.Second)
}

func (c *FrontMenuCheck) Update() error {
	c.frames++
	if c.done {
		if c.err != nil {
			return c.err
		}
		fmt.Printf("front menu check: %d integration/GPU assertions passed; settings isolated in %s\n", c.checks, c.dir)
		return ebiten.Termination
	}
	if !c.started && c.frames >= 20 && ebiten.IsFocused() {
		c.started = true
		c.checkInput()
	}
	if !c.started && c.frames >= 300 {
		return fmt.Errorf("front menu check requires an unlocked desktop and a focused test window")
	}
	return nil
}

func (c *FrontMenuCheck) Draw(screen *ebiten.Image) {
	if !c.started || c.done {
		return
	}
	g := c.g
	for _, capture := range []struct {
		page mobileui.FrontPage
		name string
	}{
		{mobileui.FrontHome, "home"}, {mobileui.FrontConquest, "conquest"},
		{mobileui.FrontOptions, "options"}, {mobileui.FrontPreferences, "settings"},
		{mobileui.FrontHelp, "help"},
	} {
		g.openMobileFront(capture.page)
		g.Layout(2424, 1080)
		dst := ebiten.NewImage(539, 240)
		g.Draw(dst)
		f, err := os.Create("/private/tmp/populous-front-" + capture.name + ".png")
		if err == nil {
			err = png.Encode(f, dst)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		c.require(err == nil, "save screenshot "+capture.name)
		screen.DrawImage(dst, nil)
	}
	c.home()
	g.StartDemo()
	for range 5 {
		g.updateDemo()
	}
	w, h := g.Layout(2424, 1080)
	c.require(w == demoWidth && h == demoHeight, "mobile demo original dimensions")
	a, b := ebiten.NewImage(w, h), ebiten.NewImage(w, h)
	m := g.mobileUI
	before := g.demo.session.World.StateHash()
	g.Draw(a)
	g.mobileUI = nil
	w2, h2 := g.Layout(2424, 1080)
	g.demo.tick = -1 // Force the second draw through the renderer too.
	g.Draw(b)
	rawA, rawB := make([]byte, w*h*4), make([]byte, w*h*4)
	a.ReadPixels(rawA)
	b.ReadPixels(rawB)
	c.require(w == w2 && h == h2, "demo layout independent of mobile UI")
	c.require(bytes.Equal(rawA, rawB), "demo frames pixel-identical with and without mobile UI")
	c.require(before == g.demo.session.World.StateHash(), "demo draw does not change simulation")
	g.stopDemo()
	g.Layout(960, 720)
	legacy, actual := ebiten.NewImage(320, 240), ebiten.NewImage(320, 240)
	g.drawFrame(legacy)
	g.Draw(actual)
	legacyPixels, actualPixels := make([]byte, 320*240*4), make([]byte, 320*240*4)
	legacy.ReadPixels(legacyPixels)
	actual.ReadPixels(actualPixels)
	c.require(!g.mobileMenuActive() && bytes.Equal(legacyPixels, actualPixels), "desktop title preserves legacy renderer")
	g.mobileUI = m
	c.done = true
}
