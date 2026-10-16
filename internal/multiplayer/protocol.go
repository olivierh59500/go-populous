// Package multiplayer implements the transport-neutral wire protocol and the
// asynchronous two-player lockstep primitives used by Go Populous.
package multiplayer

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"go-populous/internal/populous"
)

const (
	// ProtocolVersion covers both the frame header and all JSON payload schemas.
	ProtocolVersion uint16 = 2

	// MaxFramePayload bounds bytes allocated directly from an announced wire
	// length. Start and Snapshot have a separate decompressed limit below.
	MaxFramePayload    = 256 << 10
	MaxSnapshotPayload = 512 << 10

	MaxCommandsPerBatch = 64
	MaxFutureTicks      = 64
	DefaultTickRate     = 8
	DefaultInputDelay   = 2
)

var (
	frameMagic = [4]byte{'P', 'O', 'P', 'L'}

	ErrBadFrame          = errors.New("invalid multiplayer frame")
	ErrVersionMismatch   = errors.New("multiplayer protocol version mismatch")
	ErrUnknownMessage    = errors.New("unknown multiplayer message")
	ErrFrameTooLarge     = errors.New("multiplayer frame too large")
	ErrInvalidMessage    = errors.New("invalid multiplayer message")
	ErrBuildMismatch     = errors.New("multiplayer build mismatch")
	ErrHashVersion       = errors.New("multiplayer state hash version mismatch")
	ErrSendQueueFull     = errors.New("multiplayer send queue full")
	ErrSessionClosed     = errors.New("multiplayer session closed")
	ErrUnexpectedMessage = errors.New("unexpected multiplayer message")
)

const frameHeaderSize = 12

const frameFlagGZIP byte = 1 << 0

const (
	maxHelloPayload      = 512
	maxWelcomePayload    = 512
	maxIntentPayload     = 512
	maxBatchPayload      = 32 << 10
	maxStateHashPayload  = 512
	maxErrorPayload      = 1 << 10
	maxHeartbeatPayload  = 256
	maxDisconnectPayload = 1 << 10
)

var snapshotGZIPWriters = sync.Pool{
	New: func() any {
		writer, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		if err != nil {
			panic(err)
		}
		return writer
	},
}

type MessageType uint8

const (
	MessageInvalid MessageType = iota
	MessageHello
	MessageWelcome
	MessageStart
	MessageIntent
	MessageBatch
	MessageStateHash
	MessageSnapshot
	MessageError
	MessagePing
	MessagePong
	MessageDisconnect
)

func (typ MessageType) String() string {
	switch typ {
	case MessageHello:
		return "hello"
	case MessageWelcome:
		return "welcome"
	case MessageStart:
		return "start"
	case MessageIntent:
		return "intent"
	case MessageBatch:
		return "batch"
	case MessageStateHash:
		return "state_hash"
	case MessageSnapshot:
		return "snapshot"
	case MessageError:
		return "error"
	case MessagePing:
		return "ping"
	case MessagePong:
		return "pong"
	case MessageDisconnect:
		return "disconnect"
	default:
		return "invalid"
	}
}

// WireMessage is implemented by each protocol payload. The unexported method
// intentionally limits the wire vocabulary to this package's versioned types.
type WireMessage interface {
	messageType() MessageType
}

// TypeOf returns the frame type for a message received through Peer.Events.
func TypeOf(message WireMessage) MessageType {
	if message == nil {
		return MessageInvalid
	}
	return message.messageType()
}

type Hello struct {
	ProtocolVersion  uint16 `json:"protocol_version"`
	StateHashVersion uint16 `json:"state_hash_version"`
	BuildID          string `json:"build_id"`
	Name             string `json:"name,omitempty"`
	RequestedPlayer  int    `json:"requested_player"`
}

func (Hello) messageType() MessageType { return MessageHello }

// NewHello supplies all compatibility fields. buildID should identify the
// game build or content revision; peers with different non-empty IDs are
// rejected by the handshake.
func NewHello(buildID, name string, requestedPlayer int) Hello {
	return Hello{
		ProtocolVersion:  ProtocolVersion,
		StateHashVersion: populous.StateHashVersion,
		BuildID:          buildID,
		Name:             name,
		RequestedPlayer:  requestedPlayer,
	}
}

