package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"go-populous/internal/assets"
	"go-populous/internal/game"
)

func main() {
	listenAddress := flag.String("listen", "", "listen for one multiplayer peer (for example :7777)")
	joinAddress := flag.String("join", "", "join a multiplayer host (for example 192.0.2.10:7777)")
	playerName := flag.String("name", "", "player name sent when joining a multiplayer host")
	levelIndex := flag.Int("world", 0, "world index hosted in multiplayer")
	demo := flag.Bool("demo", false, "start an AI-vs-AI spectator demo immediately")
	demoDelay := flag.Duration("demo-delay", 30*time.Second, "title inactivity before demo (0 disables automatic demos)")
	demoSeed := flag.Uint64("demo-seed", 0, "repeatable demo world order (0 chooses a fresh random seed)")
	demoWorld := flag.Int("demo-world", -1, "demo a specific original world index (-1 selects random worlds)")
	recordDemo := flag.String("record-demo", "", "export a complete demo to a new MP4 file, with application audio only")
	recordMax := flag.Duration("record-max-duration", time.Hour, "fail export if no winner by this simulated duration (0 means unlimited)")
	width := flag.Int("width", 960, "window and recorded video width")
	height := flag.Int("height", 720, "window and recorded video height")
	mobileUI := flag.Bool("mobile-ui", false, "preview the mobile interface (left click acts on release, right drag pans)")
	flag.Parse()
	if *listenAddress != "" && *joinAddress != "" {
		log.Fatal("-listen and -join are mutually exclusive")
	}
	if (*demo || *recordDemo != "") && (*listenAddress != "" || *joinAddress != "") {
		log.Fatal("-demo and -record-demo cannot be combined with -listen or -join")
	}
	if *width < 320 || *height < 240 {
		log.Fatal("window dimensions must be at least 320x240")
	}

	bundle, err := assets.Load()
	if err != nil {
		log.Fatal(err)
	}
	populousGame := game.New(bundle)
	populousGame.SetMobileUI(*mobileUI)
	populousGame.SetDemoDelay(*demoDelay)
	populousGame.SetDemoSeed(*demoSeed)
	if err := populousGame.SetDemoWorld(*demoWorld); err != nil {
		log.Fatal(err)
	}
	if *recordDemo != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := populousGame.StartDemoRecording(game.DemoRecordingConfig{
			Path: *recordDemo, Width: *width, Height: *height, MaxDuration: *recordMax, Context: ctx,
			Progress: func(status string) { log.Print(status) },
		}); err != nil {
			log.Fatal(err)
		}
		log.Printf("Rendering a complete demo to %s (%dx%d); audio comes only from the game", *recordDemo, *width, *height)
	} else if *demo {
		populousGame.StartDemo()
	}
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
	ebiten.SetWindowTitle("Populous")
	ebiten.SetWindowSize(*width, *height)
	ebiten.SetTPS(8)
	if *mobileUI {
		populousGame.SetUpdateTPS(60)
		ebiten.SetTPS(60)
	}
	if *recordDemo != "" {
		// Frame-gated capture preserves the normal 8 Hz video timeline while
		// exporting as fast as the renderer and encoder allow.
		ebiten.SetTPS(ebiten.SyncWithFPS)
		ebiten.SetVsyncEnabled(false)
		ebiten.SetRunnableOnUnfocused(true)
	}
	runErr := ebiten.RunGame(populousGame)
	closeErr := populousGame.Close()
	if runErr != nil {
		log.Fatal(runErr)
	}
	if closeErr != nil {
		log.Fatal(closeErr)
	}
	if *recordDemo != "" {
		log.Printf("Complete demo saved: %s", *recordDemo)
	}
}
