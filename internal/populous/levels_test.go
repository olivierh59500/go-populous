package populous

import (
	"strings"
	"testing"
)

func TestCodeForLevel(t *testing.T) {
	tests := map[int]string{
		0:    "SHISODING",
		494:  "WEAVUSPERT",
		1000: "EOADIEPERT",
	}
	for level, want := range tests {
		if got := CodeForLevel(level); got != want {
			t.Fatalf("CodeForLevel(%d) = %q, want %q", level, got, want)
		}
	}
}

func TestDecodeLevelCode(t *testing.T) {
	if got, ok := DecodeLevelCode("genesis", 1000); !ok || got != 0 {
		t.Fatalf("DecodeLevelCode(GENESIS) = %d %v, want 0 true", got, ok)
	}
	if got, ok := DecodeLevelCode("WEAVUSPERT", 1000); !ok || got != 494 {
		t.Fatalf("DecodeLevelCode(WEAVUSPERT) = %d %v, want 494 true", got, ok)
	}
}

func TestLoadLevels(t *testing.T) {
	data := strings.NewReader("\x01\x02\x03\x04\x05\x06\x07\x08\x12\x34")
	levels, err := LoadLevels(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 5 {
		t.Fatalf("len(levels) = %d, want 5", len(levels))
	}
	if levels[0].SeedOffset != 0x1234 {
		t.Fatalf("SeedOffset = %#x, want 0x1234", levels[0].SeedOffset)
	}
}
