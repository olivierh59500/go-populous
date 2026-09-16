package multiplayer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const peerQueueSize = 64

type peerWriteRequest struct {
	message    WireMessage
	completion chan error
}

func (request peerWriteRequest) complete(err error) {
	if request.completion != nil {
		request.completion <- err
	}
}

// Peer owns one connection and serializes all writes through a dedicated
// goroutine. Incoming frames are decoded concurrently, so the Ebiten update
// loop never has to perform blocking socket I/O.
type Peer struct {
	connection net.Conn
	outgoing   chan peerWriteRequest
	incoming   chan WireMessage
	errors     chan error
	done       chan struct{}

	stopOnce sync.Once
	wait     sync.WaitGroup
}

func NewPeer(connection net.Conn) (*Peer, error) {
	if connection == nil {
		return nil, fmt.Errorf("new multiplayer peer: nil connection")
	}
	peer := &Peer{
		connection: connection,
		outgoing:   make(chan peerWriteRequest, peerQueueSize),
		incoming:   make(chan WireMessage, peerQueueSize),
		errors:     make(chan error, 1),
		done:       make(chan struct{}),
	}
	peer.wait.Add(2)
	go peer.readLoop()
	go peer.writeLoop()
	go func() {
		peer.wait.Wait()
		close(peer.errors)
	}()
	return peer, nil
}

func (peer *Peer) Incoming() <-chan WireMessage { return peer.incoming }
func (peer *Peer) Errors() <-chan error         { return peer.errors }
func (peer *Peer) Done() <-chan struct{}        { return peer.done }

// Send queues a validated message, waiting only for queue capacity or context
// cancellation. Network writes happen on the peer's writer goroutine.
func (peer *Peer) Send(ctx context.Context, message WireMessage) error {
	if peer == nil {
		return ErrSessionClosed
	}
	if err := validateMessage(message); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-peer.done:
		return ErrSessionClosed
	default:
	}
	select {
	case peer.outgoing <- peerWriteRequest{message: message}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-peer.done:
		return ErrSessionClosed
	}
}

// SendAndWait queues a validated message and waits until the writer has
// completely written its frame. If the context expires after the request was
// queued, the peer is closed: a partially written framed stream cannot be
// reused safely.
func (peer *Peer) SendAndWait(ctx context.Context, message WireMessage) error {
	if peer == nil {
		return ErrSessionClosed
	}
	if err := validateMessage(message); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-peer.done:
		return ErrSessionClosed
	default:
	}

	completion := make(chan error, 1)
	request := peerWriteRequest{message: message, completion: completion}
	select {
	case peer.outgoing <- request:
	case <-ctx.Done():
		return ctx.Err()
	case <-peer.done:
		return ErrSessionClosed
	}

	select {
	case err := <-completion:
		return err
	case <-ctx.Done():
		select {
		case err := <-completion:
			return err
		default:
		}
		peer.stop(nil)
		return ctx.Err()
	case <-peer.done:
		select {
		case err := <-completion:
			return err
		default:
			return ErrSessionClosed
		}
	}
}

// TrySend is the non-blocking counterpart used from a real-time update loop.
func (peer *Peer) TrySend(message WireMessage) error {
	if peer == nil {
		return ErrSessionClosed
	}
	if err := validateMessage(message); err != nil {
		return err
	}
	select {
	case <-peer.done:
		return ErrSessionClosed
	default:
	}
	select {
	case peer.outgoing <- peerWriteRequest{message: message}:
		return nil
	case <-peer.done:
		return ErrSessionClosed
	default:
		return ErrSendQueueFull
	}
}

func (peer *Peer) Close() error {
	if peer == nil {
		return nil
	}
	peer.stop(nil)
	peer.wait.Wait()
	return nil
}

func (peer *Peer) readLoop() {
	defer peer.wait.Done()
	defer close(peer.incoming)
	for {
		message, err := ReadMessage(peer.connection)
		if err != nil {
			peer.stop(peerError("read", err))
			return
		}
		select {
		case peer.incoming <- message:
		case <-peer.done:
			return
		}
	}
}

func (peer *Peer) writeLoop() {
	defer peer.wait.Done()
	for {
		select {
		case request := <-peer.outgoing:
			if err := WriteMessage(peer.connection, request.message); err != nil {
				writeErr := peerError("write", err)
				request.complete(writeErr)
				peer.stop(writeErr)
				return
			}
			request.complete(nil)
		case <-peer.done:
			return
		}
	}
}

func (peer *Peer) stop(err error) {
	peer.stopOnce.Do(func() {
		if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			select {
			case peer.errors <- err:
			default:
			}
		}
		close(peer.done)
		_ = peer.connection.Close()
	})
}

func peerError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("multiplayer %s: %w", operation, err)
}