type Welcome struct {
	ProtocolVersion  uint16 `json:"protocol_version"`
	StateHashVersion uint16 `json:"state_hash_version"`
	BuildID          string `json:"build_id"`
	SessionID        uint64 `json:"session_id"`
	AssignedPlayer   int    `json:"assigned_player"`
	TickRate         uint16 `json:"tick_rate"`
	InputDelay       uint64 `json:"input_delay"`
	CurrentTick      uint64 `json:"current_tick"`
}

func (Welcome) messageType() MessageType { return MessageWelcome }

// Start carries the canonical initial state. Rules are explicit because they
// are loaded from local Populous data and are not part of WorldSnapshot.
type Start struct {
	Tick     uint64                 `json:"tick"`
	Snapshot populous.WorldSnapshot `json:"snapshot"`
	Rules    populous.TerrainRules  `json:"rules"`
}

func (Start) messageType() MessageType { return MessageStart }

type Intent struct {
	RequestedTick uint64           `json:"requested_tick"`
	Sequence      uint64           `json:"sequence"`
	Command       populous.Command `json:"command"`
}

func (Intent) messageType() MessageType { return MessageIntent }

type SequencedCommand struct {
	Sequence uint64           `json:"sequence"`
	Command  populous.Command `json:"command"`
}

// CommandBatch is emitted once for every simulation tick, including ticks
// with no commands. This is the client's permission to advance that tick.
type CommandBatch struct {
	Tick     uint64             `json:"tick"`
	Commands []SequencedCommand `json:"commands,omitempty"`
}

func (CommandBatch) messageType() MessageType { return MessageBatch }

type StateHash struct {
	Tick uint64   `json:"tick"`
	Hash [32]byte `json:"hash"`
}

func (StateHash) messageType() MessageType { return MessageStateHash }

// Snapshot is used only for explicit resynchronization or a future rejoin
// flow, not as the normal per-tick replication mechanism.
type Snapshot struct {
	Tick     uint64                 `json:"tick"`
	Snapshot populous.WorldSnapshot `json:"snapshot"`
	Rules    populous.TerrainRules  `json:"rules"`
}

func (Snapshot) messageType() MessageType { return MessageSnapshot }

type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (ErrorMessage) messageType() MessageType { return MessageError }

type Ping struct {
	Nonce          uint64 `json:"nonce"`
	SentUnixMillis int64  `json:"sent_unix_millis"`
}

func (Ping) messageType() MessageType { return MessagePing }

type Pong struct {
	Nonce          uint64 `json:"nonce"`
	SentUnixMillis int64  `json:"sent_unix_millis"`
}

func (Pong) messageType() MessageType { return MessagePong }

type Disconnect struct {
	Reason string `json:"reason,omitempty"`
}

func (Disconnect) messageType() MessageType { return MessageDisconnect }

// WriteMessage writes one length-delimited frame. Calls sharing a writer must
// be serialized; Peer provides that serialization for live connections.
func WriteMessage(writer io.Writer, message WireMessage) error {
	if writer == nil {
		return fmt.Errorf("%w: nil writer", ErrInvalidMessage)
	}
	if err := validateMessage(message); err != nil {
		return err
	}
	typ := message.messageType()
	wireLimit, decodedLimit, err := messagePayloadLimits(typ)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode %s: %w", typ, err)
	}
	if len(payload) > decodedLimit {
		return fmt.Errorf("%w: decoded %s payload is %d bytes, maximum %d", ErrFrameTooLarge, typ, len(payload), decodedLimit)
	}

	flags := byte(0)
	if messageUsesCompression(typ) {
		payload, err = compressSnapshotPayload(payload)
		if err != nil {
			return fmt.Errorf("compress %s: %w", typ, err)
		}
		flags = frameFlagGZIP
	}
	if len(payload) > wireLimit {
		return fmt.Errorf("%w: encoded %s payload is %d bytes, maximum %d", ErrFrameTooLarge, typ, len(payload), wireLimit)
	}

	var header [frameHeaderSize]byte
	copy(header[:4], frameMagic[:])
	binary.BigEndian.PutUint16(header[4:6], ProtocolVersion)
	header[6] = byte(typ)
	header[7] = flags
	binary.BigEndian.PutUint32(header[8:12], uint32(len(payload)))
	buffers := net.Buffers{header[:], payload}
	written, err := buffers.WriteTo(writer)
	if err != nil {
		return fmt.Errorf("write multiplayer frame: %w", err)
	}
	if written != int64(len(header)+len(payload)) {
		return fmt.Errorf("write multiplayer frame: %w", io.ErrShortWrite)
	}
	return nil
}

