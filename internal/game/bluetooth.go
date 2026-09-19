package game

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"go-populous/internal/mobileui"
	"go-populous/internal/platformbridge"
)

// Android owns Bluetooth Classic discovery, permissions and RFCOMM sockets.
// The game and the Android side communicate through a deliberately tiny,
// polling bridge: Android proxies the RFCOMM byte stream to a TCP listener on
// the same device, while the existing lockstep protocol remains unchanged.
// No method called by Java mutates Game state directly; callbacks are consumed
// by Update on Ebitengine's thread.

type bluetoothBridge = platformbridge.Bridge

func newBluetoothBridge() *bluetoothBridge { return platformbridge.New() }

const bluetoothPreambleSize = 16

// SetBluetoothAvailable is safe to call from Android before or during the
// game loop. Desktop builds leave the capability disabled by default.
func (g *Game) SetBluetoothAvailable(available bool) {
	if g == nil {
		return
	}
	g.bluetoothAvailable.Store(available)
	if !available {
		g.CancelBluetooth()
	}
}

func (g *Game) BluetoothAvailable() bool {
	return g != nil && g.bluetoothAvailable.Load()
}

// RequestBluetoothHost starts the normal host protocol on loopback and asks
// Android to expose that one listener over RFCOMM. The host is always Good,
// exactly like TCP multiplayer.
func (g *Game) RequestBluetoothHost(level int) error {
	if g == nil || g.bluetooth == nil {
		return fmt.Errorf("Bluetooth is unavailable")
	}
	if !g.BluetoothAvailable() {
		return fmt.Errorf("Bluetooth is unavailable on this device")
	}
	if g.network != nil {
		return fmt.Errorf("a multiplayer game is already configured")
	}
	if g.bundle == nil || len(g.bundle.Levels) == 0 {
		return fmt.Errorf("no campaign world is available")
	}
	if level < 0 || level >= len(g.bundle.Levels) {
		return fmt.Errorf("world index %d is outside the campaign", level)
	}
	preamble, err := newBluetoothPreamble()
	if err != nil {
		return err
	}
	const status = "WAITING FOR A BLUETOOTH PLAYER"
	if err := g.bluetooth.Reserve(platformbridge.Hosting, status); err != nil {
		return err
	}
	g.bluetooth.SetPreamble(preamble)
	if err := g.StartMultiplayer(NetworkConfig{ListenAddress: "127.0.0.1:0", LevelIndex: level, TransportPreamble: preamble}); err != nil {
		g.bluetooth.ReleaseReservation()
		return err
	}
	g.network.bluetooth = true
	_, port, err := net.SplitHostPort(g.network.address)
	if err != nil {
		g.stopMultiplayer()
		return fmt.Errorf("prepare Bluetooth loopback listener: %w", err)
	}
	g.network.status = status
	if g.mobileUI != nil {
		g.mobileUI.status = status
	}
	g.bluetooth.EnqueueCommand("BT_HOST|" + port + "|" + hex.EncodeToString(preamble))
	g.bluetooth.ClearPreamble()
	return nil
}

// RequestBluetoothJoin asks Android to choose/connect an RFCOMM peer. Android
// later supplies the address of its private loopback proxy through
// PostBluetoothReady; the TCP client is intentionally not started here.
func (g *Game) RequestBluetoothJoin() error {
	if g == nil || g.bluetooth == nil {
		return fmt.Errorf("Bluetooth is unavailable")
	}
	if g.network != nil {
		return fmt.Errorf("a multiplayer game is already configured")
	}
	if !g.BluetoothAvailable() {
		return fmt.Errorf("Bluetooth is unavailable on this device")
	}
	preamble, err := newBluetoothPreamble()
	if err != nil {
		return err
	}
	const status = "SELECT A BLUETOOTH DEVICE"
	if err := g.bluetooth.Reserve(platformbridge.Joining, status); err != nil {
		return err
	}
	g.bluetooth.SetPreamble(preamble)
	if g.mobileUI != nil {
		g.mobileUI.status = status
	}
	g.bluetooth.EnqueueCommand("BT_JOIN|" + hex.EncodeToString(preamble))
	return nil
}

// CancelBluetooth is safe to call from Android lifecycle callbacks. The
// actual network and game state are changed later by Update.
func (g *Game) CancelBluetooth() {
	if g == nil || g.bluetooth == nil {
		return
	}
	g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventCancel})
}

// PollPlatformCommand is called by Android from outside Ebitengine's thread.
// It never blocks; an empty string means that there is no work at present.
func (g *Game) PollPlatformCommand() string {
	if g == nil {
		return ""
	}
	return g.bluetooth.PollCommand()
}

// PostBluetoothStatus queues human-readable platform progress for Update.
func (g *Game) PostBluetoothStatus(status string) {
	if g == nil {
		return
	}
	g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventStatus, Status: platformbridge.CleanText(status, 120)})
}

