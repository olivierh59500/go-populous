// Package platformbridge contains the concurrency boundary between a mobile
// platform callback thread and Populous' Ebitengine update thread. It has no
// graphics or Android dependency, so its ordering and race guarantees can be
// tested on every host.
package platformbridge

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type Phase uint8

const (
	Idle Phase = iota
	Hosting
	Joining
)

type EventKind uint8

const (
	EventStatus EventKind = iota + 1
	EventReady
	EventFailure
	EventCancel
)

type Event struct {
	Kind          EventKind
	Status        string
	Address, Name string
}

type Bridge struct {
	mu       sync.Mutex
	phase    Phase
	status   string
	preamble []byte
	commands []string
	events   []Event
}

func New() *Bridge { return &Bridge{} }

func (bridge *Bridge) Reserve(phase Phase, status string) error {
	if bridge == nil {
		return fmt.Errorf("Bluetooth is unavailable")
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.phase != Idle {
		return fmt.Errorf("a Bluetooth connection is already in progress")
	}
	bridge.phase = phase
	bridge.status = status
	clear(bridge.preamble)
	bridge.preamble = nil
	bridge.events = bridge.events[:0]
	return nil
}

func (bridge *Bridge) ReleaseReservation() {
	if bridge == nil {
		return
	}
	bridge.mu.Lock()
	bridge.phase = Idle
	bridge.status = ""
	clear(bridge.preamble)
	bridge.preamble = nil
	bridge.events = bridge.events[:0]
	bridge.mu.Unlock()
}

func (bridge *Bridge) EnqueueCommand(command string) {
	if bridge == nil || command == "" {
		return
	}
	bridge.mu.Lock()
	bridge.commands = append(bridge.commands, command)
	bridge.mu.Unlock()
}

func (bridge *Bridge) PollCommand() string {
	if bridge == nil {
		return ""
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.commands) == 0 {
		return ""
	}
	command := bridge.commands[0]
	copy(bridge.commands, bridge.commands[1:])
	bridge.commands[len(bridge.commands)-1] = ""
	bridge.commands = bridge.commands[:len(bridge.commands)-1]
	return command
}

func (bridge *Bridge) EnqueueEvent(event Event) {
	if bridge == nil {
		return
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	// Late Activity callbacks after a cancellation must not revive a match.
	if bridge.phase == Idle {
		return
	}
	// Android can report many progress strings while its picker is open. Keep
	// only the most recent adjacent one and bound hostile/repeated callbacks.
	if event.Kind == EventStatus && len(bridge.events) > 0 && bridge.events[len(bridge.events)-1].Kind == EventStatus {
		bridge.events[len(bridge.events)-1] = event
		return
	}
	const maxQueuedEvents = 32
	if len(bridge.events) == maxQueuedEvents {
		copy(bridge.events, bridge.events[1:])
		bridge.events = bridge.events[:maxQueuedEvents-1]
	}
	bridge.events = append(bridge.events, event)
}

func (bridge *Bridge) TakeEvents() []Event {
	if bridge == nil {
		return nil
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if len(bridge.events) == 0 {
		return nil
	}
	events := append([]Event(nil), bridge.events...)
	bridge.events = bridge.events[:0]
	return events
}

func (bridge *Bridge) Phase() Phase {
	if bridge == nil {
		return Idle
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return bridge.phase
}

func (bridge *Bridge) SetStatus(status string) bool {
	if bridge == nil {
		return false
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.phase == Idle {
		return false
	}
	bridge.status = status
	return true
}

func (bridge *Bridge) StatusText() string {
	if bridge == nil {
		return ""
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return bridge.status
}

func (bridge *Bridge) SetPreamble(preamble []byte) bool {
	if bridge == nil {
		return false
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.phase == Idle {
		return false
	}
	clear(bridge.preamble)
	bridge.preamble = append(bridge.preamble[:0], preamble...)
	return true
}

func (bridge *Bridge) Preamble() []byte {
	if bridge == nil {
		return nil
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return append([]byte(nil), bridge.preamble...)
}

func (bridge *Bridge) ClearPreamble() {
	if bridge == nil {
		return
	}
	bridge.mu.Lock()
	clear(bridge.preamble)
	bridge.preamble = nil
	bridge.mu.Unlock()
}

// Stop queues a cancellation without waiting for the platform. Pending start
// commands are discarded, while an already-polled command is harmlessly
// cancelled by the same BT_CANCEL message.
func (bridge *Bridge) Stop() bool {
	if bridge == nil {
		return false
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.phase == Idle {
		return false
	}
	bridge.phase = Idle
	bridge.status = ""
	clear(bridge.preamble)
	bridge.preamble = nil
	bridge.events = bridge.events[:0]
	clear(bridge.commands)
	bridge.commands = bridge.commands[:0]
	bridge.commands = append(bridge.commands, "BT_CANCEL")
	return true
}

func NormalizeLoopbackAddress(address string) (string, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return "", fmt.Errorf("invalid Bluetooth proxy address")
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return "", fmt.Errorf("Bluetooth proxy is not on loopback")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid Bluetooth proxy port")
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}

func CleanText(value string, limit int) string {
	value = strings.ToValidUTF8(value, "?")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit]))
}
