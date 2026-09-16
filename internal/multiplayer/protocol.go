// Package multiplayer implements the transport-neutral wire protocol and the
// asynchronous two-player lockstep primitives used by Go Populous.
package multiplayer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"go-populous/internal/populous"
)

const (
	// ProtocolVersion covers both the frame header and all JSON payload schemas.
	ProtocolVersion uint16 = 1

	// MaxFramePayload bounds allocations before decoding data supplied by a
	// remote peer. A full WorldSnapshot is comfortably below this limit.
	MaxFramePayload = 2 << 20

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
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode %s: %w", message.messageType(), err)
	}
	if len(payload) > MaxFramePayload {
		return fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, len(payload))
	}

	var header [frameHeaderSize]byte
	copy(header[:4], frameMagic[:])
	binary.BigEndian.PutUint16(header[4:6], ProtocolVersion)
	header[6] = byte(message.messageType())
	// header[7] is reserved and must remain zero in protocol version 1.
	binary.BigEndian.PutUint32(header[8:12], uint32(len(payload)))
	if err := writeFull(writer, header[:]); err != nil {
		return fmt.Errorf("write multiplayer header: %w", err)
	}
	if err := writeFull(writer, payload); err != nil {
		return fmt.Errorf("write multiplayer payload: %w", err)
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
	if header[7] != 0 {
		return nil, fmt.Errorf("%w: reserved header byte is non-zero", ErrBadFrame)
	}
	typ := MessageType(header[6])
	message, err := newMessage(typ)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header[8:12])
	if length > MaxFramePayload {
		return nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, length)
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(reader, payload); err != nil {
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
	if message.TickRate == 0 || message.TickRate > 240 {
		return fmt.Errorf("%w: tick rate %d", ErrInvalidMessage, message.TickRate)
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

func writeFull(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
