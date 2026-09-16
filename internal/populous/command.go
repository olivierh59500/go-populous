package populous

import (
	"errors"
	"fmt"
)

// CommandKind identifies a deterministic mutation that can be applied to a
// World. Zero is deliberately invalid so a zero-value Command cannot mutate a
// game accidentally.
type CommandKind uint8

const (
	CommandInvalid CommandKind = iota
	CommandRaise
	CommandLower
	CommandPaintRaise
	CommandPaintLower
	CommandSetMagnet
	CommandSetTendency
	CommandSwamp
	CommandQuake
	CommandVolcano
	CommandFlood
	CommandKnight
	CommandArmageddon
)

var (
	// ErrInvalidCommand reports a malformed command. It is distinct from a
	// well-formed command rejected by the game rules (for example, insufficient
	// mana), for which ApplyCommand returns (false, nil).
	ErrInvalidCommand = errors.New("invalid populous command")
	ErrNilWorld       = errors.New("cannot apply a command to a nil world")
)

// Command is the complete, transport-neutral representation of a player
// action that mutates the simulation. X and Y are used by targeted actions;
// Value is used by CommandSetTendency.
//
// Player is always explicit. A network host must additionally verify that it
// matches the player assigned to the connection before accepting a command.
type Command struct {
	Kind   CommandKind `json:"kind"`
	Player int         `json:"player"`
	X      int         `json:"x,omitempty"`
	Y      int         `json:"y,omitempty"`
	Value  int         `json:"value,omitempty"`
}

func (kind CommandKind) String() string {
	switch kind {
	case CommandRaise:
		return "raise"
	case CommandLower:
		return "lower"
	case CommandPaintRaise:
		return "paint_raise"
	case CommandPaintLower:
		return "paint_lower"
	case CommandSetMagnet:
		return "set_magnet"
	case CommandSetTendency:
		return "set_tendency"
	case CommandSwamp:
		return "swamp"
	case CommandQuake:
		return "quake"
	case CommandVolcano:
		return "volcano"
	case CommandFlood:
		return "flood"
	case CommandKnight:
		return "knight"
	case CommandArmageddon:
		return "armageddon"
	default:
		return "invalid"
	}
}

// Validate checks the command's wire-level invariants. It intentionally does
// not inspect mutable world state such as mana: those rules are evaluated at
// the command's assigned simulation tick by ApplyCommand.
func (command Command) Validate() error {
	if command.Player != GodPlayer && command.Player != DevilPlayer {
		return fmt.Errorf("%w: player %d", ErrInvalidCommand, command.Player)
	}

	switch command.Kind {
	case CommandRaise, CommandLower, CommandPaintRaise, CommandPaintLower:
		// Sculpting addresses the 65x65 altitude vertices, not only the 64x64
		// block cells.
		if command.X < 0 || command.X > MapWidth || command.Y < 0 || command.Y > MapHeight {
			return fmt.Errorf("%w: %s point (%d,%d) outside altitude map", ErrInvalidCommand, command.Kind, command.X, command.Y)
		}
		if command.Value != 0 {
			return fmt.Errorf("%w: %s has unexpected value %d", ErrInvalidCommand, command.Kind, command.Value)
		}
	case CommandSetMagnet, CommandSwamp, CommandQuake, CommandVolcano:
		if command.X < 0 || command.X >= MapWidth || command.Y < 0 || command.Y >= MapHeight {
			return fmt.Errorf("%w: %s tile (%d,%d) outside map", ErrInvalidCommand, command.Kind, command.X, command.Y)
		}
		if command.Value != 0 {
			return fmt.Errorf("%w: %s has unexpected value %d", ErrInvalidCommand, command.Kind, command.Value)
		}
	case CommandSetTendency:
		if command.Value < MagnetMode || command.Value > FightMode {
			return fmt.Errorf("%w: tendency %d", ErrInvalidCommand, command.Value)
		}
		if command.X != 0 || command.Y != 0 {
			return fmt.Errorf("%w: tendency has unexpected target (%d,%d)", ErrInvalidCommand, command.X, command.Y)
		}
	case CommandFlood, CommandKnight, CommandArmageddon:
		if command.X != 0 || command.Y != 0 || command.Value != 0 {
			return fmt.Errorf("%w: %s has unexpected operands", ErrInvalidCommand, command.Kind)
		}
	default:
		return fmt.Errorf("%w: kind %d", ErrInvalidCommand, command.Kind)
	}
	return nil
}

// ApplyCommand validates and applies one deterministic player mutation.
//
// The boolean is false when the command is well formed but the current game
// rules reject it (for example, insufficient mana, a disabled power, or no
// papal magnet carrier to knight). Such a rejection is deterministic and is
// not a protocol error.
func (w *World) ApplyCommand(command Command) (bool, error) {
	if w == nil {
		return false, ErrNilWorld
	}
	if err := command.Validate(); err != nil {
		return false, err
	}

	switch command.Kind {
	case CommandRaise:
		return w.RaiseAt(command.Player, command.X, command.Y), nil
	case CommandLower:
		return w.LowerAt(command.Player, command.X, command.Y), nil
	case CommandPaintRaise:
		return w.PaintRaiseAt(command.X, command.Y), nil
	case CommandPaintLower:
		return w.PaintLowerAt(command.X, command.Y), nil
	case CommandSetMagnet:
		return w.SetMagnetToTile(command.Player, command.X, command.Y), nil
	case CommandSetTendency:
		return w.SetMagnetMode(command.Player, command.Value), nil
	case CommandSwamp:
		return w.SwampAtTile(command.Player, command.X, command.Y), nil
	case CommandQuake:
		return w.QuakeAtTile(command.Player, command.X, command.Y), nil
	case CommandVolcano:
		return w.VolcanoAtTile(command.Player, command.X, command.Y), nil
	case CommandFlood:
		return w.Flood(command.Player), nil
	case CommandKnight:
		return w.Knight(command.Player), nil
	case CommandArmageddon:
		return w.WarPower(command.Player), nil
	default:
		// Validate already rejects this; retain the guard so future command
		// additions cannot silently become successful no-ops.
		return false, fmt.Errorf("%w: kind %d", ErrInvalidCommand, command.Kind)
	}
}
