//go:build rendercheck

// Command rendercheck runs GPU-backed mobile sprite regression checks.
package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"go-populous/internal/assets"
	"go-populous/internal/game"
)

func main() {
	screenshot := flag.String("screenshot", "", "optional PNG: actual rendering left, independent reference right")
	flag.Parse()
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowTitle("Populous mobile render regression check")
	ebiten.SetWindowSize(640, 400)
	ebiten.SetVsyncEnabled(false)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	if err := ebiten.RunGame(game.NewMobileRenderCheck(bundle, *screenshot)); err != nil {
		log.Fatal(err)
	}
}
