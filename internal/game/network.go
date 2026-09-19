package game

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"go-populous/internal/multiplayer"
	"go-populous/internal/populous"
)

const (
	// Recycled people and friendly-town traversal change shared simulation
	// results even though the packet and snapshot layouts stay compatible.
	networkCompatibilityID = "go-populous-lockstep-3"
	networkHashInterval    = 32
	networkHashHistory     = 128
	networkCatchUpTicks    = 4
	networkPingInterval    = 5 * time.Second
	networkPingTimeout     = 15 * time.Second
	networkProgressTimeout = 5 * time.Second
)

type networkRole uint8

const (
	networkHost networkRole = iota + 1
	networkClient
)

type pendingLocalCommand struct {
	kind           populous.CommandKind
	modeGeneration uint64
}

// NetworkConfig selects one modern two-player TCP mode. Exactly one of
// ListenAddress and JoinAddress must be set. The host plays good and chooses
// the world; the joining player plays evil.
type NetworkConfig struct {
	ListenAddress     string
	JoinAddress       string
	PlayerName        string
	LevelIndex        int
	TransportPreamble []byte
}

type networkGame struct {
	role    networkRole
	address string
	status  string
	// bluetooth means that TCP is the private loopback half of an Android
	// RFCOMM proxy. The wire protocol itself remains transport-independent.
	bluetooth bool

	cancel context.CancelFunc

	listener      *multiplayer.HostListener
	hostResults   <-chan multiplayer.HostConnectResult
	clientResults <-chan multiplayer.ClientConnectResult
	peer          *multiplayer.Peer
	host          *multiplayer.HostSession
	client        *multiplayer.ClientSession

	connected bool
	terminal  bool

	localHashes    map[uint64][32]byte
	remoteHashes   map[uint64][32]byte
	localCommands  map[uint64]pendingLocalCommand
	modeGeneration uint64
	pingNonce      uint64
	pingDeadline   time.Time
	nextPing       time.Time
	lastBatchAt    time.Time
	desyncCount    int
}

// StartMultiplayer prepares a single two-player match. Connection and
// handshake work continues on background goroutines; the Ebiten update loop
// only polls buffered channels.
func (g *Game) StartMultiplayer(config NetworkConfig) error {
	if g == nil || g.world == nil {
		return fmt.Errorf("start multiplayer: no active world")
	}
	if g.network != nil {
		return fmt.Errorf("start multiplayer: already configured")
	}
	listen := strings.TrimSpace(config.ListenAddress)
	join := strings.TrimSpace(config.JoinAddress)
	if (listen == "") == (join == "") {
		return fmt.Errorf("start multiplayer: set exactly one of listen or join address")
	}
	if join != "" && len(config.PlayerName) > 64 {
		return fmt.Errorf("start multiplayer: player name is longer than 64 characters")
	}

	ctx, cancel := context.WithCancel(context.Background())
	network := &networkGame{
		cancel:        cancel,
		localHashes:   make(map[uint64][32]byte),
		remoteHashes:  make(map[uint64][32]byte),
		localCommands: make(map[uint64]pendingLocalCommand),
	}
	g.network = network
	g.tutorialActive = false
	g.tutorialPaused = false
	g.paintMap = false
	g.mode = ModeSculpt
	g.computerControlled = [2]bool{false, false}
	g.state = StateGame

	if listen != "" {
		g.setLevel(config.LevelIndex)
		g.player = populous.GodPlayer
		g.world.SetScorePlayer(g.player)
		g.world.ComputerControlled = g.computerControlled
		g.resetViewedPeep()
		g.centerOnPlayerLeader()

		start := multiplayer.Start{
			Tick:     uint64(g.world.GameTurn),
			Snapshot: g.world.Snapshot(),
			Rules:    g.world.Rules,
		}
		listener, err := multiplayer.ListenHostTCP(ctx, listen, multiplayer.HostConnectionConfig{
			Handshake: multiplayer.HostHandshakeConfig{
				BuildID:        currentNetworkBuildID(),
				SessionID:      newNetworkSessionID(),
				AssignedPlayer: populous.DevilPlayer,
				TickRate:       multiplayer.DefaultTickRate,
				InputDelay:     multiplayer.DefaultInputDelay,
				Start:          start,
			},
			Timeout:           10 * time.Minute,
			TransportPreamble: config.TransportPreamble,
		})
		if err != nil {
			cancel()
			g.network = nil
			return err
		}
		network.role = networkHost
		network.listener = listener
		network.hostResults = listener.Results()
		network.address = listener.Addr().String()
		network.status = "LISTENING " + network.address
		return nil
	}

	g.player = populous.DevilPlayer
	g.world.SetScorePlayer(g.player)
	g.world.ComputerControlled = g.computerControlled
	g.resetViewedPeep()
	network.role = networkClient
	network.address = join
	network.status = "CONNECTING " + join
	network.clientResults = multiplayer.DialClientTCP(ctx, join, multiplayer.ClientConnectionConfig{
		Hello:             multiplayer.NewHello(currentNetworkBuildID(), config.PlayerName, populous.DevilPlayer),
		Timeout:           30 * time.Second,
		TransportPreamble: config.TransportPreamble,
	})
	return nil
}

