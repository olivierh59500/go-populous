package assetimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

// fixtureADF builds a small OFS volume containing synthetic payloads only.
func fixtureADF(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	blocks := 3
	for name, payload := range files {
		names = append(names, name)
		blocks += 1 + (len(payload)+487)/488
	}
	sort.Strings(names)
	data := make([]byte, max(blocks, 4)*512)
	copy(data, "DOS\x00")
	binary.BigEndian.PutUint32(data[8:], 2)
	put := func(block, index int, value uint32) { binary.BigEndian.PutUint32(data[block*512+index*4:], value) }
	fix := func(block int) {
		put(block, 5, 0)
		var sum uint32
		for i := 0; i < 128; i++ {
			sum += binary.BigEndian.Uint32(data[block*512+i*4:])
		}
		put(block, 5, -sum)
	}
	name := func(block int, value string) {
		data[block*512+432] = byte(len(value))
		copy(data[block*512+433:], value)
	}
	put(2, 0, 2)
	put(2, 3, 72)
	put(2, 127, 1)
	name(2, "Synthetic Populous resources")
	next := 3
	previous := 2
	for _, filename := range names {
		header := next
		payload := files[filename]
		count := (len(payload) + 487) / 488
		if count > 72 {
			panic("synthetic fixture needs extension support")
		}
		if previous == 2 {
			put(2, 6, uint32(header))
		} else {
			put(previous, 124, uint32(header))
			fix(previous)
		}
		next++
		put(header, 0, 2)
		put(header, 1, uint32(header))
		put(header, 2, uint32(count))
		put(header, 81, uint32(len(payload)))
		put(header, 125, 2)
		put(header, 127, 0xfffffffd)
		name(header, filename)
		if count != 0 {
			put(header, 4, uint32(next))
		}
		for i := 0; i < count; i++ {
			block := next
			next++
			put(header, 77-i, uint32(block))
			part := payload[i*488 : min((i+1)*488, len(payload))]
			put(block, 0, 8)
			put(block, 1, uint32(header))
			put(block, 2, uint32(i+1))
			put(block, 3, uint32(len(part)))
			if i+1 < count {
				put(block, 4, uint32(next))
			}
			copy(data[block*512+24:], part)
			fix(block)
		}
		fix(header)
		previous = header
	}
	fix(2)
	return data
}

func localRoot(t *testing.T) string {
	t.Helper()
	// macOS commonly exposes its temporary directory through /var -> /private/var.
	// Destination symlinks are deliberately forbidden, so use its real pathname.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func syntheticInputs(t *testing.T) (string, string, []Fingerprint, map[string][]byte) {
	t.Helper()
	root := localRoot(t)
	payloads := map[string][]byte{"FONT.DAT": bytes.Repeat([]byte{0x19, 0x42, 0x87, 0x73}, 210), "land0": []byte("synthetic terrain samples"), "populous": []byte("not imported executable")}
	var specs []Fingerprint
	for _, name := range []string{"FONT.DAT", "land0"} {
		p := payloads[name]
		specs = append(specs, Fingerprint{Name: strings.ToLower(name), Size: len(p), SHA256: fmt.Sprintf("%x", sha256.Sum256(p))})
	}
	adf := filepath.Join(root, "original.adf")
	if err := os.WriteFile(adf, fixtureADF(payloads), 0644); err != nil {
		t.Fatal(err)
	}
	return adf, filepath.Join(root, "resources"), specs, payloads
}

func TestImportSyntheticDiskOnlyRequiredResourcesAndRepeat(t *testing.T) {
	adf, output, specs, payloads := syntheticInputs(t)
	for repeat := 0; repeat < 2; repeat++ {
		count, err := importFiles(Config{ADFs: []string{adf}, Output: output}, specs)
		if err != nil || count != 2 {
			t.Fatalf("import %d: count=%d error=%v", repeat, count, err)
		}
		entries, err := os.ReadDir(output)
		if err != nil || len(entries) != 2 {
			t.Fatalf("unrelated or temporary files were imported: %v %v", entries, err)
		}
		for name, expected := range payloads {
			if name == "populous" {
				continue
			}
			actual, err := os.ReadFile(filepath.Join(output, strings.ToLower(name)))
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatalf("resource %s differs: %v", name, err)
			}
		}
	}
}

