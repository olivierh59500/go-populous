package multiplayer

import (
	"bytes"
	"testing"

	"go-populous/internal/populous"
)

func benchmarkSnapshotMessage() Start {
	world := testWorld()
	for len(world.Peeps) < populous.MaxPeeps {
		world.Peeps = append(world.Peeps, populous.Peep{
			Flags:      populous.OnMove,
			Player:     byte(len(world.Peeps) & 1),
			Population: 100,
			AtPos:      len(world.Peeps) % (populous.MapWidth * populous.MapHeight),
		})
	}
	return Start{Tick: uint64(world.GameTurn), Snapshot: world.Snapshot(), Rules: world.Rules}
}

func BenchmarkWriteEmptyBatch(b *testing.B) {
	message := CommandBatch{Tick: 1}
	var output bytes.Buffer
	for b.Loop() {
		output.Reset()
		if err := WriteMessage(&output, message); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(output.Len()), "wire_bytes/op")
}

func BenchmarkReadEmptyBatch(b *testing.B) {
	var encoded bytes.Buffer
	if err := WriteMessage(&encoded, CommandBatch{Tick: 1}); err != nil {
		b.Fatal(err)
	}
	data := append([]byte(nil), encoded.Bytes()...)
	for b.Loop() {
		if _, err := ReadMessage(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "wire_bytes/op")
}

func BenchmarkWriteSnapshot(b *testing.B) {
	message := benchmarkSnapshotMessage()
	var output bytes.Buffer
	// Warm the gzip writer pool before measuring.
	if err := WriteMessage(&output, message); err != nil {
		b.Fatal(err)
	}
	wireSize := output.Len()
	b.ResetTimer()
	for b.Loop() {
		output.Reset()
		if err := WriteMessage(&output, message); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(wireSize), "wire_bytes/op")
}

func BenchmarkReadSnapshot(b *testing.B) {
	message := benchmarkSnapshotMessage()
	var encoded bytes.Buffer
	if err := WriteMessage(&encoded, message); err != nil {
		b.Fatal(err)
	}
	data := append([]byte(nil), encoded.Bytes()...)
	for b.Loop() {
		if _, err := ReadMessage(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "wire_bytes/op")
}