// currentNetworkBuildID rejects normal builds from different revisions while
// keeping cross-platform binaries from the same revision compatible. The
// explicit compatibility prefix must be bumped whenever uncommitted protocol
// or simulation changes are distributed intentionally.
func currentNetworkBuildID() string {
	id := networkCompatibilityID
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return id
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		id += "-" + info.Main.Version
	}
	revision := ""
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if revision != "" {
		id += "-" + revision
	}
	if modified {
		id += "-dirty"
	}
	if len(id) > 128 {
		id = id[:128]
	}
	return id
}

func newNetworkSessionID() uint64 {
	var data [8]byte
	if _, err := cryptorand.Read(data[:]); err == nil {
		return binary.LittleEndian.Uint64(data[:])
	}
	return uint64(time.Now().UnixNano())
}

func (g *Game) multiplayerEnabled() bool {
	return g != nil && g.network != nil
}

func (g *Game) multiplayerReady() bool {
	return g.multiplayerEnabled() && g.network.connected && !g.network.terminal
}

func (g *Game) issueCommand(command populous.Command) bool {
	if g == nil || g.world == nil {
		return false
	}
	if g.network == nil {
		applied, err := g.world.ApplyCommand(command)
		return err == nil && applied
	}
	if !g.multiplayerReady() {
		return false
	}
	if command.Kind == populous.CommandPaintRaise || command.Kind == populous.CommandPaintLower {
		g.network.status = "PAINT MODE IS DISABLED ONLINE"
		return false
	}

	var (
		intent multiplayer.Intent
		err    error
	)
	if g.network.role == networkHost {
		intent, err = g.network.host.SubmitLocal(command)
	} else {
		intent, err = g.network.client.SubmitAt(uint64(g.world.GameTurn), command)
	}
	if err != nil {
		g.network.fail(err)
		return false
	}
	g.network.localCommands[intent.Sequence] = pendingLocalCommand{
		kind:           command.Kind,
		modeGeneration: g.network.modeGeneration,
	}
	return true
}

