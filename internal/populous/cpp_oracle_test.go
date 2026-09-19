//go:build cpporacle

package populous

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This optional test runs original bodies, not a hand-translated reference.
// See testdata/cpporacle/README.md for its deliberately limited coverage.
func TestCPPOracleParity(t *testing.T) {
	oracle := compileCPPOracle(t)
	run := func(t *testing.T, mode, input string, count int) []string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, oracle, mode)
		cmd.Stdin = strings.NewReader(input)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("C++ oracle %s: %v: %s", mode, err, output)
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) != count {
			t.Fatalf("C++ oracle %s returned %d rows, want %d: %s", mode, len(lines), count, output)
		}
		return lines
	}

	f, err := os.Open("../../assets/amiga/level.dat")
	if err != nil {
		t.Fatal(err)
	}
	levels, err := LoadLevels(f)
	closeErr := f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if len(levels) != 495 {
		t.Fatalf("campaign has %d worlds, want 495", len(levels))
	}

	t.Run("terrain_primitives_990_cases", func(t *testing.T) {
		var input strings.Builder
		for _, level := range levels {
			seed := uint16(level.SeedOffset) + uint16((level.Number*5)&7)
			for _, draws := range []int{0, 4} {
				fmt.Fprintf(&input, "%d %d 0\n", seed, draws)
			}
		}
		rows := run(t, "terrain", input.String(), len(levels)*2)
		row := 0
		for _, level := range levels {
			seed := uint16(level.SeedOffset) + uint16((level.Number*5)&7)
			for _, draws := range []int{0, 4} {
				w := &World{rng: lcg(seed)}
				for i := 0; i < draws; i++ {
					w.rng.next()
				}
				w.makeAlt()
				w.makeMap(0, 0, MapWidth-1, MapHeight-1)
				w.makeWoodsRocks()
				if got := legacyTerrainSignature(w); got != rows[row] {
					t.Fatalf("world %d, %d prior draws: Go=%s; original C++=%s", level.Number, draws, got, rows[row])
				}
				row++
			}
		}
	})

	t.Run("generated_landscapes_495_worlds", func(t *testing.T) {
		var input strings.Builder
		for _, level := range levels {
			seed := uint16(level.SeedOffset) + uint16((level.Number*5)&7)
			fmt.Fprintf(&input, "%d 4 1\n", seed)
		}
		rows := run(t, "terrain", input.String(), len(levels))
		for i, level := range levels {
			w := GenerateWorld(level)
			if got := legacyTerrainSignature(w); got != rows[i] {
				t.Fatalf("generated world %d: Go=%s; original C++=%s", level.Number, got, rows[i])
			}
		}
	})

	t.Run("combat_rng_both_players", func(t *testing.T) {
		seeds := []lcg{0, 2, 5, 8, 12345, 32767}
		var input strings.Builder
		for player := 0; player < 2; player++ {
			for _, seed := range seeds {
				fmt.Fprintf(&input, "%d %d\n", seed, player)
			}
		}
		rows := run(t, "battle", input.String(), len(seeds)*2)
		row := 0
		for player := 0; player < 2; player++ {
			for _, seed := range seeds {
				w := &World{rng: seed, Peeps: []Peep{
					{Player: byte(player), Population: 1000, Weapons: 3, BattlePopulation: 1, Flags: InBattle},
					{Player: byte(player ^ 1), Population: 2000, Weapons: 5, Flags: InBattle | OnMove},
				}}
				w.doBattle(0)
				got := fmt.Sprintf("%d %d %d", w.Peeps[0].Population, w.Peeps[1].Population, uint16(w.rng))
				if got != rows[row] {
					t.Fatalf("player %d seed %d: Go=%s; original C++=%s", player, seed, got, rows[row])
				}
				row++
			}
		}
	})

	t.Run("knight_town_reinforcement_both_players", func(t *testing.T) {
		rows := run(t, "join", "0\n1\n", 2)
		for player := 0; player < 2; player++ {
			w := &World{Peeps: []Peep{
				{Player: byte(player), Population: 1000, Weapons: 20, IQ: 4, Status: 1, HeadFor: 3, Flags: OnMove, AtPos: 650},
				{Player: byte(player), Population: 100, Weapons: 1, IQ: 1, Status: 2, Flags: InTown | InBattle, Frame: 71, BattlePopulation: 2, AtPos: 650},
				{Player: byte(player ^ 1), Population: 500, Flags: InBattle, BattlePopulation: 1, AtPos: 650},
			}}
			w.Magnets[player] = Magnet{Carried: 1, Population: 1000}
			w.joinBattle(0, 2)
			headed := 0
			if w.Peeps[1].HeadFor == 3 {
				headed = 1
			}
			p := w.Peeps[1]
			got := fmt.Sprintf("%d %d %d %d %d %d %d %d %d %d", w.Peeps[0].Population, p.Population, headed, p.Weapons, p.IQ, p.Status, p.Flags, p.Frame, w.Magnets[player].Population, w.Magnets[player].Carried)
			if got != rows[player] {
				t.Fatalf("player %d: Go=%s; original C++=%s", player, got, rows[player])
			}
		}
	})
}