// PostBluetoothReady supplies the TCP endpoint created by Android's client
// RFCOMM proxy. Only numeric loopback addresses are accepted.
func (g *Game) PostBluetoothReady(address, name string) {
	if g == nil {
		return
	}
	g.bluetooth.EnqueueEvent(platformbridge.Event{
		Kind: platformbridge.EventReady, Address: strings.TrimSpace(address),
		Name: platformbridge.CleanText(name, 64),
	})
}

// PostBluetoothFailure queues a platform error for Update.
func (g *Game) PostBluetoothFailure(message string) {
	if g == nil {
		return
	}
	g.bluetooth.EnqueueEvent(platformbridge.Event{Kind: platformbridge.EventFailure, Status: platformbridge.CleanText(message, 160)})
}

// BluetoothStatusText is read by the touch frontend on the game thread.
func (g *Game) BluetoothStatusText() string {
	if g == nil {
		return ""
	}
	return g.bluetooth.StatusText()
}

func (g *Game) updateBluetooth() {
	if g == nil || g.bluetooth == nil {
		return
	}
	for _, event := range g.bluetooth.TakeEvents() {
		switch event.Kind {
		case platformbridge.EventStatus:
			g.applyBluetoothStatus(event.Status)
		case platformbridge.EventReady:
			g.acceptBluetoothClientProxy(event.Address, event.Name)
		case platformbridge.EventFailure:
			g.failBluetooth(event.Status)
		case platformbridge.EventCancel:
			g.cancelBluetoothNow()
		}
	}
}

func (g *Game) applyBluetoothStatus(status string) {
	if status == "" || !g.bluetooth.SetStatus(status) {
		return
	}
	if g.mobileUI != nil {
		g.mobileUI.status = status
	}
	if g.network != nil && g.network.bluetooth && !g.network.connected && !g.network.terminal {
		g.network.status = status
	}
}

func (g *Game) acceptBluetoothClientProxy(address, name string) {
	if g.bluetooth.Phase() != platformbridge.Joining {
		return
	}
	loopback, err := platformbridge.NormalizeLoopbackAddress(address)
	if err != nil {
		g.failBluetooth(err.Error())
		return
	}
	status := "CONNECTING OVER BLUETOOTH"
	if name != "" {
		status = "CONNECTING TO " + name
	}
	preamble := g.bluetooth.Preamble()
	if len(preamble) != bluetoothPreambleSize {
		g.failBluetooth("Bluetooth proxy authentication is unavailable")
		return
	}
	if err := g.StartMultiplayer(NetworkConfig{
		JoinAddress:       loopback,
		PlayerName:        "ANDROID",
		TransportPreamble: preamble,
	}); err != nil {
		g.failBluetooth(err.Error())
		return
	}
	g.network.bluetooth = true
	g.bluetooth.ClearPreamble()
	g.network.status = status
	g.bluetooth.SetStatus(status)
	if g.mobileUI != nil {
		g.mobileUI.status = status
	}
}

func (g *Game) failBluetooth(message string) {
	message = platformbridge.CleanText(message, 160)
	message = strings.ToUpper(message)
	message = strings.TrimSpace(strings.TrimPrefix(message, "BLUETOOTH:"))
	message = strings.TrimSpace(strings.TrimPrefix(message, "BLUETOOTH "))
	if message == "" {
		message = "CONNECTION FAILED"
	}
	message = "BLUETOOTH: " + message
	g.cancelBluetoothNow()
	g.returnToBluetoothFront(message)
}

// cancelBluetoothNow only tears down the transport. Navigation belongs to the
// caller (for example BACK returns Home); a genuine platform failure calls
// returnToBluetoothFront separately so the error remains visible and retryable.
func (g *Game) cancelBluetoothNow() {
	if g == nil || g.bluetooth == nil || g.bluetooth.Phase() == platformbridge.Idle {
		return
	}
	if g.mobileUI != nil {
		g.mobileUI.status = ""
	}
	if g.network != nil && g.network.bluetooth {
		g.stopMultiplayer()
	} else {
		g.stopBluetoothPlatform()
	}
}

func (g *Game) returnToBluetoothFront(status string) {
	if g.mobileUI == nil {
		g.state = StateTitle
		return
	}
	g.openMobileFront(mobileui.FrontMultiplayer)
	g.mobileUI.status = status
}

// stopBluetoothPlatform is intentionally non-blocking and is also called by
// Close. Java owns all socket joins and performs them after polling BT_CANCEL.
func (g *Game) stopBluetoothPlatform() {
	if g == nil || g.bluetooth == nil {
		return
	}
	g.bluetooth.Stop()
}

func newBluetoothPreamble() ([]byte, error) {
	preamble := make([]byte, bluetoothPreambleSize)
	if _, err := cryptorand.Read(preamble); err != nil {
		return nil, fmt.Errorf("create Bluetooth proxy authentication: %w", err)
	}
	return preamble, nil
}
