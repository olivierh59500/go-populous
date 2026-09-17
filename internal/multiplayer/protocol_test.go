package multiplayer

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
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

func TestStateFramesAreCompressedAndSmallMessagesAreNot(t *testing.T) {
	world := testWorld()
	for len(world.Peeps) < populous.MaxPeeps {
		world.Peeps = append(world.Peeps, populous.Peep{
			Flags:      populous.OnMove,
			Player:     byte(len(world.Peeps) & 1),
			Population: 100,
			AtPos:      len(world.Peeps) % (populous.MapWidth * populous.MapHeight),
		})
	}
	start := Start{Tick: 0, Snapshot: world.Snapshot(), Rules: world.Rules}
	var framed bytes.Buffer
	if err := WriteMessage(&framed, start); err != nil {
		t.Fatal(err)
	}
	data := framed.Bytes()
	if data[7] != frameFlagGZIP {
		t.Fatalf("start frame flags = %#x, want gzip", data[7])
	}
	raw, err := json.Marshal(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= len(raw) || len(data) >= 16<<10 {
		t.Fatalf("compressed start = %d bytes, raw = %d", len(data), len(raw))
	}

	framed.Reset()
	if err := WriteMessage(&framed, CommandBatch{Tick: 1}); err != nil {
		t.Fatal(err)
	}
	if got := framed.Bytes()[7]; got != 0 {
		t.Fatalf("batch frame flags = %#x, want zero", got)
	}
}

func TestReadMessageRejectsCorruptCompressedSnapshot(t *testing.T) {
	world := testWorld()
	var framed bytes.Buffer
	if err := WriteMessage(&framed, Start{Tick: 0, Snapshot: world.Snapshot(), Rules: world.Rules}); err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), framed.Bytes()...)
	data[len(data)-1] ^= 0xff // Corrupt the gzip checksum.
	if _, err := ReadMessage(bytes.NewReader(data)); err == nil {
		t.Fatal("ReadMessage accepted corrupt gzip payload")
	}
}

func TestReadMessageBoundsDecompressionBomb(t *testing.T) {
	bomb := bytes.Repeat([]byte{' '}, MaxSnapshotPayload+1)
	compressed, err := compressSnapshotPayload(bomb)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) > MaxFramePayload {
		t.Fatalf("test bomb compressed to %d bytes", len(compressed))
	}
	frame := make([]byte, frameHeaderSize+len(compressed))
	copy(frame[:4], frameMagic[:])
	binary.BigEndian.PutUint16(frame[4:6], ProtocolVersion)
	frame[6] = byte(MessageSnapshot)
	frame[7] = frameFlagGZIP
	binary.BigEndian.PutUint32(frame[8:12], uint32(len(compressed)))
	copy(frame[frameHeaderSize:], compressed)
	_, err = ReadMessage(bytes.NewReader(frame))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadMessage error = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadMessageAppliesTypeLimitBeforePayloadAllocation(t *testing.T) {
	var header [frameHeaderSize]byte
	copy(header[:4], frameMagic[:])
	binary.BigEndian.PutUint16(header[4:6], ProtocolVersion)
	header[6] = byte(MessageHello)
	binary.BigEndian.PutUint32(header[8:12], maxHelloPayload+1)
	_, err := ReadMessage(bytes.NewReader(header[:]))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadMessage error = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadMessageRejectsOversizedCompressedSnapshotBeforeAllocation(t *testing.T) {
	var header [frameHeaderSize]byte
	copy(header[:4], frameMagic[:])
	binary.BigEndian.PutUint16(header[4:6], ProtocolVersion)
	header[6] = byte(MessageSnapshot)
	header[7] = frameFlagGZIP
	binary.BigEndian.PutUint32(header[8:12], MaxFramePayload+1)
	_, err := ReadMessage(bytes.NewReader(header[:]))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("ReadMessage error = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadMessageRequiresCompressionOnlyForStateFrames(t *testing.T) {
	tests := []struct {
		name  string
		typ   MessageType
		flags byte
	}{
		{name: "uncompressed start", typ: MessageStart},
		{name: "compressed batch", typ: MessageBatch, flags: frameFlagGZIP},
		{name: "unknown flag", typ: MessageHello, flags: 0x80},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var header [frameHeaderSize]byte
			copy(header[:4], frameMagic[:])
			binary.BigEndian.PutUint16(header[4:6], ProtocolVersion)
			header[6] = byte(test.typ)
			header[7] = test.flags
			_, err := ReadMessage(bytes.NewReader(header[:]))
			if !errors.Is(err, ErrBadFrame) {
				t.Fatalf("ReadMessage error = %v, want ErrBadFrame", err)
			}
		})
	}
}

func TestTickRateMustMatchSimulation(t *testing.T) {
	welcome := Welcome{
		ProtocolVersion:  ProtocolVersion,
		StateHashVersion: populous.StateHashVersion,
		AssignedPlayer:   populous.DevilPlayer,
		TickRate:         DefaultTickRate + 1,
	}
	if err := validateMessage(welcome); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("validateMessage error = %v, want ErrInvalidMessage", err)
	}
	world := testWorld()
	config := HostHandshakeConfig{
		AssignedPlayer: populous.DevilPlayer,
		TickRate:       DefaultTickRate + 1,
		Start:          Start{Snapshot: world.Snapshot(), Rules: world.Rules},
	}
	if err := validateHostHandshakeConfig(config); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("validateHostHandshakeConfig error = %v, want ErrInvalidMessage", err)
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
