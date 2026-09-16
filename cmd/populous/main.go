package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous/internal/assets"
	"go-populous/internal/game"
)

func main() {
	listenAddress := flag.String("listen", "", "listen for one multiplayer peer (for example :7777)")
	joinAddress := flag.String("join", "", "join a multiplayer host (for example 192.0.2.10:7777)")
	playerName := flag.String("name", "", "player name sent when joining a multiplayer host")
	levelIndex := flag.Int("world", 0, "world index hosted in multiplayer")
	flag.Parse()
	if *listenAddress != "" && *joinAddress != "" {
		log.Fatal("-listen and -join are mutually exclusive")
	}

	bundle, err := assets.Load()
	if err != nil {
		log.Fatal(err)
	}
	populousGame := game.New(bundle)
	if *listenAddress != "" || *joinAddress != "" {
		if err := populousGame.StartMultiplayer(game.NetworkConfig{
			ListenAddress: *listenAddress,
			JoinAddress:   *joinAddress,
			PlayerName:    *playerName,
			LevelIndex:    *levelIndex,
		}); err != nil {
			log.Fatal(err)
		}
	}
	defer populousGame.Close()

	ebiten.SetWindowTitle("Populous")
	ebiten.SetWindowSize(960, 720)
	ebiten.SetTPS(8)
	if err := ebiten.RunGame(populousGame); err != nil {
		log.Fatal(err)
	}
}