// advanceWorld advances one offline tick, one host tick, or as many as four
// already-buffered client ticks. A client never predicts a simulation tick.
func (g *Game) advanceWorld() bool {
	if g == nil || g.world == nil {
		return false
	}
	viewedPos := g.viewedPeepPosition()
	if g.network == nil {
		g.world.TickWithComputer(g.computerControlled)
		g.refreshViewedPeep(viewedPos)
		return true
	}

	g.network.pollConnection(g)
	g.network.pollTransport()
	g.network.pollEvents(g)
	g.network.pollHeartbeat()
	if g.network.terminal && g.network.bluetooth {
		g.stopBluetoothPlatform()
	}
	if !g.multiplayerReady() {
		return false
	}

	advanced := false
	switch g.network.role {
	case networkHost:
		next := uint64(g.world.GameTurn + 1)
		batch, err := g.network.host.Advance(next)
		if err != nil {
			g.network.fail(err)
			return false
		}
		if err := g.network.applyBatch(g, batch); err != nil {
			g.network.fail(err)
			return false
		}
		advanced = true
		g.network.afterTick(g)
	case networkClient:
		for count := 0; count < networkCatchUpTicks; count++ {
			next := uint64(g.world.GameTurn + 1)
			batch, ok, err := g.network.client.NextBatch(next)
			if err != nil {
				g.network.fail(err)
				return advanced
			}
			if !ok {
				break
			}
			if err := g.network.applyBatch(g, batch); err != nil {
				g.network.fail(err)
				return advanced
			}
			advanced = true
			g.network.afterTick(g)
		}
		if advanced {
			g.network.lastBatchAt = time.Now()
		} else if g.network.progressTimedOut(time.Now()) {
			g.network.fail(errors.New("multiplayer host stopped advancing the simulation"))
			return false
		}
	}
	if advanced {
		g.refreshViewedPeep(viewedPos)
	}
	return advanced
}

func (network *networkGame) applyBatch(g *Game, batch multiplayer.CommandBatch) error {
	results, err := multiplayer.ApplyBatch(g.world, batch, g.computerControlled)
	if err != nil {
		return err
	}
	for index, sequenced := range batch.Commands {
		if sequenced.Command.Player != g.player {
			continue
		}
		pending, ok := network.localCommands[sequenced.Sequence]
		if !ok {
			continue
		}
		delete(network.localCommands, sequenced.Sequence)
		if pending.kind == populous.CommandSwamp && pending.modeGeneration == network.modeGeneration && g.mode == ModeSwamp && index < len(results) && results[index] {
			g.mode = ModeSculpt
			network.modeGeneration++
		}
	}
	return nil
}

func (network *networkGame) pollConnection(g *Game) {
	if network == nil || network.connected || network.terminal {
		return
	}
	switch network.role {
	case networkHost:
		select {
		case result, ok := <-network.hostResults:
			if !ok {
				if !network.connected {
					network.fail(errors.New("multiplayer listener closed"))
				}
				return
			}
			if result.Err != nil {
				network.fail(result.Err)
				return
			}
			network.peer = result.Peer
			network.host = result.Session
			network.connected = true
			network.status = "ONLINE AS GOOD"
			network.nextPing = time.Now()
		default:
		}
	case networkClient:
		select {
		case result, ok := <-network.clientResults:
			if !ok {
				if !network.connected {
					network.fail(errors.New("multiplayer connection closed"))
				}
				return
			}
			if result.Err != nil {
				network.fail(result.Err)
				return
			}
			world, err := multiplayer.RestoreWorld(result.Start.Snapshot, result.Start.Rules, result.Welcome.AssignedPlayer)
			if err != nil {
				_ = result.Close()
				network.fail(err)
				return
			}
			world.ComputerControlled = [2]bool{false, false}
			g.world = world
			g.player = result.Welcome.AssignedPlayer
			g.computerControlled = [2]bool{false, false}
			g.levelIndex = clampInt(world.Level.Number, 0, maxInt(0, len(g.bundle.Levels)-1))
			g.mode = ModeSculpt
			g.resetViewedPeep()
			g.centerOnPlayerLeader()
			network.peer = result.Peer
			network.client = result.Session
			network.connected = true
			network.status = "ONLINE AS EVIL"
			now := time.Now()
			network.nextPing = now
			network.lastBatchAt = now
		default:
		}
	}
}

func (network *networkGame) pollTransport() {
	if network == nil || network.peer == nil || network.terminal {
		return
	}
	select {
	case err, ok := <-network.peer.Errors():
		if ok && err != nil {
			network.fail(err)
		}
	default:
	}
	if network.terminal {
		return
	}
	select {
	case <-network.peer.Done():
		network.fail(errors.New("other player disconnected"))
	default:
	}
}

