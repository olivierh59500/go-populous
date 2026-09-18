// Package mobile exposes Populous to ebitenmobile.
package mobile

import (
	"fmt"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"go-populous/internal/assets"
	"go-populous/internal/game"
)

var populousGame *game.Game

func init() {
	bundle, err := assets.LoadEmbedded()
	if err != nil {
		panic(fmt.Errorf("load embedded Populous assets: %w", err))
	}
	populousGame = game.New(bundle)
	populousGame.SetUpdateTPS(60)
	ebiten.SetTPS(60)
	enginemobile.SetGame(populousGame)
}

// SetFilesDir configures Android's private files directory for save games.
// MainActivity calls this before creating the Ebiten view.
func SetFilesDir(path string) {
	if populousGame == nil {
		return
	}
	if path == "" {
		populousGame.SetSavePath("")
		return
	}
	populousGame.SetSavePath(filepath.Join(path, "go-populous.sav"))
}

// Dummy forces gomobile to include this package in the Android binding.
func Dummy() {}
