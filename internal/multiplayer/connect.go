package multiplayer

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

const DefaultConnectionTimeout = 15 * time.Second

// HostConnectionConfig controls one host-side connection attempt. A zero
// Timeout uses DefaultConnectionTimeout. AssignedPlayer in Handshake is the
// remote player's side; the host session owns the opposite side.
type HostConnectionConfig struct {
	Handshake HostHandshakeConfig
	Timeout   time.Duration
}

// ClientConnectionConfig controls one client-side connection attempt. A zero
// Timeout uses DefaultConnectionTimeout.
type ClientConnectionConfig struct {
	Hello   Hello
	Timeout time.Duration
}

// HostConnectResult is delivered exactly once by a host connection operation.
// On success Peer and Session share the accepted connection; consume network
// events through Session, and call Close when the match ends.
type HostConnectResult struct {
	Hello   Hello
	Peer    *Peer
	Session *HostSession
	Err     error
}

func (result HostConnectResult) Close() error {
	if result.Session != nil {
		return result.Session.Close()
	}
	if result.Peer != nil {
		return result.Peer.Close()
	}
	return nil
}

// ClientConnectResult is delivered exactly once by a client connection
// operation. Welcome and Start contain the authoritative session metadata and
// initial world received during the handshake.
type ClientConnectResult struct {
	Welcome Welcome
	Start   Start
	Peer    *Peer
	Session *ClientSession
	Err     error
}

func (result ClientConnectResult) Close() error {
	if result.Session != nil {
		return result.Session.Close()
	}
	if result.Peer != nil {
		return result.Peer.Close()
	}
	return nil
}

// HostListener accepts at most one TCP peer. Results is buffered, so accepting
// and handshaking never wait for the caller to poll it. Close cancels a pending
// accept or handshake and waits for its connection to be released. Once a
// successful result is delivered, ownership of the connection moves to it.
type HostListener struct {
	listener net.Listener
	address  net.Addr
	results  chan HostConnectResult
	done     chan struct{}
	cancel   context.CancelFunc

	closeOnce sync.Once
}

func (host *HostListener) Addr() net.Addr {
	if host == nil {
		return nil
	}
	return host.address
}

func (host *HostListener) Results() <-chan HostConnectResult {
	if host == nil {
		return nil
	}
	return host.results
}

func (host *HostListener) Done() <-chan struct{} {
	if host == nil {
		return nil
	}
	return host.done
}

func (host *HostListener) Close() error {
	if host == nil {
		return nil
	}
	host.closeOnce.Do(func() {
		host.cancel()
		_ = host.listener.Close()
		<-host.done
	})
	return nil
}

// ListenHostTCP starts a one-peer TCP listener. Binding errors are returned
// immediately; accept, handshake and session construction complete through
// HostListener.Results.
func ListenHostTCP(ctx context.Context, address string, config HostConnectionConfig) (*HostListener, error) {
	if err := validateHostConnectionConfig(config); err != nil {
		return nil, err
	}
	ctx = connectionContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen for multiplayer peer: %w", err)
	}
	operationContext, cancel := context.WithTimeout(ctx, connectionTimeout(config.Timeout))
	host := &HostListener{
		listener: listener,
		address:  listener.Addr(),
		results:  make(chan HostConnectResult, 1),
		done:     make(chan struct{}),
		cancel:   cancel,
	}
	go host.accept(operationContext, config)
	return host, nil
}

func (host *HostListener) accept(ctx context.Context, config HostConnectionConfig) {
	defer host.cancel()
	defer close(host.done)
	defer close(host.results)

	stopCancellation := context.AfterFunc(ctx, func() {
		_ = host.listener.Close()
	})
	connection, err := host.listener.Accept()
	_ = host.listener.Close() // This listener intentionally serves one peer.
	_ = stopCancellation()
	if err != nil {
		host.results <- HostConnectResult{Err: connectionOperationError(ctx, "accept multiplayer peer", err)}
		return
	}
	host.results <- establishHostConnection(ctx, connection, config)
}

// DialClientTCP asynchronously dials a host, completes the handshake, and
// constructs a Peer and ClientSession. The returned buffered channel receives
// exactly one result and is then closed.
func DialClientTCP(ctx context.Context, address string, config ClientConnectionConfig) <-chan ClientConnectResult {
	results := make(chan ClientConnectResult, 1)
	if err := validateClientConnectionConfig(config); err != nil {
		results <- ClientConnectResult{Err: err}
		close(results)
		return results
	}
	operationContext, cancel := context.WithTimeout(connectionContext(ctx), connectionTimeout(config.Timeout))
	go func() {
		defer cancel()
		defer close(results)
		connection, err := (&net.Dialer{}).DialContext(operationContext, "tcp", address)
		if err != nil {
			results <- ClientConnectResult{Err: connectionOperationError(operationContext, "dial multiplayer host", err)}
			return
		}
		results <- establishClientConnection(operationContext, connection, config)
	}()
	return results
}

