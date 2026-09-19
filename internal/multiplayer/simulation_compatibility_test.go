package multiplayer

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// A simulation revision must reject the older executable before either peer
// begins lockstep. Packet schemas alone cannot detect changed world rules.
func TestSimulationBuildMismatchRejectsBothDirections(t *testing.T) {
	for _, ids := range [][2]string{
		{"go-populous-lockstep-2", "go-populous-lockstep-1"},
		{"go-populous-lockstep-1", "go-populous-lockstep-2"},
		{"go-populous-lockstep-3", "go-populous-lockstep-2"},
		{"go-populous-lockstep-2", "go-populous-lockstep-3"},
		{"go-populous-lockstep-3", "go-populous-lockstep-1"},
	} {
		t.Run(ids[0]+"_vs_"+ids[1], func(t *testing.T) {
			hostConn, clientConn := net.Pipe()
			defer hostConn.Close()
			defer clientConn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			config := testHostConnectionConfig(time.Second)
			config.Handshake.BuildID = ids[0]
			hostResults := AcceptHostConnection(ctx, hostConn, config)
			clientResults := JoinClientConnection(ctx, clientConn, ClientConnectionConfig{
				Hello: NewHello(ids[1], "different simulation", 1), Timeout: time.Second,
			})
			host := awaitHostConnectResult(t, hostResults)
			client := awaitClientConnectResult(t, clientResults)
			defer host.Close()
			defer client.Close()
			if !errors.Is(host.Err, ErrBuildMismatch) || client.Err == nil {
				t.Fatalf("incompatible simulation accepted: host=%v client=%v", host.Err, client.Err)
			}
			if host.Session != nil || client.Session != nil || host.Peer != nil || client.Peer != nil {
				t.Fatal("incompatible builds started a multiplayer session")
			}
		})
	}
}