func TestImportRejectsMissingAndDamagedFilesBeforeWriting(t *testing.T) {
	adf, output, specs, _ := syntheticInputs(t)
	cases := []struct {
		name   string
		config Config
		specs  []Fingerprint
	}{
		{"no disks", Config{Output: output}, specs},
		{"missing resource", Config{ADFs: []string{adf}, Output: output}, append(append([]Fingerprint(nil), specs...), Fingerprint{Name: "missing", Size: 1, SHA256: strings.Repeat("0", 64)})},
		{"damaged resource", Config{ADFs: []string{adf}, Output: output}, []Fingerprint{{Name: specs[0].Name, Size: specs[0].Size, SHA256: strings.Repeat("0", 64)}}},
		{"unsafe filename", Config{ADFs: []string{adf}, Output: output}, []Fingerprint{{Name: "../font.dat", Size: 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := importFiles(c.config, c.specs); err == nil {
				t.Fatal("invalid import succeeded")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("invalid import touched output: %v", err)
			}
		})
	}
}

func TestImportDryRunAndConflictingOutput(t *testing.T) {
	adf, output, specs, _ := syntheticInputs(t)
	if _, err := importFiles(Config{ADFs: []string{adf}, Output: output, DryRun: true}, specs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("dry run created an output directory")
	}
	if err := os.Mkdir(output, 0755); err != nil {
		t.Fatal(err)
	}
	conflicting := filepath.Join(output, specs[1].Name)
	if err := os.WriteFile(conflicting, []byte("keep my existing file"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{true, false} {
		if _, err := importFiles(Config{ADFs: []string{adf}, Output: output, DryRun: dry}, specs); err == nil {
			t.Fatal("conflict was accepted")
		}
		got, err := os.ReadFile(conflicting)
		if err != nil || string(got) != "keep my existing file" {
			t.Fatal("existing file was overwritten", err)
		}
		entries, err := os.ReadDir(output)
		if err != nil || len(entries) != 1 {
			t.Fatal("failed import left partial output", entries, err)
		}
	}
}

func TestImportRejectsOutputSymlinks(t *testing.T) {
	adf, output, specs, _ := syntheticInputs(t)
	target := filepath.Join(filepath.Dir(output), "outside")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, output); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	if _, err := importFiles(Config{ADFs: []string{adf}, Output: filepath.Join(output, "nested")}, specs); err == nil {
		t.Fatal("symlinked directory was accepted")
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(target, "original")
	if err := os.WriteFile(original, []byte("do not alter"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, filepath.Join(output, specs[0].Name)); err != nil {
		t.Fatal(err)
	}
	if _, err := importFiles(Config{ADFs: []string{adf}, Output: output}, specs); err == nil {
		t.Fatal("symlinked file was accepted")
	}
	if got, err := os.ReadFile(original); err != nil || string(got) != "do not alter" {
		t.Fatal("symlink target changed", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 1 {
		t.Fatal("symlinked import escaped its output", entries, err)
	}
}

func TestManifestAndValidateRequireAllFifteenResources(t *testing.T) {
	if len(Files) != 15 {
		t.Fatalf("manifest count %d", len(Files))
	}
	seen := map[string]bool{}
	for _, spec := range Files {
		if spec.Name != filepath.Base(spec.Name) || spec.Size <= 0 || len(spec.SHA256) != 64 || seen[spec.Name] {
			t.Fatal("invalid fingerprint", spec)
		}
		seen[spec.Name] = true
		if err := check(spec, []byte("synthetic invalid bytes")); err == nil {
			t.Fatal("different resource was accepted")
		}
	}
	if err := Validate(fstest.MapFS{}); err == nil {
		t.Fatal("empty installation validated")
	}
}

// This comparison uses an optional private ADF; no original data are fixtures.
func TestPrivateOriginalADFRecreatesEveryResource(t *testing.T) {
	input := os.Getenv("POPULOUS_ADF_TEST")
	if input == "" {
		t.Skip("set POPULOUS_ADF_TEST to a compatible local original ADF")
	}
	output := filepath.Join(localRoot(t), "resources")
	count, err := Import(Config{ADFs: []string{input}, Output: output})
	if err != nil || count != 15 {
		t.Fatalf("original import count=%d error=%v", count, err)
	}
	if err := Validate(os.DirFS(output)); err != nil {
		t.Fatal(err)
	}
	if baseline := os.Getenv("POPULOUS_ASSET_TEST_DIR"); baseline != "" {
		for _, spec := range Files {
			actual, err := os.ReadFile(filepath.Join(output, spec.Name))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(filepath.Join(baseline, spec.Name))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, expected) {
				t.Fatalf("resource %s differs from the original build input", spec.Name)
			}
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 15 {
		t.Fatalf("non-resource disk contents were extracted: %v %v", entries, err)
	}
}
