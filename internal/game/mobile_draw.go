package game

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"go-populous/internal/mobileui"
	"go-populous/internal/populous"
)

func (g *Game) drawMobileGame(screen *ebiten.Image) {
	m := g.mobileUI
	layout, view := g.mobileLayout(), g.mobileViewport()
	if m.mapCache == nil || m.mapCache.Bounds() != screen.Bounds() {
		if m.mapCache != nil {
			m.mapCache.Deallocate()
		}
		m.mapCache = ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
		m.mapCacheTick = -1
	}
	if m.mapCacheTick != g.tick || m.mapCacheView != view {
		m.mapCache.Fill(color.RGBA{R: 15, G: 31, B: 47, A: 255})
		g.drawMobileScene(m.mapCache, view)
		m.mapCacheTick, m.mapCacheView = g.tick, view
	}
	screen.DrawImage(m.mapCache, nil)
	if g.world != nil {
		g.drawMobilePreview(screen, view)
		g.drawMobileMiniMap(screen, layout, view)
	}
	g.drawMobileHUD(screen, layout, g.mobileHUDState())
	if g.tutorialPaused {
		box := image.Rect((layout.Width-300)/2, 55, (layout.Width+300)/2, 165)
		drawMobileRoundedRect(screen, box, 6, mobileHUDPanel)
		for i, line := range []string{"TUTORIAL", "TWO FINGERS: DRAG THE MAP", "ONE FINGER: ACT ON RELEASE", "RAISE / LOWER SELECT THE TERRAIN TOOL", "POWER: CHOOSE THEN TARGET THE MAP", "TOUCH TO START"} {
			ebitenutil.DebugPrintAt(screen, line, box.Min.X+10, box.Min.Y+6+i*16)
		}
	}
	if g.state == StateEnd {
		if m.endPanel == nil {
			m.endPanel = ebiten.NewImage(logicalWidth, logicalHeight)
		}
		m.endPanel.Clear()
		g.drawEndScreen(m.endPanel)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64((layout.Width-logicalWidth)/2), 0)
		screen.DrawImage(m.endPanel, op)
	}
}

func (g *Game) drawMobileMiniMap(screen *ebiten.Image, layout mobileui.HUDLayout, view mobileui.Viewport) {
	if g.miniMap == nil {
		return
	}
	if g.miniMapDirty {
		g.rebuildMiniMap()
	}
	r := layout.MiniMapRect
	vector.DrawFilledRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), mobileHUDBackground, false)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(0.5, 0.5)
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	screen.DrawImage(g.miniMap, op)
	clip := screen.SubImage(r.Intersect(screen.Bounds())).(*ebiten.Image)
	points := [4][2]float64{}
	for i, p := range [4]image.Point{{view.Rect.Min.X, view.Rect.Min.Y}, {view.Rect.Max.X, view.Rect.Min.Y}, {view.Rect.Max.X, view.Rect.Max.Y}, {view.Rect.Min.X, view.Rect.Max.Y}} {
		x, y := view.Unproject(float64(p.X), float64(p.Y), 0)
		points[i] = [2]float64{float64(r.Min.X) + (64+x-y)/2, float64(r.Min.Y) + (x+y)/4}
	}
	for i, p := range points {
		q := points[(i+1)%4]
		vector.StrokeLine(clip, float32(p[0]), float32(p[1]), float32(q[0]), float32(q[1]), 0.75, mobileHUDAccent, false)
	}
}

func (g *Game) drawMobilePreview(screen *ebiten.Image, view mobileui.Viewport) {
	m := g.mobileUI
	if !m.hasPreview || m.overlay != mobileui.OverlayNone || !g.mobileLayout().IsTerrainPoint(int(m.preview.X), int(m.preview.Y)) {
		return
	}
	x, y, ok := view.PickTile(m.preview.X, m.preview.Y, populous.MapWidth, populous.MapHeight, func(x, y int) float64 { return float64(g.world.MapAlt[x+y*populous.MapWidth]) })
	if !ok {
		return
	}
	ink := color.Color(mobileHUDAccent)
	if m.tool == mobileui.ActionRaise || m.tool == mobileui.ActionLower {
		scope := m.prefs.BuildScope
		if g.multiplayerEnabled() {
			scope = mobileui.BuildScopeClassic
		}
		allowed := mobileui.CanBuild(g.world, g.player, x, y, scope, func(xx, yy int) bool { return g.mobileTileVisible(view, xx, yy) }) && g.world.Magnets[g.player].Mana >= populous.ManaPointCost
		if m.tool == mobileui.ActionLower && g.world.Level.GameMode&populous.GameOnlyRaise != 0 {
			allowed = false
		}
		if !g.paintMap && !allowed {
			ink = color.RGBA{R: 255, G: 100, B: 90, A: 255}
		}
	}
	if m.tool >= mobileui.ActionQuake && m.tool <= mobileui.ActionArmageddon && !g.mobilePowerEnabled(m.tool) {
		ink = color.RGBA{R: 255, G: 100, B: 90, A: 255}
	}
	clip := screen.SubImage(view.Rect.Intersect(screen.Bounds())).(*ebiten.Image)
	px, py := view.Project(float64(x), float64(y), float64(g.world.MapAlt[x+y*populous.MapWidth]))
	z := view.Zoom
	points := [4][2]float64{{px, py - 8*z}, {px + 16*z, py}, {px, py + 8*z}, {px - 16*z, py}}
	for i, p := range points {
		q := points[(i+1)%4]
		vector.StrokeLine(clip, float32(p[0]), float32(p[1]), float32(q[0]), float32(q[1]), 1.3, ink, true)
	}
	if m.tool != mobileui.ActionQuake && m.tool != mobileui.ActionVolcano {
		return
	}
	startX, startY := clampInt(x-4, 0, populous.MapWidth-9), clampInt(y-4, 0, populous.MapHeight-9)
	for i, p := range [4][2]float64{{float64(startX) - 0.5, float64(startY) - 0.5}, {float64(startX) + 8.5, float64(startY) - 0.5}, {float64(startX) + 8.5, float64(startY) + 8.5}, {float64(startX) - 0.5, float64(startY) + 8.5}} {
		xx, yy := clampInt(int(math.Round(p[0])), 0, populous.MapWidth-1), clampInt(int(math.Round(p[1])), 0, populous.MapHeight-1)
		points[i][0], points[i][1] = view.Project(p[0], p[1], float64(g.world.MapAlt[xx+yy*populous.MapWidth]))
	}
	for i, p := range points {
		q := points[(i+1)%4]
		vector.StrokeLine(clip, float32(p[0]), float32(p[1]), float32(q[0]), float32(q[1]), 1, mobileHUDAccent, true)
	}
}
