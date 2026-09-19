package multiplayer

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"go-populous/internal/populous"
)

func TestListenHostTCPAndDialClientTCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	hostConfig := testHostConnectionConfig(2 * time.Second)
	host, err := ListenHostTCP(ctx, "127.0.0.1:0", hostConfig)
	if err != nil {
		skipIfLocalhostUnavailable(t, err)
		t.Fatalf("ListenHostTCP: %v", err)
	}
	defer host.Close()

	clientResults := DialClientTCP(ctx, host.Addr().String(), ClientConnectionConfig{
		Hello:   NewHello(hostConfig.Handshake.BuildID, "remote", populous.DevilPlayer),
		Timeout: 2 * time.Second,
	})
	hostResult := awaitHostConnectResult(t, host.Results())
	clientResult := awaitClientConnectResult(t, clientResults)
	if hostResult.Err != nil {
		t.Fatalf("host connection: %v", hostResult.Err)
	}
	if clientResult.Err != nil {
		t.Fatalf("client connection: %v", clientResult.Err)
	}
	defer hostResult.Close()
	defer clientResult.Close()

	if hostResult.Peer == nil || hostResult.Session == nil {
		t.Fatal("host did not construct peer and session")
	}
	if clientResult.Peer == nil || clientResult.Session == nil {
		t.Fatal("client did not construct peer and session")
	}
	if hostResult.Hello.Name != "remote" {
		t.Fatalf("host received name %q, want remote", hostResult.Hello.Name)
	}
	if clientResult.Welcome.SessionID != hostConfig.Handshake.SessionID {
		t.Fatalf("client session ID = %d, want %d", clientResult.Welcome.SessionID, hostConfig.Handshake.SessionID)
	}
	if clientResult.Session.CurrentTick() != hostConfig.Handshake.Start.Tick || hostResult.Session.CurrentTick() != hostConfig.Handshake.Start.Tick {
		t.Fatal("sessions did not start at the authoritative tick")
	}

	select {
	case _, open := <-host.Results():
		if open {
			t.Fatal("host result channel produced more than one result")
		}
	case <-time.After(time.Second):
		t.Fatal("host result channel was not closed")
	}
	connection, err := net.DialTimeout("tcp", host.Addr().String(), 100*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		t.Fatal("one-peer listener remained open after accepting a client")
	}
}

func TestConnectionSetupOverNetPipe(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hostConfig := testHostConnectionConfig(time.Second)
	hostConfig.TransportPreamble = []byte("local-proxy-key")
	hostResults := AcceptHostConnection(ctx, hostConnection, hostConfig)
	clientResults := JoinClientConnection(ctx, clientConnection, ClientConnectionConfig{
		Hello:             NewHello(hostConfig.Handshake.BuildID, "pipe", populous.DevilPlayer),
		Timeout:           time.Second,
		TransportPreamble: []byte("local-proxy-key"),
	})
	hostResult := awaitHostConnectResult(t, hostResults)
	clientResult := awaitClientConnectResult(t, clientResults)
	if hostResult.Err != nil {
		t.Fatalf("pipe host connection: %v", hostResult.Err)
	}
	if clientResult.Err != nil {
		t.Fatalf("pipe client connection: %v", clientResult.Err)
	}
	if hostResult.Session.localPlayer != populous.GodPlayer {
		t.Fatalf("host local player = %d, want god", hostResult.Session.localPlayer)
	}
	if clientResult.Session.player != populous.DevilPlayer {
		t.Fatalf("client player = %d, want devil", clientResult.Session.player)
	}
	if err := clientResult.Close(); err != nil {
		t.Fatalf("close client result: %v", err)
	}
	if err := hostResult.Close(); err != nil {
		t.Fatalf("close host result: %v", err)
	}
}

func TestConnectionRejectsWrongTransportPreamble(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	hostConfig := testHostConnectionConfig(time.Second)
	hostConfig.TransportPreamble = []byte("expected-preamble")
	hostResults := AcceptHostConnection(ctx, hostConnection, hostConfig)
	clientResults := JoinClientConnection(ctx, clientConnection, ClientConnectionConfig{
		Hello:             NewHello(hostConfig.Handshake.BuildID, "pipe", populous.DevilPlayer),
		Timeout:           time.Second,
		TransportPreamble: []byte("incorrect-preamble"),
	})
	if result := awaitHostConnectResult(t, hostResults); result.Err == nil {
		t.Fatal("host accepted an incorrect transport preamble")
	}
	if result := awaitClientConnectResult(t, clientResults); result.Err == nil {
		t.Fatal("client unexpectedly completed after preamble rejection")
	}
}

