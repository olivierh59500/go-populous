package multiplayer

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"go-populous/internal/populous"
)

type disconnectSender interface {
	SendDisconnect(context.Context, string) error
	Close() error
}

func TestSessionDisconnectIsReceivedBeforeEOF(t *testing.T) {
	tests := []struct {
		name string
		new  func(*Peer) (disconnectSender, error)
	}{
		{
			name: "host",
			new: func(peer *Peer) (disconnectSender, error) {
				return NewHostSession(peer, populous.GodPlayer, 0, DefaultInputDelay)
			},
		},
		{
			name: "client",
			new: func(peer *Peer) (disconnectSender, error) {
				return NewClientSession(peer, populous.DevilPlayer, 0, DefaultInputDelay)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			localConnection, remoteConnection := net.Pipe()
			defer remoteConnection.Close()
			peer, err := NewPeer(localConnection)
			if err != nil {
				t.Fatal(err)
			}
			session, err := test.new(peer)
			if err != nil {
				_ = peer.Close()
				t.Fatal(err)
			}

			type readResult struct {
				message WireMessage
				readErr error
				eofErr  error
			}
			read := make(chan readResult, 1)
			go func() {
				message, readErr := ReadMessage(remoteConnection)
				var eofErr error
				if readErr == nil {
					_, eofErr = ReadMessage(remoteConnection)
				}
				read <- readResult{message: message, readErr: readErr, eofErr: eofErr}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err = session.SendDisconnect(ctx, "leaving")
			cancel()
			if err != nil {
				_ = session.Close()
				t.Fatalf("SendDisconnect: %v", err)
			}
			if err := session.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			select {
			case result := <-read:
				if result.readErr != nil {
					t.Fatalf("read disconnect: %v", result.readErr)
				}
				disconnect, ok := result.message.(*Disconnect)
				if !ok || disconnect.Reason != "leaving" {
					t.Fatalf("first message = %#v, want disconnect", result.message)
				}
				if !errors.Is(result.eofErr, io.EOF) {
					t.Fatalf("read after disconnect = %v, want EOF", result.eofErr)
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for disconnect and EOF")
			}
		})
	}
}

func TestSendAndWaitCancellationStopsBlockedWriter(t *testing.T) {
	localConnection, remoteConnection := net.Pipe()
	defer remoteConnection.Close()
	peer, err := NewPeer(localConnection)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- peer.SendAndWait(ctx, Disconnect{Reason: "blocked"})
	}()

	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("SendAndWait error = %v, want context deadline", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SendAndWait did not honor cancellation")
	}
	select {
	case <-peer.Done():
	case <-time.After(time.Second):
		t.Fatal("cancelled SendAndWait did not close the peer")
	}

	closed := make(chan error, 1)
	go func() {
		closed <- peer.Close()
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Peer.Close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Peer.Close leaked a blocked writer")
	}
	for err := range peer.Errors() {
		t.Fatalf("cancelled local write reported a transport error: %v", err)
	}
}
