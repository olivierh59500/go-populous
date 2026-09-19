//go:build frontmenucheck

package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"go-populous/internal/assets"
	"go-populous/internal/game"
	"log"
)

func main() {
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		log.Fatal(err)
	}
	check, err := game.NewFrontMenuCheck(bundle)
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowTitle("Populous touch menu integration checks")
	ebiten.SetWindowSize(1078, 480)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(check); err != nil {
		log.Fatal(err)
	}
}
