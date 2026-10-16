package multiplayer

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"go-populous/internal/populous"
)

var (
	ErrPlayerOwnership = errors.New("command belongs to a different player")
	ErrSequence        = errors.New("invalid command sequence")
	ErrTickOrder       = errors.New("invalid lockstep tick order")
	ErrTooFarFuture    = errors.New("command requested too far in the future")
	ErrBatchFull       = errors.New("too many commands assigned to one tick")
	ErrDuplicateBatch  = errors.New("duplicate command batch")
)

const maxBufferedBatches = 512

// HostSession is an asynchronous host-side command scheduler. Remote intents
// are consumed on a background goroutine; Advance is called by the simulation
// loop once per tick and never performs socket I/O itself.
type HostSession struct {
	peer         *Peer
	localPlayer  int
	remotePlayer int
	inputDelay   uint64

	mu           sync.Mutex
	currentTick  uint64
	lastSequence [2]uint64
	lastTick     [2]uint64
	pending      map[uint64][]SequencedCommand

	activity chan struct{}
	events   chan WireMessage
	issues   chan error
}

func NewHostSession(peer *Peer, localPlayer int, startTick, inputDelay uint64) (*HostSession, error) {
	if peer == nil {
		return nil, fmt.Errorf("new host session: nil peer")
	}
	if localPlayer != populous.GodPlayer && localPlayer != populous.DevilPlayer {
		return nil, fmt.Errorf("%w: local player %d", ErrInvalidMessage, localPlayer)
	}
	if inputDelay > MaxFutureTicks {
		return nil, fmt.Errorf("%w: input delay %d", ErrInvalidMessage, inputDelay)
	}
	session := &HostSession{
		peer:         peer,
		localPlayer:  localPlayer,
		remotePlayer: localPlayer ^ 1,
		inputDelay:   inputDelay,
		currentTick:  startTick,
		pending:      make(map[uint64][]SequencedCommand),
		activity:     make(chan struct{}, 1),
		events:       make(chan WireMessage, peerQueueSize),
		issues:       make(chan error, peerQueueSize),
	}
	go session.receiveLoop()
	return session, nil
}

func (session *HostSession) CurrentTick() uint64 {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.currentTick
}

func (session *HostSession) Activity() <-chan struct{}  { return session.activity }
func (session *HostSession) Events() <-chan WireMessage { return session.events }
func (session *HostSession) Issues() <-chan error       { return session.issues }
func (session *HostSession) Done() <-chan struct{}      { return session.peer.Done() }
func (session *HostSession) Close() error               { return session.peer.Close() }