// ReadMessage reads and validates one frame, returning a pointer to its
// concrete payload type.
func ReadMessage(reader io.Reader) (WireMessage, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrBadFrame)
	}
	var header [frameHeaderSize]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	if !bytes.Equal(header[:4], frameMagic[:]) {
		return nil, fmt.Errorf("%w: wrong magic", ErrBadFrame)
	}
	version := binary.BigEndian.Uint16(header[4:6])
	if version != ProtocolVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrVersionMismatch, version, ProtocolVersion)
	}
	typ := MessageType(header[6])
	wireLimit, decodedLimit, err := messagePayloadLimits(typ)
	if err != nil {
		return nil, err
	}
	flags := header[7]
	if err := validateFrameFlags(typ, flags); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header[8:12])
	if uint64(length) > uint64(wireLimit) {
		return nil, fmt.Errorf("%w: encoded %s payload is %d bytes, maximum %d", ErrFrameTooLarge, typ, length, wireLimit)
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}

	if flags&frameFlagGZIP != 0 {
		payload, err = decompressSnapshotPayload(payload, decodedLimit)
		if err != nil {
			return nil, fmt.Errorf("decompress %s: %w", typ, err)
		}
	} else if len(payload) > decodedLimit {
		return nil, fmt.Errorf("%w: decoded %s payload is %d bytes, maximum %d", ErrFrameTooLarge, typ, len(payload), decodedLimit)
	}

	message, err := newMessage(typ)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(message); err != nil {
		return nil, fmt.Errorf("decode %s: %w", typ, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%w: trailing %s payload", ErrBadFrame, typ)
		}
		return nil, fmt.Errorf("%w: trailing %s payload: %v", ErrBadFrame, typ, err)
	}
	if err := validateMessage(message); err != nil {
		return nil, err
	}
	return message, nil
}

func newMessage(typ MessageType) (WireMessage, error) {
	switch typ {
	case MessageHello:
		return &Hello{}, nil
	case MessageWelcome:
		return &Welcome{}, nil
	case MessageStart:
		return &Start{}, nil
	case MessageIntent:
		return &Intent{}, nil
	case MessageBatch:
		return &CommandBatch{}, nil
	case MessageStateHash:
		return &StateHash{}, nil
	case MessageSnapshot:
		return &Snapshot{}, nil
	case MessageError:
		return &ErrorMessage{}, nil
	case MessagePing:
		return &Ping{}, nil
	case MessagePong:
		return &Pong{}, nil
	case MessageDisconnect:
		return &Disconnect{}, nil
	default:
		return nil, fmt.Errorf("%w: type %d", ErrUnknownMessage, typ)
	}
}