// AcceptHostConnection performs the asynchronous host setup on an already
// accepted connection, which it owns from this call onward. It is useful for
// custom transports and net.Pipe tests.
func AcceptHostConnection(ctx context.Context, connection net.Conn, config HostConnectionConfig) <-chan HostConnectResult {
	results := make(chan HostConnectResult, 1)
	if err := validateHostConnectionConfig(config); err != nil {
		if connection != nil {
			_ = connection.Close()
		}
		results <- HostConnectResult{Err: err}
		close(results)
		return results
	}
	operationContext, cancel := context.WithTimeout(connectionContext(ctx), connectionTimeout(config.Timeout))
	go func() {
		defer cancel()
		defer close(results)
		results <- establishHostConnection(operationContext, connection, config)
	}()
	return results
}

// JoinClientConnection performs the asynchronous client setup on an already
// connected stream, which it owns from this call onward.
func JoinClientConnection(ctx context.Context, connection net.Conn, config ClientConnectionConfig) <-chan ClientConnectResult {
	results := make(chan ClientConnectResult, 1)
	if err := validateClientConnectionConfig(config); err != nil {
		if connection != nil {
			_ = connection.Close()
		}
		results <- ClientConnectResult{Err: err}
		close(results)
		return results
	}
	operationContext, cancel := context.WithTimeout(connectionContext(ctx), connectionTimeout(config.Timeout))
	go func() {
		defer cancel()
		defer close(results)
		results <- establishClientConnection(operationContext, connection, config)
	}()
	return results
}

func establishHostConnection(ctx context.Context, connection net.Conn, config HostConnectionConfig) HostConnectResult {
	if connection == nil {
		return HostConnectResult{Err: fmt.Errorf("accept multiplayer peer: nil connection")}
	}
	owned := true
	defer func() {
		if owned {
			_ = connection.Close()
		}
	}()
	if err := configureTCPConnection(connection); err != nil {
		return HostConnectResult{Err: err}
	}

	hello, err := AcceptHandshake(ctx, connection, config.Handshake)
	if err != nil {
		return HostConnectResult{Err: connectionOperationError(ctx, "host multiplayer handshake", err)}
	}
	if err := ctx.Err(); err != nil {
		return HostConnectResult{Err: fmt.Errorf("host multiplayer handshake: %w", err)}
	}
	peer, err := NewPeer(connection)
	if err != nil {
		return HostConnectResult{Err: err}
	}
	session, err := NewHostSession(peer, config.Handshake.AssignedPlayer^1, config.Handshake.Start.Tick, config.Handshake.InputDelay)
	if err != nil {
		_ = peer.Close()
		owned = false
		return HostConnectResult{Err: err}
	}
	owned = false
	return HostConnectResult{Hello: hello, Peer: peer, Session: session}
}

func establishClientConnection(ctx context.Context, connection net.Conn, config ClientConnectionConfig) ClientConnectResult {
	if connection == nil {
		return ClientConnectResult{Err: fmt.Errorf("join multiplayer host: nil connection")}
	}
	owned := true
	defer func() {
		if owned {
			_ = connection.Close()
		}
	}()
	if err := configureTCPConnection(connection); err != nil {
		return ClientConnectResult{Err: err}
	}

	welcome, start, err := JoinHandshake(ctx, connection, config.Hello)
	if err != nil {
		return ClientConnectResult{Err: connectionOperationError(ctx, "client multiplayer handshake", err)}
	}
	if err := ctx.Err(); err != nil {
		return ClientConnectResult{Err: fmt.Errorf("client multiplayer handshake: %w", err)}
	}
	peer, err := NewPeer(connection)
	if err != nil {
		return ClientConnectResult{Err: err}
	}
	session, err := NewClientSession(peer, welcome.AssignedPlayer, start.Tick, welcome.InputDelay)
	if err != nil {
		_ = peer.Close()
		owned = false
		return ClientConnectResult{Err: err}
	}
	owned = false
	return ClientConnectResult{Welcome: welcome, Start: start, Peer: peer, Session: session}
}

func configureTCPConnection(connection net.Conn) error {
	tcp, ok := connection.(*net.TCPConn)
	if !ok {
		return nil
	}
	if err := tcp.SetNoDelay(true); err != nil {
		return fmt.Errorf("configure multiplayer TCP no-delay: %w", err)
	}
	if err := tcp.SetKeepAlive(true); err != nil {
		return fmt.Errorf("configure multiplayer TCP keepalive: %w", err)
	}
	if err := tcp.SetKeepAlivePeriod(30 * time.Second); err != nil {
		return fmt.Errorf("configure multiplayer TCP keepalive period: %w", err)
	}
	return nil
}

func validateHostConnectionConfig(config HostConnectionConfig) error {
	if config.Timeout < 0 {
		return fmt.Errorf("invalid multiplayer connection timeout %s", config.Timeout)
	}
	return validateHostHandshakeConfig(config.Handshake)
}

func validateClientConnectionConfig(config ClientConnectionConfig) error {
	if config.Timeout < 0 {
		return fmt.Errorf("invalid multiplayer connection timeout %s", config.Timeout)
	}
	return validateMessage(config.Hello)
}

func connectionContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func connectionTimeout(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return DefaultConnectionTimeout
	}
	return timeout
}

func connectionOperationError(ctx context.Context, operation string, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return fmt.Errorf("%s: %w", operation, contextErr)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