func TestConnectionTransportPreambleTimeout(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	defer clientConnection.Close()
	hostConfig := testHostConnectionConfig(50 * time.Millisecond)
	hostConfig.TransportPreamble = []byte("expected-preamble")
	result := awaitHostConnectResult(t, AcceptHostConnection(context.Background(), hostConnection, hostConfig))
	if !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("preamble timeout error = %v, want context.DeadlineExceeded", result.Err)
	}
}

func TestTransportPreambleContextCapsAndPreservesDeadline(t *testing.T) {
	started := time.Now()
	longParent, cancelLong := context.WithTimeout(context.Background(), time.Hour)
	defer cancelLong()
	capped, cancelCapped := transportPreambleContext(longParent)
	defer cancelCapped()
	cappedDeadline, ok := capped.Deadline()
	if !ok {
		t.Fatal("capped preamble context has no deadline")
	}
	remaining := cappedDeadline.Sub(started)
	if remaining < transportPreambleTimeout-100*time.Millisecond || remaining > transportPreambleTimeout+100*time.Millisecond {
		t.Fatalf("preamble deadline = %s, want about %s", remaining, transportPreambleTimeout)
	}

	parentDeadline := time.Now().Add(time.Second)
	shortParent, cancelShort := context.WithDeadline(context.Background(), parentDeadline)
	defer cancelShort()
	shorter, cancelShorter := transportPreambleContext(shortParent)
	defer cancelShorter()
	shortDeadline, ok := shorter.Deadline()
	if !ok || !shortDeadline.Equal(parentDeadline) {
		t.Fatalf("short parent deadline = %v, %t; want %v", shortDeadline, ok, parentDeadline)
	}
}

func TestConnectionRejectsOversizedTransportPreamble(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	defer clientConnection.Close()
	hostConfig := testHostConnectionConfig(time.Second)
	hostConfig.TransportPreamble = make([]byte, 65)
	result := awaitHostConnectResult(t, AcceptHostConnection(context.Background(), hostConnection, hostConfig))
	if result.Err == nil {
		t.Fatal("accepted a transport preamble longer than 64 bytes")
	}
}

func TestAcceptHostConnectionCancellationClosesPipe(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	defer clientConnection.Close()

	ctx, cancel := context.WithCancel(context.Background())
	results := AcceptHostConnection(ctx, hostConnection, testHostConnectionConfig(time.Second))
	cancel()
	result := awaitHostConnectResult(t, results)
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("host cancellation error = %v, want context.Canceled", result.Err)
	}

	if err := clientConnection.SetReadDeadline(time.Now().Add(time.Second)); err == nil {
		var buffer [1]byte
		if _, err := clientConnection.Read(buffer[:]); err == nil {
			t.Fatal("cancelled host left its pipe connection open")
		}
	}
	select {
	case _, open := <-results:
		if open {
			t.Fatal("cancelled host produced more than one result")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled host result channel was not closed")
	}
}

func TestHostListenerCloseCancelsPendingAccept(t *testing.T) {
	host, err := ListenHostTCP(context.Background(), "127.0.0.1:0", testHostConnectionConfig(5*time.Second))
	if err != nil {
		skipIfLocalhostUnavailable(t, err)
		t.Fatalf("ListenHostTCP: %v", err)
	}
	results := host.Results()
	closed := make(chan error, 1)
	go func() {
		closed <- host.Close()
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("HostListener.Close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("HostListener.Close blocked on pending accept")
	}
	result := awaitHostConnectResult(t, results)
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("pending accept error = %v, want context.Canceled", result.Err)
	}
}

func skipIfLocalhostUnavailable(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		t.Skipf("localhost sockets unavailable in test sandbox: %v", err)
	}
}

func testHostConnectionConfig(timeout time.Duration) HostConnectionConfig {
	world := testWorld()
	return HostConnectionConfig{
		Handshake: HostHandshakeConfig{
			BuildID:        "connection-test",
			SessionID:      101,
			AssignedPlayer: populous.DevilPlayer,
			TickRate:       DefaultTickRate,
			InputDelay:     DefaultInputDelay,
			Start: Start{
				Tick:     uint64(world.GameTurn),
				Snapshot: world.Snapshot(),
				Rules:    world.Rules,
			},
		},
		Timeout: timeout,
	}
}

func awaitHostConnectResult(t *testing.T, results <-chan HostConnectResult) HostConnectResult {
	t.Helper()
	select {
	case result, open := <-results:
		if !open {
			t.Fatal("host result channel closed without a result")
		}
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for host connection result")
		return HostConnectResult{}
	}
}

func awaitClientConnectResult(t *testing.T, results <-chan ClientConnectResult) ClientConnectResult {
	t.Helper()
	select {
	case result, open := <-results:
		if !open {
			t.Fatal("client result channel closed without a result")
		}
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for client connection result")
		return ClientConnectResult{}
	}
}
