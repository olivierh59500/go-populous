package multiplayer

import "context"

// SendDisconnect flushes a disconnect notice to the remote peer. Call Close
// after it returns to finish the graceful shutdown.
func (session *HostSession) SendDisconnect(ctx context.Context, reason string) error {
	if session == nil || session.peer == nil {
		return ErrSessionClosed
	}
	return session.peer.SendAndWait(ctx, Disconnect{Reason: reason})
}

// SendDisconnect flushes a disconnect notice to the host. Call Close after it
// returns to finish the graceful shutdown.
func (session *ClientSession) SendDisconnect(ctx context.Context, reason string) error {
	if session == nil || session.peer == nil {
		return ErrSessionClosed
	}
	return session.peer.SendAndWait(ctx, Disconnect{Reason: reason})
}