func validateMessage(message WireMessage) error {
	if message == nil {
		return fmt.Errorf("%w: nil payload", ErrInvalidMessage)
	}
	switch value := message.(type) {
	case Hello:
		return validateHello(value)
	case *Hello:
		if value == nil {
			return fmt.Errorf("%w: nil hello", ErrInvalidMessage)
		}
		return validateHello(*value)
	case Welcome:
		return validateWelcome(value)
	case *Welcome:
		if value == nil {
			return fmt.Errorf("%w: nil welcome", ErrInvalidMessage)
		}
		return validateWelcome(*value)
	case Start:
		return validateStart(value)
	case *Start:
		if value == nil {
			return fmt.Errorf("%w: nil start", ErrInvalidMessage)
		}
		return validateStart(*value)
	case Intent:
		return validateIntent(value)
	case *Intent:
		if value == nil {
			return fmt.Errorf("%w: nil intent", ErrInvalidMessage)
		}
		return validateIntent(*value)
	case CommandBatch:
		return validateBatch(value)
	case *CommandBatch:
		if value == nil {
			return fmt.Errorf("%w: nil batch", ErrInvalidMessage)
		}
		return validateBatch(*value)
	case StateHash, Ping, Pong:
		return nil
	case *StateHash:
		if value == nil {
			return fmt.Errorf("%w: nil state hash", ErrInvalidMessage)
		}
		return nil
	case *Ping:
		if value == nil {
			return fmt.Errorf("%w: nil ping", ErrInvalidMessage)
		}
		return nil
	case *Pong:
		if value == nil {
			return fmt.Errorf("%w: nil pong", ErrInvalidMessage)
		}
		return nil
	case Snapshot:
		return validateSnapshot(value.Tick, value.Snapshot)
	case *Snapshot:
		if value == nil {
			return fmt.Errorf("%w: nil snapshot", ErrInvalidMessage)
		}
		return validateSnapshot(value.Tick, value.Snapshot)
	case ErrorMessage:
		return validateErrorMessage(value)
	case *ErrorMessage:
		if value == nil {
			return fmt.Errorf("%w: nil error message", ErrInvalidMessage)
		}
		return validateErrorMessage(*value)
	case Disconnect:
		return validateText("disconnect reason", value.Reason, 512)
	case *Disconnect:
		if value == nil {
			return fmt.Errorf("%w: nil disconnect", ErrInvalidMessage)
		}
		return validateText("disconnect reason", value.Reason, 512)
	default:
		return fmt.Errorf("%w: unsupported payload %T", ErrInvalidMessage, message)
	}
}

func validateHello(message Hello) error {
	if message.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("%w: hello has %d, want %d", ErrVersionMismatch, message.ProtocolVersion, ProtocolVersion)
	}
	if message.StateHashVersion != populous.StateHashVersion {
		return fmt.Errorf("%w: hello has %d, want %d", ErrHashVersion, message.StateHashVersion, populous.StateHashVersion)
	}
	if message.RequestedPlayer != populous.GodPlayer && message.RequestedPlayer != populous.DevilPlayer {
		return fmt.Errorf("%w: requested player %d", ErrInvalidMessage, message.RequestedPlayer)
	}
	if err := validateText("build ID", message.BuildID, 128); err != nil {
		return err
	}
	return validateText("player name", message.Name, 64)
}

func validateWelcome(message Welcome) error {
	if message.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("%w: welcome has %d, want %d", ErrVersionMismatch, message.ProtocolVersion, ProtocolVersion)
	}
	if message.StateHashVersion != populous.StateHashVersion {
		return fmt.Errorf("%w: welcome has %d, want %d", ErrHashVersion, message.StateHashVersion, populous.StateHashVersion)
	}
	if message.AssignedPlayer != populous.GodPlayer && message.AssignedPlayer != populous.DevilPlayer {
		return fmt.Errorf("%w: assigned player %d", ErrInvalidMessage, message.AssignedPlayer)
	}
	if message.TickRate != DefaultTickRate {
		return fmt.Errorf("%w: tick rate %d, want %d", ErrInvalidMessage, message.TickRate, DefaultTickRate)
	}
	if message.InputDelay > MaxFutureTicks {
		return fmt.Errorf("%w: input delay %d", ErrInvalidMessage, message.InputDelay)
	}
	return validateText("build ID", message.BuildID, 128)
}

func validateStart(message Start) error {
	return validateSnapshot(message.Tick, message.Snapshot)
}

func validateSnapshot(tick uint64, snapshot populous.WorldSnapshot) error {
	if snapshot.GameTurn < 0 || uint64(snapshot.GameTurn) != tick {
		return fmt.Errorf("%w: snapshot turn %d does not match tick %d", ErrInvalidMessage, snapshot.GameTurn, tick)
	}
	if len(snapshot.Peeps) > populous.MaxPeeps {
		return fmt.Errorf("%w: snapshot has %d peeps", ErrInvalidMessage, len(snapshot.Peeps))
	}
	return nil
}

