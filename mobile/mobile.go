// Package mobile exposes Populous to ebitenmobile.
package mobile

import (
	"fmt"
	"path/filepath"
	"time"

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
	populousGame.SetMobileUI(true)
	populousGame.SetUpdateTPS(60)
	populousGame.SetIntroDuration(3 * time.Second)
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

// SetAudioCapturePath enables a source-audio trace before the view starts.
// Android exposes this only in development builds; normal launches do not record.
func SetAudioCapturePath(path string) error {
	return populousGame.SetAudioTracePath(path)
}

// SetBluetoothAvailable tells the touch frontend whether this Android device
// exposes a Bluetooth Classic adapter.
func SetBluetoothAvailable(available bool) {
	if populousGame != nil {
		populousGame.SetBluetoothAvailable(available)
	}
}

// PollBluetoothCommand lets the Android Activity consume one pending
// Bluetooth transport command without ever blocking its UI thread.
func PollBluetoothCommand() string {
	if populousGame == nil {
		return ""
	}
	return populousGame.PollPlatformCommand()
}

// BluetoothStatus forwards platform progress to the Ebitengine update loop.
func BluetoothStatus(status string) {
	if populousGame != nil {
		populousGame.PostBluetoothStatus(status)
	}
}

// BluetoothClientReady reports the private TCP loopback endpoint created by
// Android after its RFCOMM client socket is connected.
func BluetoothClientReady(address, name string) {
	if populousGame != nil {
		populousGame.PostBluetoothReady(address, name)
	}
}

// BluetoothFailed forwards a permission, discovery or socket failure without
// mutating game state from Android's callback thread.
func BluetoothFailed(message string) {
	if populousGame != nil {
		populousGame.PostBluetoothFailure(message)
	}
}

// CancelBluetooth is safe for Activity lifecycle callbacks.
func CancelBluetooth() {
	if populousGame != nil {
		populousGame.CancelBluetooth()
	}
}

// CancelInput prevents an unfinished gesture becoming an action after resume.
func CancelInput() { populousGame.CancelInput() }

// Dummy forces gomobile to include this package in the Android binding.
func Dummy() {}
