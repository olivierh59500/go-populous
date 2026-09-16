package multiplayer

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"go-populous/internal/populous"
)

func testWorld() *populous.World {
	return populous.GenerateWorld(populous.Level{
		Number:             9,
		Code:               "NETTEST",
		SeedOffset:         0x1234,
		PlayerPopulation:   3,
		EnemyPopulation:    3,
		PlayerPowers:       0x3f,
		EnemyPowers:        0x3f,
		EnemyRating:        3,
		EnemyReactionSpeed: 4,
	})
}

func TestFrameRoundTrip(t *testing.T) {
	world := testWorld()
	tests := []WireMessage{
		NewHello("build-1", "player", populous.DevilPlayer),
		Welcome{
			ProtocolVersion:  ProtocolVersion,
			StateHashVersion: populous.StateHashVersion,
			BuildID:          "build-1",
			SessionID:        42,
			AssignedPlayer:   populous.DevilPlayer,
			TickRate:         DefaultTickRate,
			InputDelay:       DefaultInputDelay,
		},
		Start{Tick: 0, Snapshot: world.Snapshot(), Rules: world.Rules},
		Intent{RequestedTick: 2, Sequence: 1, Command: populous.Command{Kind: populous.CommandSetTendency, Player: populous.DevilPlayer, Value: populous.FightMode}},
		CommandBatch{Tick: 1},
		StateHash{Tick: 1, Hash: world.StateHash()},
		Snapshot{Tick: 0, Snapshot: world.Snapshot(), Rules: world.Rules},
		ErrorMessage{Code: "test", Message: "message"},
		Ping{Nonce: 5, SentUnixMillis: 123},
		Pong{Nonce: 5, SentUnixMillis: 123},
		Disconnect{Reason: "done"},
	}

	for _, want := range tests {
		t.Run(want.messageType().String(), func(t *testing.T) {
			var buffer bytes.Buffer
			if err := WriteMessage(&buffer, want); err != nil {
				t.Fatalf("WriteMessage: %v", err)
			}
			got, err := ReadMessage(&buffer)
			if err != nil {
				t.Fatalf("ReadMessage: %v", err)
			}
			wantValue := reflect.ValueOf(want)
			if wantValue.Kind() != reflect.Pointer {
				pointer := reflect.New(wantValue.Type())
				pointer.Elem().Set(wantValue)
				want = pointer.Interface().(WireMessage)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip got %#v, want %#v", got, want)
			}
		})
	}
}

func TestReadMessageRejectsWrongVersionBeforeAllocation(t *testing.T) {
	var frame [frameHeaderSize]byte
	copy(frame[:4], frameMagic[:])
	binary.BigEndian.PutUint16(frame[4:6], ProtocolVersion+1)
	frame[6] = byte(MessageHello)
	binary.BigEndian.PutUint32(frame[8:12], MaxFramePayload)
	_, err := ReadMessage(bytes.NewReader(frame[:]))
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("ReadMessage error = %v, want ErrVersionMismatch", err)
	}
}

func TestHandshakeOverNetPipe(t *testing.T) {
	hostConnection, clientConnection := net.Pipe()
	defer hostConnection.Close()
	defer clientConnection.Close()

	world := testWorld()
	config := HostHandshakeConfig{
		BuildID:        "test-build",
		SessionID:      99,
		AssignedPlayer: populous.DevilPlayer,
		TickRate:       DefaultTickRate,
		InputDelay:     DefaultInputDelay,
		Start:          Start{Tick: 0, Snapshot: world.Snapshot(), Rules: world.Rules},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hostResult := make(chan error, 1)
	go func() {
		hello, err := AcceptHandshake(ctx, hostConnection, config)
		if err == nil && hello.Name != "remote" {
			err = errors.New("host received wrong player name")
		}
		hostResult <- err
	}()

	welcome, start, err := JoinHandshake(ctx, clientConnection, NewHello("test-build", "remote", populous.DevilPlayer))
	if err != nil {
		t.Fatalf("JoinHandshake: %v", err)
	}
	if welcome.AssignedPlayer != populous.DevilPlayer || welcome.SessionID != 99 {
		t.Fatalf("unexpected welcome: %+v", welcome)
	}
	if start.Tick != 0 || start.Snapshot.GameTurn != 0 {
		t.Fatalf("unexpected start tick: %+v", start)
	}
	if err := <-hostResult; err != nil {
		t.Fatalf("AcceptHandshake: %v", err)
	}
}