func validateIntent(message Intent) error {
	if message.Sequence == 0 {
		return fmt.Errorf("%w: intent sequence is zero", ErrInvalidMessage)
	}
	if err := message.Command.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	return nil
}

func validateBatch(message CommandBatch) error {
	if message.Tick == 0 {
		return fmt.Errorf("%w: batch tick is zero", ErrInvalidMessage)
	}
	if len(message.Commands) > MaxCommandsPerBatch {
		return fmt.Errorf("%w: batch has %d commands", ErrInvalidMessage, len(message.Commands))
	}
	lastPlayer := -1
	var lastSequence uint64
	for index, command := range message.Commands {
		if command.Sequence == 0 {
			return fmt.Errorf("%w: batch command %d has zero sequence", ErrInvalidMessage, index)
		}
		if err := command.Command.Validate(); err != nil {
			return fmt.Errorf("%w: batch command %d: %v", ErrInvalidMessage, index, err)
		}
		player := command.Command.Player
		if player < lastPlayer || (player == lastPlayer && command.Sequence <= lastSequence) {
			return fmt.Errorf("%w: batch commands are not canonically ordered", ErrInvalidMessage)
		}
		lastPlayer = player
		lastSequence = command.Sequence
	}
	return nil
}

func validateErrorMessage(message ErrorMessage) error {
	if message.Code == "" {
		return fmt.Errorf("%w: empty error code", ErrInvalidMessage)
	}
	if err := validateText("error code", message.Code, 64); err != nil {
		return err
	}
	return validateText("error message", message.Message, 512)
}

func validateText(name, value string, maximum int) error {
	if len(value) > maximum {
		return fmt.Errorf("%w: %s is %d bytes, maximum %d", ErrInvalidMessage, name, len(value), maximum)
	}
	return nil
}

func messageUsesCompression(typ MessageType) bool {
	return typ == MessageStart || typ == MessageSnapshot
}

func validateFrameFlags(typ MessageType, flags byte) error {
	if flags&^frameFlagGZIP != 0 {
		return fmt.Errorf("%w: unsupported frame flags %#x", ErrBadFrame, flags)
	}
	compressed := flags&frameFlagGZIP != 0
	if compressed != messageUsesCompression(typ) {
		return fmt.Errorf("%w: invalid compression flag for %s", ErrBadFrame, typ)
	}
	return nil
}

func messagePayloadLimits(typ MessageType) (wire, decoded int, err error) {
	switch typ {
	case MessageHello:
		return maxHelloPayload, maxHelloPayload, nil
	case MessageWelcome:
		return maxWelcomePayload, maxWelcomePayload, nil
	case MessageStart, MessageSnapshot:
		return MaxFramePayload, MaxSnapshotPayload, nil
	case MessageIntent:
		return maxIntentPayload, maxIntentPayload, nil
	case MessageBatch:
		return maxBatchPayload, maxBatchPayload, nil
	case MessageStateHash:
		return maxStateHashPayload, maxStateHashPayload, nil
	case MessageError:
		return maxErrorPayload, maxErrorPayload, nil
	case MessagePing, MessagePong:
		return maxHeartbeatPayload, maxHeartbeatPayload, nil
	case MessageDisconnect:
		return maxDisconnectPayload, maxDisconnectPayload, nil
	default:
		return 0, 0, fmt.Errorf("%w: type %d", ErrUnknownMessage, typ)
	}
}

func compressSnapshotPayload(payload []byte) ([]byte, error) {
	var compressed bytes.Buffer
	writer := snapshotGZIPWriters.Get().(*gzip.Writer)
	writer.Reset(&compressed)
	defer func() {
		writer.Reset(io.Discard)
		snapshotGZIPWriters.Put(writer)
	}()
	if _, err := writer.Write(payload); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func decompressSnapshotPayload(payload []byte, limit int) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(decoded) > limit {
		return nil, fmt.Errorf("%w: decompressed payload exceeds %d bytes", ErrFrameTooLarge, limit)
	}
	return decoded, nil
}
