package mobileui

import (
	"testing"

	"go-populous/internal/populous"
)

func buildWorld(x, y int) *populous.World {
	pos := x + y*populous.MapWidth
	w := &populous.World{Peeps: []populous.Peep{{Player: populous.GodPlayer, Population: 100, AtPos: pos, Flags: populous.OnMove}}}
	w.MapWho[pos] = 1
	w.Magnets[populous.GodPlayer].Mana = 12345
	return w
}

func TestBuildClassicIsIndependentOfViewport(t *testing.T) {
	w := buildWorld(20, 20)
	for _, visible := range []func(int, int) bool{nil, func(int, int) bool { return true }, func(int, int) bool { return false }} {
		if !CanBuild(w, populous.GodPlayer, 22, 22, BuildScopeClassic, visible) {
			t.Fatal("classic construction changed with viewport")
		}
		if CanBuild(w, populous.GodPlayer, 45, 45, BuildScopeClassic, visible) {
			t.Fatal("classic construction extended to distant target")
		}
	}
}

func TestBuildVisibleExtensionIsOptIn(t *testing.T) {
	w := buildWorld(20, 20)
	visible := func(x, y int) bool { return x >= 15 && x <= 50 && y >= 15 && y <= 50 }
	if CanBuild(w, populous.GodPlayer, 45, 45, BuildScopeClassic, visible) {
		t.Fatal("wide view extended classic scope")
	}
	if !CanBuild(w, populous.GodPlayer, 45, 45, BuildScopeVisible, visible) {
		t.Fatal("visible allied group did not permit visible-scope construction")
	}
	if CanBuild(w, populous.GodPlayer, 45, 45, BuildScopeVisible, func(x, y int) bool { return x > 30 }) {
		t.Fatal("offscreen allied group permitted construction")
	}
	if CanBuild(w, populous.GodPlayer, 45, 45, BuildScopeVisible, nil) {
		t.Fatal("visible scope accepted a missing visibility test")
	}
}

func TestBuildPresenceValidatesBothScopes(t *testing.T) {
	for _, scope := range []BuildScope{BuildScopeClassic, BuildScopeVisible} {
		w := buildWorld(20, 20)
		canBuild := func() bool {
			return CanBuild(w, populous.GodPlayer, 22, 22, scope, func(int, int) bool { return true })
		}
		w.Level.GameMode = populous.GameRaiseTown
		if canBuild() {
			t.Fatalf("scope %d: wandering group passed towns-only restriction", scope)
		}
		w.Peeps[0].Flags = populous.InTown | populous.InBattle
		if !canBuild() {
			t.Fatalf("scope %d: inhabited town not recognized", scope)
		}
		w.Peeps[0].Population = 0
		if canBuild() {
			t.Fatalf("scope %d: dead group permitted construction", scope)
		}
		w.Peeps[0].Population = 100
		w.Peeps[0].Player = populous.DevilPlayer
		if canBuild() {
			t.Fatalf("scope %d: enemy permitted construction", scope)
		}
		w.Peeps[0].Player = populous.GodPlayer
		w.MapWho[20+20*populous.MapWidth] = 200
		if canBuild() {
			t.Fatalf("scope %d: invalid MapWho permitted construction", scope)
		}
	}
}

func TestBuildScopeBoundariesAndRestrictions(t *testing.T) {
	for _, scope := range []BuildScope{BuildScopeClassic, BuildScopeVisible} {
		visible := func(int, int) bool { return true }
		for _, corner := range [][2]int{{0, 0}, {64, 0}, {0, 64}, {64, 64}} {
			// The complete 8x8 reference region remains available at the edge.
			x, y := 7, 7
			if corner[0] == 64 {
				x = 56
			}
			if corner[1] == 64 {
				y = 56
			}
			w := buildWorld(x, y)
			if !CanBuild(w, populous.GodPlayer, corner[0], corner[1], scope, visible) {
				t.Fatalf("scope %d: valid corner %v rejected", scope, corner)
			}
		}
		w := buildWorld(20, 20)
		for _, target := range [][2]int{{-1, 22}, {65, 22}, {22, -1}, {22, 65}} {
			if CanBuild(w, populous.GodPlayer, target[0], target[1], scope, visible) {
				t.Fatalf("scope %d: invalid target %v accepted", scope, target)
			}
		}
		for _, player := range []int{-1, 2, 255} {
			if CanBuild(w, player, 22, 22, scope, visible) {
				t.Fatalf("scope %d: invalid player %d accepted", scope, player)
			}
		}
		w.Level.GameMode = populous.GameNoBuild | populous.GameOnlyRaise
		if CanBuild(w, populous.GodPlayer, 22, 22, scope, visible) {
			t.Fatalf("scope %d: no-build restriction ignored", scope)
		}
		if CanBuild(nil, populous.GodPlayer, 22, 22, scope, visible) {
			t.Fatalf("scope %d: nil world accepted", scope)
		}
	}
	if CanBuild(buildWorld(20, 20), populous.GodPlayer, 22, 22, BuildScope(12), nil) {
		t.Fatal("invalid construction scope accepted")
	}
}

func TestBuildCheckPreservesWorldAndMana(t *testing.T) {
	w := buildWorld(20, 20)
	// OnlyRaise is enforced by the eventual command, not the presence query.
	w.Level.GameMode = populous.GameOnlyRaise
	before := w.StateHash()
	for _, scope := range []BuildScope{BuildScopeClassic, BuildScopeVisible} {
		if !CanBuild(w, populous.GodPlayer, 22, 22, scope, func(int, int) bool { return true }) {
			t.Fatalf("scope %d: raising was prevented", scope)
		}
	}
	if w.StateHash() != before || w.Magnets[populous.GodPlayer].Mana != 12345 {
		t.Fatal("presence check mutated shared world or spent mana")
	}
}
