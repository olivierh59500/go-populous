//go:build rendercheck

package game

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous/internal/assets"
	"go-populous/internal/mobileui"
	"go-populous/internal/populous"
)

// MobileRenderCheck exercises the real GPU renderer without starting the game,
// audio, networking, or persistence. It compares against terrain-only rendering
// followed by independently composited CPU sprites, as on the desktop renderer.
// Run with: go run -tags rendercheck ./cmd/rendercheck
type MobileRenderCheck struct {
	game       *Game
	bundle     *assets.Bundle
	cases      []mobileRenderCase
	actual     *ebiten.Image
	terrain    *ebiten.Image
	next       int
	failed     int
	firstError error
	screenshot string
}

type mobileRenderCase struct {
	name    string
	land    int
	surface int // 0: flat, 1: farm, 2: uphill
	peep    populous.Peep
	zoom    int
	edge    int // 1..4: clip each viewport edge; 5: outside the 8x8 limit
}

func NewMobileRenderCheck(bundle *assets.Bundle, screenshot string) *MobileRenderCheck {
	g := &Game{bundle: bundle, viewPeep: 0}
	for _, land := range bundle.Lands {
		g.landFrames = append(g.landFrames, verticalImageFrames(ebiten.NewImageFromImage(land), populous.BlockWidth, populous.BlockHeight))
	}
	g.spriteFrames = verticalImageFrames(ebiten.NewImageFromImage(bundle.Sprites), populous.SpriteWidth, populous.SpriteHeight)
	c := &MobileRenderCheck{game: g, bundle: bundle, screenshot: screenshot,
		actual: ebiten.NewImage(320, 200), terrain: ebiten.NewImage(320, 200)}
	base := populous.Peep{Flags: populous.OnMove, Population: 100, Direction: 1, Frame: 6}
	c.cases = append(c.cases, mobileRenderCase{name: "representative-farm-east", surface: 1, peep: base, zoom: 2})
	for land := range bundle.Lands {
		for surface := 0; surface < 3; surface++ {
			for _, direction := range []int{-65, -64, -63, 1, 65, 64, 63, -1} {
				for frame := 0; frame < 7; frame++ {
					peep := base
					peep.Direction, peep.Frame, peep.Player = direction, frame, byte(frame%2)
					c.cases = append(c.cases, mobileRenderCase{
						name: fmt.Sprintf("land%d/surface%d/direction%d/frame%d", land, surface, direction, frame),
						land: land, surface: surface, peep: peep, zoom: 1 + frame%2})
				}
			}
		}
		for player := byte(0); player < 2; player++ {
			for _, state := range []struct {
				name   string
				flags  byte
				frame  int
				knight bool
			}{
				{"stationary", populous.OnMove, 0, false},
				{"knight-stationary", populous.OnMove, 0, true},
				{"town", populous.InTown, 0, false},
				{"waiting", populous.IAmWaiting, populous.FirstWaitSprite, false},
				{"knight-waiting", populous.IAmWaiting, populous.FirstWaitSprite, true},
				{"water", populous.InWater, populous.FirstWaterSprite, false},
				{"knight-water", populous.InWater, populous.FirstWaterSprite, true},
				{"battle", populous.InBattle, populous.BattleFirstFrame, false},
				{"knight-battle", populous.InBattle, populous.BattleFirstFrame, true},
				{"victory", populous.InEffect, populous.VictorySprite, false},
				{"knight-victory", populous.InEffect, populous.VictorySprite, true},
				{"ruin", populous.InRuin, 0, false},
			} {
				peep := base
				peep.Flags, peep.Frame, peep.Direction, peep.Player = state.flags, state.frame, 0, player
				if state.knight {
					peep.Status = populous.KnightStatus
				}
				c.cases = append(c.cases, mobileRenderCase{name: fmt.Sprintf("land%d/player%d/%s", land, player, state.name), land: land, surface: 1, peep: peep, zoom: 2})
			}
		}
	}
	for edge := 1; edge <= 5; edge++ {
		c.cases = append(c.cases, mobileRenderCase{name: fmt.Sprintf("clipping%d", edge), peep: base, zoom: 2, edge: edge})
	}
	return c
}

func (c *MobileRenderCheck) Layout(_, _ int) (int, int) { return 320, 200 }

func (c *MobileRenderCheck) Update() error {
	if c.next < len(c.cases) {
		return nil
	}
	if c.firstError != nil {
		return fmt.Errorf("mobile render check: %d/%d failed; first: %w", c.failed, len(c.cases), c.firstError)
	}
	fmt.Printf("mobile render check: %d GPU/CPU comparisons passed; simulation state unchanged\n", len(c.cases))
	return ebiten.Termination
}

