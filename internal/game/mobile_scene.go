package game

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous/internal/mobileui"
	"go-populous/internal/populous"
)

// drawMobileScene uses the original artwork but has no dependency on the
// desktop frame's coordinates or 8-by-8 window. SubImage clips all primitives,
// including high terrain, to the playing area beneath the mobile controls.
func (g *Game) drawMobileScene(screen *ebiten.Image, view mobileui.Viewport) {
	if g.world == nil || view.Rect.Empty() {
		return
	}
	land := g.world.Terrain
	if land < 0 || land >= len(g.landFrames) {
		return
	}
	clip := view.Rect.Intersect(screen.Bounds())
	if clip.Empty() {
		return
	}
	dst := screen.SubImage(clip).(*ebiten.Image)
	bounds := view.Bounds(populous.MapWidth, populous.MapHeight, 8)
	zoom := mobileViewScale(view)
	for diagonal := bounds.Min.X + bounds.Min.Y; diagonal <= bounds.Max.X+bounds.Max.Y-2; diagonal++ {
		for x := max(bounds.Min.X, diagonal-bounds.Max.Y+1); x <= min(bounds.Max.X-1, diagonal-bounds.Min.Y); x++ {
			y := diagonal - x
			pos := x + y*populous.MapWidth
			height := int(g.world.MapAlt[pos])
			px, py := view.Project(float64(x), float64(y), float64(height))
			// Include walking sprites and raised overlays in the culling margin.
			if px+32*zoom < float64(clip.Min.X) || px-32*zoom >= float64(clip.Max.X) ||
				py+24*zoom < float64(clip.Min.Y) || py-32*zoom >= float64(clip.Max.Y) {
				continue
			}
			g.drawMobileMapWalls(dst, view, x, y, height)
			block := int(g.world.MapBlk[pos])
			if block == populous.WaterBlock && g.tick%2 == 0 {
				block = 16
			}
			g.drawMobileBlock(dst, land, block, px, py, zoom)
			if overlay := int(g.world.MapBk2[pos]); overlay != 0 {
				g.drawMobileBlock(dst, land, overlay, px, py-8*zoom, zoom)
			}
			devilTarget, godTarget := g.magnetTargetsAt(pos)
			if devilTarget {
				g.drawMobileBlock(dst, land, populous.DevilsMagnetBlock, px, py-8*zoom, zoom)
			}
			if godTarget {
				g.drawMobileBlock(dst, land, populous.GodsMagnetBlock, px, py-8*zoom, zoom)
			}
		}
	}
	// MapWho anchors a walking sprite to its departure tile throughout the
	// animation. Its feet can already overlap the following tiles. As on the
	// desktop, finish all ground/objects before drawing people so those tiles
	// cannot paint over the moving sprite or its carried markers.
	g.drawMobilePeople(dst, view, bounds)
	if g.hoverOK && g.hoverX >= 0 && g.hoverX < populous.MapWidth && g.hoverY >= 0 && g.hoverY < populous.MapHeight {
		mode := populous.CursorDefault
		switch g.mode {
		case ModeMagnet:
			mode = populous.CursorMagnet
		case ModeSwamp:
			mode = populous.CursorSwamp
		}
		pos := g.hoverX + g.hoverY*populous.MapWidth
		x, y := view.Project(float64(g.hoverX), float64(g.hoverY), float64(g.world.MapAlt[pos]))
		g.drawMobileSprite(dst, populous.CursorSprite(mode, g.player), x-8*zoom, y-8*zoom, zoom)
	}
}

