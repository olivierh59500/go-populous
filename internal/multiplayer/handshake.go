package multiplayer

import (
	"context"
	"fmt"
	"net"
	"time"

	"go-populous/internal/populous"
)

// HostHandshakeConfig is the authoritative session metadata and initial state
// sent by a host after accepting a compatible Hello.
type HostHandshakeConfig struct {
	BuildID        string
	SessionID      uint64
	AssignedPlayer int
	TickRate       uint16
	InputDelay     uint64
	Start          Start
}

// AcceptHandshake performs the host half of the synchronous handshake. Call it
// before constructing a Peer, since both operations consume the same stream.
func AcceptHandshake(ctx context.Context, connection net.Conn, config HostHandshakeConfig) (Hello, error) {
	if connection == nil {
		return Hello{}, fmt.Errorf("accept handshake: nil connection")
	}
	if err := validateHostHandshakeConfig(config); err != nil {
		return Hello{}, err
	}
	cleanup, err := applyContextDeadline(ctx, connection)
	if err != nil {
		return Hello{}, err
	}
	defer cleanup()

	message, err := ReadMessage(connection)
	if err != nil {
		return Hello{}, fmt.Errorf("read hello: %w", err)
	}
	hello, ok := message.(*Hello)
	if !ok {
		_ = WriteMessage(connection, ErrorMessage{Code: "expected_hello", Message: "first message must be hello"})
		return Hello{}, fmt.Errorf("%w: got %s, want hello", ErrUnexpectedMessage, message.messageType())
	}
	if config.BuildID != "" && hello.BuildID != config.BuildID {
		_ = WriteMessage(connection, ErrorMessage{Code: "build_mismatch", Message: "host and client builds differ"})
		return Hello{}, fmt.Errorf("%w: client %q, host %q", ErrBuildMismatch, hello.BuildID, config.BuildID)
	}

	welcome := Welcome{
		ProtocolVersion:  ProtocolVersion,
		StateHashVersion: populous.StateHashVersion,
		BuildID:          config.BuildID,
		SessionID:        config.SessionID,
		AssignedPlayer:   config.AssignedPlayer,
		TickRate:         config.TickRate,
		InputDelay:       config.InputDelay,
		CurrentTick:      config.Start.Tick,
	}
	if err := WriteMessage(connection, welcome); err != nil {
		return Hello{}, fmt.Errorf("write welcome: %w", err)
	}
	if err := WriteMessage(connection, config.Start); err != nil {
		return Hello{}, fmt.Errorf("write start: %w", err)
	}
	return *hello, nil
}

// JoinHandshake performs the client half of the synchronous handshake and
// returns the host-selected session settings and canonical initial state.
func JoinHandshake(ctx context.Context, connection net.Conn, hello Hello) (Welcome, Start, error) {
	if connection == nil {
		return Welcome{}, Start{}, fmt.Errorf("join handshake: nil connection")
	}
	if err := validateMessage(hello); err != nil {
		return Welcome{}, Start{}, err
	}
	cleanup, err := applyContextDeadline(ctx, connection)
	if err != nil {
		return Welcome{}, Start{}, err
	}
	defer cleanup()

	if err := WriteMessage(connection, hello); err != nil {
		return Welcome{}, Start{}, fmt.Errorf("write hello: %w", err)
	}
	message, err := ReadMessage(connection)
	if err != nil {
		return Welcome{}, Start{}, fmt.Errorf("read welcome: %w", err)
	}
	if remoteError, ok := message.(*ErrorMessage); ok {
		return Welcome{}, Start{}, fmt.Errorf("remote rejected handshake (%s): %s", remoteError.Code, remoteError.Message)
	}
	welcome, ok := message.(*Welcome)
	if !ok {
		return Welcome{}, Start{}, fmt.Errorf("%w: got %s, want welcome", ErrUnexpectedMessage, message.messageType())
	}
	if hello.BuildID != "" && welcome.BuildID != hello.BuildID {
		return Welcome{}, Start{}, fmt.Errorf("%w: host %q, client %q", ErrBuildMismatch, welcome.BuildID, hello.BuildID)
	}

	message, err = ReadMessage(connection)
	if err != nil {
		return Welcome{}, Start{}, fmt.Errorf("read start: %w", err)
	}
	if remoteError, ok := message.(*ErrorMessage); ok {
		return Welcome{}, Start{}, fmt.Errorf("remote aborted handshake (%s): %s", remoteError.Code, remoteError.Message)
	}
	start, ok := message.(*Start)
	if !ok {
		return Welcome{}, Start{}, fmt.Errorf("%w: got %s, want start", ErrUnexpectedMessage, message.messageType())
	}
	if welcome.CurrentTick != start.Tick {
		return Welcome{}, Start{}, fmt.Errorf("%w: welcome tick %d, start tick %d", ErrInvalidMessage, welcome.CurrentTick, start.Tick)
	}
	return *welcome, *start, nil
}

func validateHostHandshakeConfig(config HostHandshakeConfig) error {
	if config.AssignedPlayer != populous.GodPlayer && config.AssignedPlayer != populous.DevilPlayer {
		return fmt.Errorf("%w: assigned player %d", ErrInvalidMessage, config.AssignedPlayer)
	}
	if config.TickRate != DefaultTickRate {
		return fmt.Errorf("%w: tick rate %d, want %d", ErrInvalidMessage, config.TickRate, DefaultTickRate)
	}
	if config.InputDelay > MaxFutureTicks {
		return fmt.Errorf("%w: input delay %d", ErrInvalidMessage, config.InputDelay)
	}
	if err := validateText("build ID", config.BuildID, 128); err != nil {
		return err
	}
	return validateStart(config.Start)
}

func applyContextDeadline(ctx context.Context, connection net.Conn) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set handshake deadline: %w", err)
		}
	}

	// Context cancellation must also wake a socket operation when the context
	// has no deadline (or is cancelled before its deadline). Wait for a callback
	// already in flight before clearing the deadline, otherwise an expired
	// deadline could leak into the Peer created immediately after the handshake.
	callbackDone := make(chan struct{})
	stopCancellation := context.AfterFunc(ctx, func() {
		_ = connection.SetDeadline(time.Now())
		close(callbackDone)
	})
	return func() {
		if !stopCancellation() {
			<-callbackDone
		}
		_ = connection.SetDeadline(time.Time{})
	}, nil
}