func (c *MobileRenderCheck) Draw(screen *ebiten.Image) {
	if c.next >= len(c.cases) {
		return
	}
	fixture := c.cases[c.next]
	actual, expected, err := c.check(fixture)
	if err == nil && c.next == 0 && c.screenshot != "" {
		err = saveRenderComparison(c.screenshot, actual, expected)
	}
	if err == nil && !bytes.Equal(actual.Pix, expected.Pix) {
		different := 0
		for i := 0; i < len(actual.Pix); i += 4 {
			if !bytes.Equal(actual.Pix[i:i+4], expected.Pix[i:i+4]) {
				different++
			}
		}
		err = fmt.Errorf("%d changed pixels", different)
	}
	if err != nil {
		c.failed++
		if c.firstError == nil {
			c.firstError = fmt.Errorf("%s: %w", fixture.name, err)
		}
	}
	screen.DrawImage(c.actual, nil)
	c.next++
}

func (c *MobileRenderCheck) check(fixture mobileRenderCase) (*image.RGBA, *image.RGBA, error) {
	const source = 32 + 32*populous.MapWidth
	w := &populous.World{Terrain: fixture.land}
	for pos := range w.MapAlt {
		w.MapAlt[pos], w.MapBlk[pos] = 2, populous.FlatBlock
	}
	if fixture.surface == 1 {
		w.MapBlk[source] = populous.FarmBlock + fixture.peep.Player
	} else if fixture.surface == 2 {
		w.MapBlk[source], w.MapAlt[source+fixture.peep.Direction] = 7, 3
	}
	peep := fixture.peep
	peep.AtPos = source + peep.Direction
	w.Peeps, w.MapWho[source] = []populous.Peep{peep}, 1
	w.Magnets[peep.Player].Carried = 1
	view := mobileui.Viewport{Rect: image.Rect(24, 16, 296, 184), CenterX: 32, CenterY: 32, Zoom: float64(fixture.zoom)}
	if fixture.edge > 0 && fixture.edge < 5 {
		px, py := view.Project(32, 32, 2)
		target := [][2]float64{{}, {24, 100}, {296, 100}, {160, 16}, {160, 184}}[fixture.edge]
		view = view.Pan(target[0]-px, target[1]-py)
	} else if fixture.edge == 5 {
		view.Limit = image.Rect(24, 24, 32, 32)
	}
	c.game.world = w
	before := w.StateHash()
	background := color.RGBA{12, 18, 24, 255}
	c.actual.Fill(background)
	c.game.drawMobileScene(c.actual, view)
	actual := image.NewRGBA(c.actual.Bounds())
	c.actual.ReadPixels(actual.Pix)
	if w.StateHash() != before {
		return actual, actual, fmt.Errorf("renderer mutated simulation state")
	}
	// Only use the production renderer for terrain. Sprite composition below
	// does not use its drawing, ordering, clipping, or positioning helpers.
	terrainWorld := *w
	terrainWorld.MapWho = [populous.MapWidth * populous.MapHeight]byte{}
	c.game.world = &terrainWorld
	c.terrain.Fill(background)
	c.game.drawMobileScene(c.terrain, view)
	c.game.world = w
	expected := image.NewRGBA(c.terrain.Bounds())
	c.terrain.ReadPixels(expected.Pix)
	if image.Pt(32, 32).In(view.Bounds(populous.MapWidth, populous.MapHeight, 8)) {
		frame, dx, dy := c.game.peepSpriteFrame(source, peep)
		px, py := view.Project(32, 32, 2)
		// Desktop sprite origin relative to the tile image is (8+dx, dy).
		x, y := px+float64(dx-8)*view.Zoom, py+float64(dy-8)*view.Zoom
		c.composeSprite(expected, view, frame, x, y)
		c.composeSprite(expected, view, populous.ShieldSprite, x+8*view.Zoom, y)
		c.composeSprite(expected, view, populous.MagnetSprite+int(peep.Player), x+8*view.Zoom, y)
	}
	return actual, expected, nil
}

func (c *MobileRenderCheck) composeSprite(dst *image.RGBA, view mobileui.Viewport, frame int, x, y float64) {
	origin := image.Pt(int(math.Round(x)), int(math.Round(y)))
	zoom := int(view.Zoom)
	clip := dst.SubImage(view.Rect.Intersect(dst.Bounds())).(*image.RGBA)
	for sy := 0; sy < populous.SpriteHeight; sy++ {
		for sx := 0; sx < populous.SpriteWidth; sx++ {
			pixel := c.bundle.Sprites.At(sx, frame*populous.SpriteHeight+sy)
			r := image.Rect(origin.X+sx*zoom, origin.Y+sy*zoom, origin.X+(sx+1)*zoom, origin.Y+(sy+1)*zoom)
			draw.Draw(clip, r, image.NewUniform(pixel), image.Point{}, draw.Over)
		}
	}
}

// Each comparison is actual on the left, reference on the right, enlarged 2x.
func saveRenderComparison(path string, actual, expected *image.RGBA) error {
	comparison := image.NewRGBA(image.Rect(0, 0, 1280, 400))
	for side, src := range []*image.RGBA{actual, expected} {
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				r := image.Rect(side*640+x*2, y*2, side*640+x*2+2, y*2+2)
				draw.Draw(comparison, r, image.NewUniform(src.At(x, y)), image.Point{}, draw.Src)
			}
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(file, comparison)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}