func (g *Game) drawMobilePeople(screen *ebiten.Image, view mobileui.Viewport, bounds image.Rectangle) {
	if len(g.spriteFrames) == 0 {
		return
	}
	zoom := mobileViewScale(view)
	for diagonal := bounds.Min.X + bounds.Min.Y; diagonal <= bounds.Max.X+bounds.Max.Y-2; diagonal++ {
		for x := max(bounds.Min.X, diagonal-bounds.Max.Y+1); x <= min(bounds.Max.X-1, diagonal-bounds.Min.Y); x++ {
			y := diagonal - x
			pos := x + y*populous.MapWidth
			id := int(g.world.MapWho[pos])
			if id <= 0 || id > len(g.world.Peeps) || g.world.Peeps[id-1].Population <= 0 {
				continue
			}
			peep := g.world.Peeps[id-1]
			frame, dx, dy := g.peepSpriteFrame(pos, peep)
			px, py := view.Project(float64(x), float64(y), float64(g.world.MapAlt[pos]))
			sx, sy := px+float64(dx-8)*zoom, py+float64(dy-8)*zoom
			g.drawMobileSprite(screen, frame, sx, sy, zoom)
			if id-1 == g.viewPeep {
				g.drawMobileSprite(screen, populous.ShieldSprite, sx+8*zoom, sy, zoom)
			}
			if frame, ok := g.carriedMagnetFrame(id, peep); ok {
				g.drawMobileSprite(screen, frame, sx+8*zoom, sy, zoom)
			}
		}
	}
}

func mobileViewScale(view mobileui.Viewport) float64 {
	if view.Zoom <= 0 || math.IsNaN(view.Zoom) || math.IsInf(view.Zoom, 0) {
		return 1
	}
	return view.Zoom
}

func (g *Game) drawMobileBlock(screen *ebiten.Image, land, block int, centerX, centerY, zoom float64) {
	if block < 0 || block >= len(g.landFrames[land]) {
		return
	}
	drawMobileFrame(screen, g.landFrames[land][block], centerX-16*zoom, centerY-8*zoom, zoom)
}

func (g *Game) drawMobileSprite(screen *ebiten.Image, frame int, x, y, zoom float64) {
	if frame < 0 || frame >= len(g.spriteFrames) {
		return
	}
	drawMobileFrame(screen, g.spriteFrames[frame], x, y, zoom)
}

func drawMobileFrame(screen, frame *ebiten.Image, x, y, zoom float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(math.Round(x), math.Round(y))
	screen.DrawImage(frame, op)
}

func (g *Game) drawMobileMapWalls(screen *ebiten.Image, view mobileui.Viewport, x, y, height int) {
	zoom := mobileViewScale(view)
	for altitude := height; altitude > 0; altitude-- {
		if x == populous.MapWidth-1 {
			px, py := view.Project(float64(x+1), float64(y), float64(altitude-1))
			g.drawMobileSprite(screen, populous.SideWall1, px-16*zoom, py-8*zoom, zoom)
		}
		if y == populous.MapHeight-1 {
			px, py := view.Project(float64(x+1), float64(y+1), float64(altitude))
			g.drawMobileSprite(screen, populous.SideWall2, px-16*zoom, py-8*zoom, zoom)
		}
	}
}

// mobileVisiblePeepAt matches the mobile drawing coordinates for inspection.
func (g *Game) mobileVisiblePeepAt(view mobileui.Viewport, screenX, screenY int) (int, bool) {
	if g.world == nil || screenX < view.Rect.Min.X || screenX >= view.Rect.Max.X || screenY < view.Rect.Min.Y || screenY >= view.Rect.Max.Y {
		return 0, false
	}
	bounds := view.Bounds(populous.MapWidth, populous.MapHeight, 8)
	zoom := mobileViewScale(view)
	for diagonal := bounds.Max.X + bounds.Max.Y - 2; diagonal >= bounds.Min.X+bounds.Min.Y; diagonal-- {
		for x := min(bounds.Max.X-1, diagonal-bounds.Min.Y); x >= max(bounds.Min.X, diagonal-bounds.Max.Y+1); x-- {
			y := diagonal - x
			pos := x + y*populous.MapWidth
			id := int(g.world.MapWho[pos])
			if id <= 0 || id > len(g.world.Peeps) || g.world.Peeps[id-1].Population <= 0 {
				continue
			}
			_, dx, dy := g.peepSpriteFrame(pos, g.world.Peeps[id-1])
			px, py := view.Project(float64(x), float64(y), float64(g.world.MapAlt[pos]))
			px += float64(dx-8) * zoom
			py += float64(dy-8) * zoom
			if float64(screenX) >= px && float64(screenX) < px+16*zoom && float64(screenY) >= py && float64(screenY) < py+16*zoom {
				return id - 1, true
			}
		}
	}
	return 0, false
}