func (network *networkGame) pollEvents(g *Game) {
	if network == nil || !network.connected || network.terminal {
		return
	}
	if network.role == networkHost {
		for count := 0; count < 64; count++ {
			select {
			case err, ok := <-network.host.Issues():
				if ok && err != nil {
					network.fail(err)
				}
			case message, ok := <-network.host.Events():
				if ok {
					network.handleEvent(g, message)
				}
			default:
				return
			}
			if network.terminal {
				return
			}
		}
		return
	}
	for count := 0; count < 64; count++ {
		select {
		case err, ok := <-network.client.Issues():
			if ok && err != nil {
				network.fail(err)
			}
		case message, ok := <-network.client.Events():
			if ok {
				network.handleEvent(g, message)
			}
		default:
			return
		}
		if network.terminal {
			return
		}
	}
}

func (network *networkGame) handleEvent(g *Game, message multiplayer.WireMessage) {
	switch value := message.(type) {
	case *multiplayer.StateHash:
		if value.Tick%networkHashInterval != 0 {
			network.fail(fmt.Errorf("remote state hash for unexpected tick %d", value.Tick))
			return
		}
		currentTick := uint64(g.world.GameTurn)
		if currentTick > networkHashHistory && value.Tick < currentTick-networkHashHistory {
			return
		}
		if _, exists := network.remoteHashes[value.Tick]; !exists && len(network.remoteHashes) >= networkHashHistory*2 {
			network.fail(errors.New("too many unmatched remote state hashes"))
			return
		}
		network.remoteHashes[value.Tick] = value.Hash
		network.compareHash(g, value.Tick)
	case *multiplayer.Snapshot:
		if network.role != networkClient {
			return
		}
		world, err := multiplayer.RestoreWorld(value.Snapshot, value.Rules, g.player)
		if err != nil {
			network.fail(err)
			return
		}
		covered, err := network.client.ResyncTo(value.Tick)
		if err != nil {
			network.fail(err)
			return
		}
		for sequence := range network.localCommands {
			if sequence <= covered[g.player] {
				delete(network.localCommands, sequence)
			}
		}
		g.world = world
		g.world.ComputerControlled = g.computerControlled
		g.resetViewedPeep()
		network.localHashes = make(map[uint64][32]byte)
		network.remoteHashes = make(map[uint64][32]byte)
		network.lastBatchAt = time.Now()
		network.status = "ONLINE AS EVIL - RESYNCED"
	case *multiplayer.ErrorMessage:
		network.fail(fmt.Errorf("remote rejected the session (%s): %s", value.Code, value.Message))
	case *multiplayer.Pong:
		if !network.pingDeadline.IsZero() && value.Nonce == network.pingNonce {
			network.pingDeadline = time.Time{}
			network.nextPing = time.Now().Add(networkPingInterval)
		}
	case *multiplayer.Disconnect:
		reason := strings.TrimSpace(value.Reason)
		if reason == "" {
			reason = "other player disconnected"
		}
		network.fail(errors.New(reason))
	}
}

func (network *networkGame) pollHeartbeat() {
	if network == nil || !network.connected || network.terminal {
		return
	}
	now := time.Now()
	if !network.pingDeadline.IsZero() {
		if now.After(network.pingDeadline) {
			network.fail(errors.New("multiplayer heartbeat timed out"))
		}
		return
	}
	if now.Before(network.nextPing) {
		return
	}
	network.pingNonce++
	var err error
	if network.role == networkHost {
		err = network.host.SendPing(network.pingNonce, now.UnixMilli())
	} else {
		err = network.client.SendPing(network.pingNonce, now.UnixMilli())
	}
	if err != nil {
		network.fail(err)
		return
	}
	network.pingDeadline = now.Add(networkPingTimeout)
}

func (network *networkGame) progressTimedOut(now time.Time) bool {
	return network != nil && network.role == networkClient && network.connected && !network.terminal && !network.lastBatchAt.IsZero() && now.After(network.lastBatchAt.Add(networkProgressTimeout))
}

