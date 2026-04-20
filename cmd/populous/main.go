package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous/internal/assets"
	"go-populous/internal/game"
)

func main() {
	bundle, err := assets.Load()
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowTitle("Populous")
	ebiten.SetWindowSize(960, 720)
	ebiten.SetTPS(8)
	if err := ebiten.RunGame(game.New(bundle)); err != nil {
		log.Fatal(err)
	}
}
