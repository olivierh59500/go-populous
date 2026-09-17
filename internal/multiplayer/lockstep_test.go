package multiplayer

import (
	"errors"
	"net"
	"testing"
	"time"

	"go-populous/internal/populous"
)

func TestHostClientLockstepReplayOverNetPipe(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	hostPeer, err := NewPeer(hostConnection)
	if err != nil {
		t.Fatal(err)
	}
	clientPeer, err := NewPeer(clientConnection)
	if err != nil {
		t.Fatal(err)
	}
	defer hostPeer.Close()
	defer clientPeer.Close()

	host, err := NewHostSession(hostPeer, populous.GodPlayer, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClientSession(clientPeer, populous.DevilPlayer, 0, 2)
	if err != nil {
		t.Fatal(err)
	}

	hostWorld := testWorld()
	clientWorld := testWorld()
	hostWorld.Score = 123
	hostWorld.ScorePlayer = populous.GodPlayer
	clientWorld.Score = 456
	clientWorld.ScorePlayer = populous.DevilPlayer

	remoteCommand := populous.Command{
		Kind:   populous.CommandSetTendency,
		Player: populous.DevilPlayer,
		Value:  populous.FightMode,
	}
	if _, err := client.SubmitAt(0, remoteCommand); err != nil {
		t.Fatalf("SubmitAt: %v", err)
	}
	select {
	case <-host.Activity():
	case <-time.After(2 * time.Second):
		t.Fatal("host did not receive remote intent")
	}

	for tick := uint64(1); tick <= 2; tick++ {
		hostBatch, err := host.Advance(tick)
		if err != nil {
			t.Fatalf("host Advance(%d): %v", tick, err)
		}
		clientBatch := awaitBatch(t, client, tick)
		if len(hostBatch.Commands) != len(clientBatch.Commands) {
			t.Fatalf("tick %d command count host=%d client=%d", tick, len(hostBatch.Commands), len(clientBatch.Commands))
		}
		if _, err := ApplyBatch(hostWorld, hostBatch, [2]bool{}); err != nil {
			t.Fatalf("apply host batch %d: %v", tick, err)
		}
		if _, err := ApplyBatch(clientWorld, clientBatch, [2]bool{}); err != nil {
			t.Fatalf("apply client batch %d: %v", tick, err)
		}
		if hostWorld.StateHash() != clientWorld.StateHash() {
			t.Fatalf("worlds diverged at tick %d", tick)
		}
	}
	if hostWorld.Score == clientWorld.Score || hostWorld.ScorePlayer == clientWorld.ScorePlayer {
		t.Fatal("test did not retain deliberately different local scores")
	}
	if hostWorld.Magnets[populous.DevilPlayer].Flags != populous.FightMode {
		t.Fatal("remote command was not applied")
	}
}

func TestHostOrdersPlayersThenSequence(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	hostPeer, _ := NewPeer(hostConnection)
	clientPeer, _ := NewPeer(clientConnection)
	defer hostPeer.Close()
	defer clientPeer.Close()

	host, err := NewHostSession(hostPeer, populous.GodPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClientSession(clientPeer, populous.DevilPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.SubmitAt(0, populous.Command{Kind: populous.CommandSetTendency, Player: populous.DevilPlayer, Value: populous.JoinMode}); err != nil {
		t.Fatal(err)
	}
	if _, err := host.SubmitLocal(populous.Command{Kind: populous.CommandSetTendency, Player: populous.GodPlayer, Value: populous.SettleMode}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-host.Activity():
	case <-time.After(2 * time.Second):
		t.Fatal("host did not receive remote intent")
	}

	batch, err := host.Advance(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Commands) != 2 {
		t.Fatalf("batch has %d commands, want 2", len(batch.Commands))
	}
	if batch.Commands[0].Command.Player != populous.GodPlayer || batch.Commands[1].Command.Player != populous.DevilPlayer {
		t.Fatalf("batch order = %d then %d", batch.Commands[0].Command.Player, batch.Commands[1].Command.Player)
	}
	_ = awaitBatch(t, client, 1)
}

func TestHostDoesNotLetLaterSequenceOvertakeAndRejectsRemotePaint(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	hostPeer, _ := NewPeer(hostConnection)
	clientPeer, _ := NewPeer(clientConnection)
	defer hostPeer.Close()
	defer clientPeer.Close()

	host, err := NewHostSession(hostPeer, populous.GodPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	first := Intent{
		RequestedTick: 10,
		Sequence:      1,
		Command:       populous.Command{Kind: populous.CommandSetTendency, Player: populous.DevilPlayer, Value: populous.JoinMode},
	}
	second := Intent{
		RequestedTick: 1,
		Sequence:      2,
		Command:       populous.Command{Kind: populous.CommandSetTendency, Player: populous.DevilPlayer, Value: populous.FightMode},
	}
	if err := host.scheduleRemote(first); err != nil {
		t.Fatal(err)
	}
	if err := host.scheduleRemote(second); err != nil {
		t.Fatal(err)
	}
	host.mu.Lock()
	commands := append([]SequencedCommand(nil), host.pending[10]...)
	host.mu.Unlock()
	if len(commands) != 2 || commands[0].Sequence != 1 || commands[1].Sequence != 2 {
		t.Fatalf("tick 10 commands = %+v, want sequences 1,2", commands)
	}

	paint := Intent{
		RequestedTick: 10,
		Sequence:      3,
		Command:       populous.Command{Kind: populous.CommandPaintRaise, Player: populous.DevilPlayer, X: 10, Y: 10},
	}
	if err := host.scheduleRemote(paint); !errors.Is(err, ErrPlayerOwnership) {
		t.Fatalf("remote paint error = %v, want ErrPlayerOwnership", err)
	}
}

func TestRestoreWorldPreservesCanonicalScoresForBothPlayers(t *testing.T) {
	hostWorld := testWorld()
	hostWorld.Magnets[populous.GodPlayer].Mana = 100000
	hostWorld.Magnets[populous.DevilPlayer].Mana = 100000
	initial := hostWorld.Scores
	if !hostWorld.QuakeAtTile(populous.GodPlayer, 10, 10) {
		t.Fatal("god quake was rejected")
	}
	if !hostWorld.SwampAtTile(populous.DevilPlayer, 20, 20) {
		t.Fatal("devil swamp was rejected")
	}
	want := [2]int{initial[populous.GodPlayer] + populous.ScoreQuake, initial[populous.DevilPlayer] + populous.ScoreSwamp}
	if hostWorld.Scores != want {
		t.Fatalf("host canonical scores = %v, want %v", hostWorld.Scores, want)
	}

	restored, err := RestoreWorld(hostWorld.Snapshot(), hostWorld.Rules, populous.DevilPlayer)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Scores != want {
		t.Fatalf("restored canonical scores = %v, want %v", restored.Scores, want)
	}
	if restored.Score != want[populous.DevilPlayer] || restored.ScorePlayer != populous.DevilPlayer {
		t.Fatalf("restored local score = %d/%d", restored.ScorePlayer, restored.Score)
	}
	if restored.StateHash() != hostWorld.StateHash() {
		t.Fatal("canonical-score restore changed shared state hash")
	}
}

func TestClientResyncDropsCoveredBatchesAndKeepsFuture(t *testing.T) {
	client := &ClientSession{
		currentTick:     0,
		highestReceived: 3,
		batches: map[uint64]CommandBatch{
			1: {Tick: 1},
			2: {Tick: 2},
			3: {Tick: 3},
		},
		ready: make(chan struct{}, 1),
	}
	signal(client.ready)
	covered, err := client.ResyncTo(2)
	if err != nil {
		t.Fatal(err)
	}
	if client.currentTick != 2 {
		t.Fatalf("current tick = %d, want 2", client.currentTick)
	}
	if _, ok := client.batches[1]; ok {
		t.Fatal("batch 1 survived resync")
	}
	if _, ok := client.batches[2]; ok {
		t.Fatal("batch 2 survived resync")
	}
	if _, ok := client.batches[3]; !ok {
		t.Fatal("future batch 3 was discarded")
	}
	if covered != [2]uint64{} {
		t.Fatalf("covered sequences = %v, want zero", covered)
	}
	select {
	case <-client.ready:
	default:
		t.Fatal("future batch did not restore ready signal")
	}
}

func TestClientResyncReturnsOnlyCoveredSequenceWatermarks(t *testing.T) {
	client := &ClientSession{
		currentTick:     1,
		highestReceived: 4,
		coveredSequence: [2]uint64{1, 0},
		batches: map[uint64]CommandBatch{
			2: {Tick: 2, Commands: []SequencedCommand{{Sequence: 1, Command: populous.Command{Kind: populous.CommandSetTendency, Player: populous.DevilPlayer, Value: populous.JoinMode}}}},
			3: {Tick: 3, Commands: []SequencedCommand{{Sequence: 2, Command: populous.Command{Kind: populous.CommandSetTendency, Player: populous.GodPlayer, Value: populous.FightMode}}}},
			4: {Tick: 4, Commands: []SequencedCommand{{Sequence: 2, Command: populous.Command{Kind: populous.CommandSetTendency, Player: populous.DevilPlayer, Value: populous.FightMode}}}},
		},
		ready: make(chan struct{}, 1),
	}
	covered, err := client.ResyncTo(3)
	if err != nil {
		t.Fatal(err)
	}
	if covered != [2]uint64{2, 1} {
		t.Fatalf("covered sequences = %v, want [2 1]", covered)
	}
	if _, ok := client.batches[4]; !ok {
		t.Fatal("future batch containing devil sequence 2 was discarded")
	}
}

func TestSessionsPingRoundTrip(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	hostPeer, _ := NewPeer(hostConnection)
	clientPeer, _ := NewPeer(clientConnection)
	defer hostPeer.Close()
	defer clientPeer.Close()

	host, err := NewHostSession(hostPeer, populous.GodPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClientSession(clientPeer, populous.DevilPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.SendPing(41, 1001); err != nil {
		t.Fatal(err)
	}
	assertPong := func(events <-chan WireMessage, nonce uint64, sent int64) {
		t.Helper()
		select {
		case message := <-events:
			pong, ok := message.(*Pong)
			if !ok || pong.Nonce != nonce || pong.SentUnixMillis != sent {
				t.Fatalf("pong = %#v, want nonce=%d sent=%d", message, nonce, sent)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for pong")
		}
	}
	assertPong(host.Events(), 41, 1001)

	if err := client.SendPing(42, 1002); err != nil {
		t.Fatal(err)
	}
	assertPong(client.Events(), 42, 1002)
}

func TestHostRejectsClientOnlyProtocolViolations(t *testing.T) {
	hostConnection, remoteConnection := net.Pipe()
	hostPeer, _ := NewPeer(hostConnection)
	remotePeer, _ := NewPeer(remoteConnection)
	defer hostPeer.Close()
	defer remotePeer.Close()

	host, err := NewHostSession(hostPeer, populous.GodPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := remotePeer.TrySend(CommandBatch{Tick: 1}); err != nil {
		t.Fatal(err)
	}
	assertSessionIssue(t, host.Issues(), ErrUnexpectedMessage)
}

func TestClientRejectsHostProtocolViolations(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	hostPeer, _ := NewPeer(hostConnection)
	clientPeer, _ := NewPeer(clientConnection)
	defer hostPeer.Close()
	defer clientPeer.Close()

	client, err := NewClientSession(clientPeer, populous.DevilPlayer, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	intent := Intent{
		RequestedTick: 1,
		Sequence:      1,
		Command:       populous.Command{Kind: populous.CommandSetTendency, Player: populous.GodPlayer, Value: populous.SettleMode},
	}
	if err := hostPeer.TrySend(intent); err != nil {
		t.Fatal(err)
	}
	assertSessionIssue(t, client.Issues(), ErrUnexpectedMessage)
}

func assertSessionIssue(t *testing.T, issues <-chan error, target error) {
	t.Helper()
	select {
	case err := <-issues:
		if !errors.Is(err, target) {
			t.Fatalf("session issue = %v, want %v", err, target)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %v", target)
	}
}

func awaitBatch(t *testing.T, client *ClientSession, tick uint64) CommandBatch {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		batch, ok, err := client.NextBatch(tick)
		if err != nil {
			t.Fatalf("NextBatch(%d): %v", tick, err)
		}
		if ok {
			return batch
		}
		select {
		case <-client.Ready():
		case issue := <-client.Issues():
			t.Fatalf("client issue: %v", issue)
		case <-deadline.C:
			t.Fatalf("timed out waiting for batch %d", tick)
		}
	}
}