func (network *networkGame) afterTick(g *Game) {
	tick := uint64(g.world.GameTurn)
	if tick%networkHashInterval != 0 {
		return
	}
	hash := g.world.StateHash()
	network.localHashes[tick] = hash
	network.compareHash(g, tick)
	var err error
	if network.role == networkHost {
		err = network.host.SendStateHash(tick, hash)
	} else {
		err = network.client.SendStateHash(tick, hash)
	}
	if err != nil {
		network.fail(err)
	}
	if tick > networkHashHistory {
		delete(network.localHashes, tick-networkHashHistory)
		delete(network.remoteHashes, tick-networkHashHistory)
	}
}

func (network *networkGame) compareHash(g *Game, tick uint64) {
	local, localOK := network.localHashes[tick]
	remote, remoteOK := network.remoteHashes[tick]
	if !localOK || !remoteOK {
		return
	}
	delete(network.remoteHashes, tick)
	if local == remote {
		network.desyncCount = 0
		return
	}
	network.desyncCount++
	if network.desyncCount >= 3 {
		network.fail(errors.New("repeated world-state divergence; builds are incompatible"))
		return
	}
	if network.role == networkHost {
		if err := network.host.SendSnapshot(g.world); err != nil {
			network.fail(err)
			return
		}
		network.status = "ONLINE AS GOOD - RESYNC SENT"
		return
	}
	network.status = "ONLINE AS EVIL - DESYNC DETECTED"
}

func (network *networkGame) fail(err error) {
	if network == nil || network.terminal {
		return
	}
	network.terminal = true
	network.connected = false
	if err == nil {
		network.status = "NETWORK STOPPED"
	} else {
		network.status = "NETWORK ERROR: " + err.Error()
	}
	_ = network.close()
}

func (network *networkGame) displayStatus() string {
	if network == nil || network.status == "" {
		return "NETWORK"
	}
	return network.status
}

// Close releases a pending recording, listener/dial or the active peer. It is safe to
// call after Ebiten exits even when setup never completed.
func (g *Game) Close() error {
	if g == nil {
		return nil
	}
	g.stopBluetoothPlatform()
	recordingErr := g.closeDemoRecording()
	recordingErr = errors.Join(recordingErr, g.audioTrace.Close())
	if g.network == nil {
		return recordingErr
	}
	return errors.Join(recordingErr, g.network.close())
}

func (network *networkGame) close() error {
	if network == nil {
		return nil
	}
	var first error
	if network.connected && !network.terminal {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		if network.host != nil {
			first = network.host.SendDisconnect(ctx, "other player left the match")
		} else if network.client != nil {
			first = network.client.SendDisconnect(ctx, "other player left the match")
		}
		cancel()
		if errors.Is(first, context.Canceled) || errors.Is(first, context.DeadlineExceeded) || errors.Is(first, multiplayer.ErrSessionClosed) {
			first = nil
		}
	}
	network.cancel()
	if network.host != nil {
		if err := network.host.Close(); first == nil {
			first = err
		}
	} else if network.client != nil {
		if err := network.client.Close(); first == nil {
			first = err
		}
	}
	if network.listener != nil {
		if err := network.listener.Close(); first == nil {
			first = err
		}
	}
	// A successful handshake can finish between the last Update poll and
	// cancellation. Drain and close ownership transferred through that buffered
	// result so neither endpoint is left believing the match is alive.
	if network.host == nil && network.hostResults != nil {
		if result, ok := <-network.hostResults; ok {
			if err := result.Close(); first == nil {
				first = err
			}
		}
	}
	if network.client == nil && network.clientResults != nil {
		if result, ok := <-network.clientResults; ok {
			if err := result.Close(); first == nil {
				first = err
			}
		}
	}
	return first
}

func (g *Game) stopMultiplayer() {
	if g == nil || g.network == nil {
		return
	}
	_ = g.Close()
	g.network = nil
	g.player = populous.GodPlayer
	g.computerControlled = [2]bool{false, true}
	if g.world != nil {
		g.world.ComputerControlled = g.computerControlled
	}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
