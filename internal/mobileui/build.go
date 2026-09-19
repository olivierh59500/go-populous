package mobileui

import "go-populous/internal/populous"

// CanBuild checks local construction presence without spending mana or
// modifying the world. The caller picks a visible terrain vertex and the
// ordinary terrain command still enforces mana, altitude, and raising-only
// restrictions. Vertices include the far map edges, unlike occupied tiles.
func CanBuild(w *populous.World, player, targetX, targetY int, scope BuildScope, visible func(mapX, mapY int) bool) bool {
	if w == nil || player < 0 || player >= len(w.Magnets) ||
		targetX < 0 || targetX > populous.MapWidth || targetY < 0 || targetY > populous.MapHeight ||
		w.Level.GameMode&populous.GameNoBuild != 0 {
		return false
	}
	switch scope {
	case BuildScopeClassic:
		// A complete original-size neighbourhood is centred near the target,
		// then clamped at map edges. Its size never depends on the screen.
		xoff := max(0, min(targetX-3, populous.MapWidth-8))
		yoff := max(0, min(targetY-3, populous.MapHeight-8))
		return w.HasBuildPresenceAt(player, xoff, yoff, 8, 8, targetX, targetY)
	case BuildScopeVisible:
		if visible == nil {
			return false
		}
		needTown := w.Level.GameMode&populous.GameRaiseTown != 0 && w.Alt[targetX+targetY*populous.EndWidth] > 0
		for pos, id := range w.MapWho {
			index := int(id) - 1
			if index < 0 || index >= len(w.Peeps) {
				continue
			}
			peep := w.Peeps[index]
			if peep.Population <= 0 || int(peep.Player) != player || needTown && peep.Flags&populous.InTown == 0 {
				continue
			}
			if visible(pos%populous.MapWidth, pos/populous.MapWidth) {
				return true
			}
		}
	}
	return false
}