func compileCPPOracle(t *testing.T) string {
	t.Helper()
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skip("optional C++ oracle unavailable: c++ compiler not found on PATH")
	}
	root, err := filepath.Abs("../../previous/DCPopulous-master")
	if err != nil {
		t.Fatal(err)
	}
	units := []struct {
		file      string
		functions []string
	}{
		{"populous_misc.cpp", []string{"newrand"}},
		{"populous_map.cpp", []string{"raise_point", "make_alt", "make_thing", "make_map"}},
		{"populous_effect.cpp", []string{"make_woods_rocks"}},
		{"populous_battle.cpp", []string{"do_battle", "join_battle"}},
	}
	prefix, err := os.ReadFile("testdata/cpporacle/prefix.hpp")
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	source.Write(prefix)
	for _, unit := range units {
		path := filepath.Join(root, unit.file)
		original, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			t.Skipf("optional C++ oracle unavailable: missing original source %s", path)
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("original %s SHA-256 %x", unit.file, sha256.Sum256(original))
		for _, name := range unit.functions {
			body, line, err := extractCPPOracleFunction(original, name)
			if err != nil {
				t.Fatalf("extract %s from %s: %v", name, path, err)
			}
			fmt.Fprintf(&source, "\n#line %d %s\n", line, strconv.Quote(filepath.ToSlash(path)))
			source.Write(body)
			source.WriteByte('\n')
		}
	}
	mainPath, err := filepath.Abs("testdata/cpporacle/main.cpp")
	if err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&source, "\n#line 1 %s\n", strconv.Quote(filepath.ToSlash(mainPath)))
	source.Write(main)
	directory := t.TempDir()
	cppPath := filepath.Join(directory, "oracle.cpp")
	if err := os.WriteFile(cppPath, source.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(directory, "oracle")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, compiler, "-std=c++14", "-O1", cppPath, "-o", binaryPath).CombinedOutput()
	if err != nil {
		t.Fatalf("compile original C++ oracle: %v: %s", err, output)
	}
	return binaryPath
}

// Only lexical boundaries are interpreted: function bytes are never rewritten.
func extractCPPOracleFunction(source []byte, name string) ([]byte, int, error) {
	declaration := regexp.MustCompile(`(?:int|void|SHORT)\s+CPopulous::` + regexp.QuoteMeta(name) + `\s*\([^)]*\)\s*\{`)
	match := declaration.FindIndex(source)
	if match == nil {
		return nil, 0, fmt.Errorf("function not found")
	}
	line := bytes.Count(source[:match[0]], []byte{'\n'}) + 1
	depth := 0
	for i := match[1] - 1; i < len(source); i++ {
		switch source[i] {
		case '"', '\'':
			quote := source[i]
			for i++; i < len(source) && source[i] != quote; i++ {
				if source[i] == '\\' {
					i++
				}
			}
		case '/':
			if i+1 < len(source) && source[i+1] == '/' {
				for i += 2; i < len(source) && source[i] != '\n'; i++ {
				}
			} else if i+1 < len(source) && source[i+1] == '*' {
				end := bytes.Index(source[i+2:], []byte("*/"))
				if end < 0 {
					return nil, 0, fmt.Errorf("unclosed block comment")
				}
				i += end + 3
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[match[0] : i+1], line, nil
			}
		}
	}
	return nil, 0, fmt.Errorf("unclosed function")
}

func TestCPPOracleExtractionPreservesBodyAndIgnoresCommentBraces(t *testing.T) {
	function := "void CPopulous::fixture() {\n" +
		"// } ignored\n/* { ignored } */\n" +
		"const char *text = \"escaped \\\" }\"; char brace = '}';\n" +
		"if (true) { /* nested */ }\n}"
	input := []byte("// lead\n" + function + "\nvoid after() {}")
	got, line, err := extractCPPOracleFunction(input, "fixture")
	if err != nil || line != 2 || string(got) != function {
		t.Fatalf("extraction changed function bytes: line=%d error=%v body=%q", line, err, got)
	}
	if _, _, err := extractCPPOracleFunction([]byte("void CPopulous::fixture() { /* unterminated"), "fixture"); err == nil {
		t.Fatal("accepted an unterminated source body")
	}
}