// SubmitLocal schedules a host player's command at the earliest safe tick.
// The returned Intent records the assigned tick and canonical sequence.
func (session *HostSession) SubmitLocal(command populous.Command) (Intent, error) {
	if command.Player != session.localPlayer {
		return Intent{}, fmt.Errorf("%w: command player %d, local player %d", ErrPlayerOwnership, command.Player, session.localPlayer)
	}
	if err := command.Validate(); err != nil {
		return Intent{}, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	sequence := session.lastSequence[session.localPlayer] + 1
	intent := Intent{Sequence: sequence, Command: command}
	tick, err := session.scheduleLocked(intent, session.localPlayer)
	if err != nil {
		return Intent{}, err
	}
	intent.RequestedTick = tick
	return intent, nil
}

// Advance emits the canonical batch for exactly the next tick. It sends even
// an empty batch so the client knows that it may advance its simulation.
func (session *HostSession) Advance(tick uint64) (CommandBatch, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if tick != session.currentTick+1 {
		return CommandBatch{}, fmt.Errorf("%w: got %d, want %d", ErrTickOrder, tick, session.currentTick+1)
	}

	commands := append([]SequencedCommand(nil), session.pending[tick]...)
	sort.Slice(commands, func(i, j int) bool {
		left, right := commands[i], commands[j]
		if left.Command.Player != right.Command.Player {
			return left.Command.Player < right.Command.Player
		}
		return left.Sequence < right.Sequence
	})
	batch := CommandBatch{Tick: tick, Commands: commands}
	if err := validateBatch(batch); err != nil {
		return CommandBatch{}, err
	}
	if err := session.peer.TrySend(batch); err != nil {
		return CommandBatch{}, err
	}
	delete(session.pending, tick)
	session.currentTick = tick
	return batch, nil
}

func (session *HostSession) SendStateHash(tick uint64, hash [32]byte) error {
	return session.peer.TrySend(StateHash{Tick: tick, Hash: hash})
}

// SendPing starts an application-level heartbeat. The remote session replies
// with a Pong carrying the same nonce and timestamp on Events.
func (session *HostSession) SendPing(nonce uint64, sentUnixMillis int64) error {
	return session.peer.TrySend(Ping{Nonce: nonce, SentUnixMillis: sentUnixMillis})
}

func (session *HostSession) SendSnapshot(world *populous.World) error {
	if world == nil || world.GameTurn < 0 {
		return fmt.Errorf("send snapshot: nil or invalid world")
	}
	return session.peer.TrySend(Snapshot{
		Tick:     uint64(world.GameTurn),
		Snapshot: world.Snapshot(),
		Rules:    world.Rules,
	})
}

func (session *HostSession) receiveLoop() {
	defer close(session.events)
	defer close(session.issues)
	for message := range session.peer.Incoming() {
		switch value := message.(type) {
		case *Intent:
			if err := session.scheduleRemote(*value); err != nil {
				session.reportIssue(err)
				_ = session.peer.TrySend(ErrorMessage{Code: "invalid_intent", Message: err.Error()})
			}
		case *Ping:
			if err := session.peer.TrySend(Pong{Nonce: value.Nonce, SentUnixMillis: value.SentUnixMillis}); err != nil {
				session.reportIssue(err)
				session.peer.stop(err)
				return
			}
		case *StateHash, *Pong, *Disconnect, *ErrorMessage:
			select {
			case session.events <- message:
			case <-session.peer.Done():
				return
			}
		default:
			err := fmt.Errorf("%w: host received %s", ErrUnexpectedMessage, TypeOf(message))
			session.reportIssue(err)
			session.peer.stop(err)
			return
		}
	}
}

func (session *HostSession) scheduleRemote(intent Intent) error {
	if intent.Command.Player != session.remotePlayer {
		return fmt.Errorf("%w: command player %d, remote player %d", ErrPlayerOwnership, intent.Command.Player, session.remotePlayer)
	}
	if intent.Command.Kind == populous.CommandPaintRaise || intent.Command.Kind == populous.CommandPaintLower {
		return fmt.Errorf("%w: remote paint command", ErrPlayerOwnership)
	}
	session.mu.Lock()
	_, err := session.scheduleLocked(intent, session.remotePlayer)
	session.mu.Unlock()
	if err == nil {
		signal(session.activity)
	}
	return err
}

func (session *HostSession) scheduleLocked(intent Intent, player int) (uint64, error) {
	if err := validateIntent(intent); err != nil {
		return 0, err
	}
	if intent.Command.Player != player {
		return 0, fmt.Errorf("%w: command player %d, expected %d", ErrPlayerOwnership, intent.Command.Player, player)
	}
	if intent.Sequence != session.lastSequence[player]+1 {
		return 0, fmt.Errorf("%w: player %d got %d, want %d", ErrSequence, player, intent.Sequence, session.lastSequence[player]+1)
	}

	earliest := session.currentTick + session.inputDelay
	if earliest <= session.currentTick {
		earliest = session.currentTick + 1
	}
	tick := intent.RequestedTick
	if tick < earliest {
		tick = earliest
	}
	// Commands from one player must never overtake an earlier sequence merely
	// because it requested a nearer tick. Multiple commands may share a tick and
	// are then applied in sequence order.
	if tick < session.lastTick[player] {
		tick = session.lastTick[player]
	}
	if tick > session.currentTick+MaxFutureTicks {
		return 0, fmt.Errorf("%w: requested %d from current %d", ErrTooFarFuture, tick, session.currentTick)
	}
	if len(session.pending[tick]) >= MaxCommandsPerBatch {
		return 0, fmt.Errorf("%w: tick %d", ErrBatchFull, tick)
	}

	session.pending[tick] = append(session.pending[tick], SequencedCommand{
		Sequence: intent.Sequence,
		Command:  intent.Command,
	})
	session.lastSequence[player] = intent.Sequence
	session.lastTick[player] = tick
	return tick, nil
}

func (session *HostSession) reportIssue(err error) {
	select {
	case session.issues <- err:
	default:
	}
}

// ClientSession sends player intents asynchronously and buffers the ordered
// batches received from the host. The simulation calls NextBatch and advances
// only when the requested tick is available.
type ClientSession struct {
	peer       *Peer
	player     int
	inputDelay uint64

	mu               sync.Mutex
	currentTick      uint64
	highestReceived  uint64
	nextSequence     uint64
	receivedSequence [2]uint64
	coveredSequence  [2]uint64
	batches          map[uint64]CommandBatch

	ready  chan struct{}
	events chan WireMessage
	issues chan error
}

func NewClientSession(peer *Peer, player int, startTick, inputDelay uint64) (*ClientSession, error) {
	if peer == nil {
		return nil, fmt.Errorf("new client session: nil peer")
	}
	if player != populous.GodPlayer && player != populous.DevilPlayer {
		return nil, fmt.Errorf("%w: client player %d", ErrInvalidMessage, player)
	}
	if inputDelay > MaxFutureTicks {
		return nil, fmt.Errorf("%w: input delay %d", ErrInvalidMessage, inputDelay)
	}
	session := &ClientSession{
		peer:            peer,
		player:          player,
		inputDelay:      inputDelay,
		currentTick:     startTick,
		highestReceived: startTick,
		batches:         make(map[uint64]CommandBatch),
		ready:           make(chan struct{}, 1),
		events:          make(chan WireMessage, peerQueueSize),
		issues:          make(chan error, peerQueueSize),
	}
	go session.receiveLoop()
	return session, nil
}

func (session *ClientSession) CurrentTick() uint64 {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.currentTick
}

func (session *ClientSession) Ready() <-chan struct{}     { return session.ready }
func (session *ClientSession) Events() <-chan WireMessage { return session.events }
func (session *ClientSession) Issues() <-chan error       { return session.issues }
func (session *ClientSession) Done() <-chan struct{}      { return session.peer.Done() }
func (session *ClientSession) Close() error               { return session.peer.Close() }

// SubmitAt queues a local intent. currentTick should be the client's currently
// rendered simulation tick; the host remains authoritative and may move the
// command to a later tick when it arrives.
func (session *ClientSession) SubmitAt(currentTick uint64, command populous.Command) (Intent, error) {
	if command.Player != session.player {
		return Intent{}, fmt.Errorf("%w: command player %d, client player %d", ErrPlayerOwnership, command.Player, session.player)
	}
	if err := command.Validate(); err != nil {
		return Intent{}, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	if currentTick != session.currentTick {
		return Intent{}, fmt.Errorf("%w: submit at %d, current client tick %d", ErrTickOrder, currentTick, session.currentTick)
	}
	requested := currentTick + session.inputDelay
	if requested <= currentTick {
		requested = currentTick + 1
	}
	intent := Intent{
		RequestedTick: requested,
		Sequence:      session.nextSequence + 1,
		Command:       command,
	}
	if err := session.peer.TrySend(intent); err != nil {
		return Intent{}, err
	}
	session.nextSequence = intent.Sequence
	return intent, nil
}

// NextBatch returns and consumes the next batch only when it has arrived.
func (session *ClientSession) NextBatch(tick uint64) (CommandBatch, bool, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if tick != session.currentTick+1 {
		return CommandBatch{}, false, fmt.Errorf("%w: got %d, want %d", ErrTickOrder, tick, session.currentTick+1)
	}
	batch, ok := session.batches[tick]
	if !ok {
		return CommandBatch{}, false, nil
	}
	for _, command := range batch.Commands {
		session.coveredSequence[command.Command.Player] = command.Sequence
	}
	delete(session.batches, tick)
	session.currentTick = tick
	return batch, true, nil
}

// ResyncTo advances the client's network cursor after installing a Snapshot at
// tick. Because TCP preserves frame order, all batches through the snapshot's
// tick have already reached the receive loop; they are discarded here because
// their effects are present in the snapshot. The returned per-player sequence
// watermarks cover consumed batches and discarded batches through tick, but
// never future buffered commands.
func (session *ClientSession) ResyncTo(tick uint64) ([2]uint64, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if tick < session.currentTick {
		return session.coveredSequence, fmt.Errorf("%w: snapshot tick %d is behind current tick %d", ErrTickOrder, tick, session.currentTick)
	}
	if tick > session.highestReceived {
		return session.coveredSequence, fmt.Errorf("%w: snapshot tick %d is ahead of received batches %d", ErrTickOrder, tick, session.highestReceived)
	}
	for batchTick, batch := range session.batches {
		if batchTick <= tick {
			for _, command := range batch.Commands {
				player := command.Command.Player
				if command.Sequence > session.coveredSequence[player] {
					session.coveredSequence[player] = command.Sequence
				}
			}
			delete(session.batches, batchTick)
		}
	}
	session.currentTick = tick
	// Remove stale wakeups, then restore one if the next usable batch is already
	// buffered.
	select {
	case <-session.ready:
	default:
	}
	if _, ok := session.batches[tick+1]; ok {
		signal(session.ready)
	}
	return session.coveredSequence, nil
}

func (session *ClientSession) SendStateHash(tick uint64, hash [32]byte) error {
	return session.peer.TrySend(StateHash{Tick: tick, Hash: hash})
}

// SendPing starts an application-level heartbeat. The remote session replies
// with a Pong carrying the same nonce and timestamp on Events.
func (session *ClientSession) SendPing(nonce uint64, sentUnixMillis int64) error {
	return session.peer.TrySend(Ping{Nonce: nonce, SentUnixMillis: sentUnixMillis})
}

func (session *ClientSession) receiveLoop() {
	defer close(session.events)
	defer close(session.issues)
	for message := range session.peer.Incoming() {
		switch value := message.(type) {
		case *CommandBatch:
			if err := session.storeBatch(*value); err != nil {
				session.reportIssue(err)
				_ = session.peer.Close()
				return
			}
		case *Ping:
			if err := session.peer.TrySend(Pong{Nonce: value.Nonce, SentUnixMillis: value.SentUnixMillis}); err != nil {
				session.reportIssue(err)
				session.peer.stop(err)
				return
			}
		case *StateHash, *Snapshot, *Pong, *Disconnect, *ErrorMessage:
			select {
			case session.events <- message:
			case <-session.peer.Done():
				return
			}
		default:
			err := fmt.Errorf("%w: client received %s", ErrUnexpectedMessage, TypeOf(message))
			session.reportIssue(err)
			session.peer.stop(err)
			return
		}
	}
}

func (session *ClientSession) storeBatch(batch CommandBatch) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if batch.Tick <= session.currentTick {
		return fmt.Errorf("%w: tick %d already consumed", ErrDuplicateBatch, batch.Tick)
	}
	if batch.Tick != session.highestReceived+1 {
		return fmt.Errorf("%w: received %d after %d", ErrTickOrder, batch.Tick, session.highestReceived)
	}
	if len(session.batches) >= maxBufferedBatches {
		return fmt.Errorf("%w: more than %d buffered batches", ErrBatchFull, maxBufferedBatches)
	}
	receivedSequence := session.receivedSequence
	for _, command := range batch.Commands {
		player := command.Command.Player
		expected := receivedSequence[player] + 1
		if command.Sequence != expected {
			return fmt.Errorf("%w: player %d got %d, want %d", ErrSequence, player, command.Sequence, expected)
		}
		receivedSequence[player] = command.Sequence
	}
	session.batches[batch.Tick] = batch
	session.highestReceived = batch.Tick
	session.receivedSequence = receivedSequence
	signal(session.ready)
	return nil
}

func (session *ClientSession) reportIssue(err error) {
	select {
	case session.issues <- err:
	default:
	}
}

func signal(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

// ApplyBatch validates a complete batch, applies its commands in canonical
// order, then advances the simulation exactly once. Rejected game actions are
// returned as false entries and remain deterministic on every peer.
func ApplyBatch(world *populous.World, batch CommandBatch, computerControlled [2]bool) ([]bool, error) {
	if world == nil {
		return nil, populous.ErrNilWorld
	}
	if err := validateBatch(batch); err != nil {
		return nil, err
	}
	if world.GameTurn < 0 || batch.Tick != uint64(world.GameTurn+1) {
		return nil, fmt.Errorf("%w: batch %d, world expects %d", ErrTickOrder, batch.Tick, world.GameTurn+1)
	}
	// Validate every command before mutating the world, so a corrupt batch can
	// never be applied partially.
	for _, sequenced := range batch.Commands {
		if err := sequenced.Command.Validate(); err != nil {
			return nil, err
		}
	}

	results := make([]bool, len(batch.Commands))
	for index, sequenced := range batch.Commands {
		applied, err := world.ApplyCommand(sequenced.Command)
		if err != nil {
			return nil, err
		}
		results[index] = applied
	}
	world.TickWithComputer(computerControlled)
	return results, nil
}

// RestoreWorld constructs a world from an authoritative network snapshot and
// selects the receiving player's local score view. Both canonical player
// scores are carried by the snapshot and remain part of World.StateHash.
func RestoreWorld(snapshot populous.WorldSnapshot, rules populous.TerrainRules, localPlayer int) (*populous.World, error) {
	if localPlayer != populous.GodPlayer && localPlayer != populous.DevilPlayer {
		return nil, fmt.Errorf("%w: local player %d", ErrInvalidMessage, localPlayer)
	}
	if snapshot.GameTurn < 0 || len(snapshot.Peeps) > populous.MaxPeeps {
		return nil, fmt.Errorf("%w: invalid world snapshot", ErrInvalidMessage)
	}
	world := populous.WorldFromSnapshot(snapshot, rules)
	world.SetScorePlayer(localPlayer)
	return world, nil
}
