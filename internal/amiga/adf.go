// Package amiga reads AmigaDOS disk images without executing their contents.
// It is independent of the renderer and game simulation.
package amiga

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const (
	BlockSize     = 512
	hashTableSize = 72
)

// Entry describes a reachable filesystem entry, rather than a header recovered
// by scanning unused sectors. Paths use '/' and retain the on-disk spelling.
type Entry struct {
	Path      string `json:"path"`
	Block     uint32 `json:"block"`
	Size      uint32 `json:"size"`
	Directory bool   `json:"directory,omitempty"`
}

// Disk supports standard 512-byte OFS and FFS images, including international
// and directory-cache variants (DOS\0 through DOS\5). Custom track loaders,
// compressed ADZ images, partitioned hard disks and filesystem links are not
// interpreted as ordinary files.
type Disk struct {
	Name   string
	Format string
	data   []byte
	root   uint32
	dos    byte
	files  map[string]Entry
}

// ParseDisk validates directory headers and follows their hash chains. File
// data and extension blocks are checked when ReadFile is called.
func ParseDisk(data []byte) (*Disk, error) {
	if len(data) < 3*BlockSize || len(data)%BlockSize != 0 {
		return nil, fmt.Errorf("ADF size %d is not a whole number of 512-byte sectors", len(data))
	}
	if !bytes.Equal(data[:3], []byte("DOS")) || data[3] > 5 {
		return nil, fmt.Errorf("unsupported disk signature %x: expected an AmigaDOS OFS/FFS image", data[:4])
	}
	d := &Disk{data: bytes.Clone(data), dos: data[3], files: make(map[string]Entry)}
	d.Format = "OFS"
	if d.dos&1 != 0 {
		d.Format = "FFS"
	}
	if d.dos >= 2 {
		d.Format += "-international"
	}
	if d.dos >= 4 {
		d.Format += "-dircache"
	}
	d.root = word(data, 2)
	if d.root == 0 {
		d.root = uint32(len(data) / BlockSize / 2)
	}
	root, err := d.checkedBlock(d.root)
	if err != nil {
		return nil, fmt.Errorf("root: %w", err)
	}
	if word(root, 0) != 2 || word(root, 127) != 1 || word(root, 3) != hashTableSize {
		return nil, fmt.Errorf("block %d is not an AmigaDOS root directory", d.root)
	}
	d.Name, err = blockName(root)
	if err != nil {
		return nil, fmt.Errorf("volume name: %w", err)
	}
	type directory struct {
		block uint32
		path  string
	}
	queue := []directory{{block: d.root}}
	seen := map[uint32]bool{d.root: true}
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		b, err := d.checkedBlock(dir.block)
		if err != nil {
			return nil, err
		}
		for bucket := 0; bucket < hashTableSize; bucket++ {
			for next := word(b, 6+bucket); next != 0; {
				if seen[next] {
					return nil, fmt.Errorf("directory %q has a cycle or repeated header at block %d", dir.path, next)
				}
				seen[next] = true
				header, err := d.checkedBlock(next)
				if err != nil {
					return nil, err
				}
				if word(header, 0) != 2 || word(header, 1) != next || word(header, 125) != dir.block {
					return nil, fmt.Errorf("invalid entry header/parent at block %d", next)
				}
				name, err := blockName(header)
				if err != nil {
					return nil, fmt.Errorf("block %d: %w", next, err)
				}
				p := name
				if dir.path != "" {
					p = dir.path + "/" + name
				}
				entry := Entry{Path: p, Block: next}
				switch int32(word(header, 127)) {
				case 2:
					entry.Directory = true
					queue = append(queue, directory{block: next, path: p})
				case -3:
					entry.Size = word(header, 81)
					if uint64(entry.Size) > uint64(len(data)) {
						return nil, fmt.Errorf("file %q is larger than its disk image", p)
					}
				default:
					return nil, fmt.Errorf("unsupported entry type %d for %q (links are not followed)", int32(word(header, 127)), p)
				}
				key := d.key(p)
				if _, exists := d.files[key]; exists {
					return nil, fmt.Errorf("duplicate case-insensitive path %q", p)
				}
				d.files[key] = entry
				next = word(header, 124)
			}
		}
	}
	return d, nil
}

// Entries returns a sorted copy, including directories.
func (d *Disk) Entries() []Entry {
	entries := make([]Entry, 0, len(d.files))
	for _, entry := range d.files {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

// ReadFile follows reverse-ordered block tables and their extensions. OFS
// additionally validates data checksums, ownership, sequence and next pointers.
// The returned bytes are independent of the immutable disk image.
func (d *Disk) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	}
	entry, ok := d.files[d.key(name)]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	if entry.Directory {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fmt.Errorf("is a directory")}
	}
	content, err := d.readEntry(entry)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: err}
	}
	return content, nil
}

func (d *Disk) readEntry(entry Entry) ([]byte, error) {
	blocks := make([]uint32, 0)
	seen := make(map[uint32]bool)
	for index := entry.Block; index != 0; {
		if seen[index] {
			return nil, fmt.Errorf("cyclic file extension at block %d", index)
		}
		seen[index] = true
		b, err := d.checkedBlock(index)
		if err != nil {
			return nil, err
		}
		wantType := uint32(16)
		if index == entry.Block {
			wantType = 2
		}
		if word(b, 0) != wantType || word(b, 1) != index || int32(word(b, 127)) != -3 {
			return nil, fmt.Errorf("invalid file header/extension at block %d", index)
		}
		if index != entry.Block && word(b, 125) != entry.Block {
			return nil, fmt.Errorf("file extension %d belongs to another file", index)
		}
		count := word(b, 2)
		if count > hashTableSize {
			return nil, fmt.Errorf("invalid block count %d at block %d", count, index)
		}
		for i := uint32(0); i < count; i++ {
			block := word(b, 77-int(i))
			if block == 0 {
				return nil, fmt.Errorf("missing file data pointer at block %d", index)
			}
			blocks = append(blocks, block)
		}
		index = word(b, 126)
	}
	capacity := BlockSize
	if d.dos&1 == 0 {
		capacity -= 24
	}
	expected := (uint64(entry.Size) + uint64(capacity) - 1) / uint64(capacity)
	if uint64(len(blocks)) != expected {
		return nil, fmt.Errorf("size %d needs %d data blocks, got %d", entry.Size, expected, len(blocks))
	}
	content := make([]byte, 0, int(entry.Size))
	for i, index := range blocks {
		if seen[index] {
			return nil, fmt.Errorf("repeated data/header block %d", index)
		}
		seen[index] = true
		b, err := d.block(index)
		if err != nil {
			return nil, err
		}
		length := min(capacity, int(entry.Size)-len(content))
		if d.dos&1 == 0 {
			if err := checksum(b, index); err != nil {
				return nil, err
			}
			next := uint32(0)
			if i+1 < len(blocks) {
				next = blocks[i+1]
			}
			if word(b, 0) != 8 || word(b, 1) != entry.Block || word(b, 2) != uint32(i+1) || word(b, 3) != uint32(length) || word(b, 4) != next {
				return nil, fmt.Errorf("invalid OFS data header at block %d", index)
			}
			b = b[24:]
		}
		content = append(content, b[:length]...)
	}
	if d.dos&1 == 0 {
		header, _ := d.block(entry.Block)
		first := uint32(0)
		if len(blocks) > 0 {
			first = blocks[0]
		}
		if word(header, 4) != first {
			return nil, fmt.Errorf("first OFS data pointer disagrees with block table")
		}
	}
	return content, nil
}

func (d *Disk) block(index uint32) ([]byte, error) {
	if index < 2 || uint64(index) >= uint64(len(d.data)/BlockSize) {
		return nil, fmt.Errorf("block %d is outside the filesystem", index)
	}
	start := int(index) * BlockSize
	return d.data[start : start+BlockSize], nil
}

func (d *Disk) checkedBlock(index uint32) ([]byte, error) {
	b, err := d.block(index)
	if err != nil {
		return nil, err
	}
	if err := checksum(b, index); err != nil {
		return nil, err
	}
	return b, nil
}

func checksum(b []byte, index uint32) error {
	var sum uint32
	for i := 0; i < BlockSize/4; i++ {
		sum += word(b, i)
	}
	if sum != 0 {
		return fmt.Errorf("bad checksum at block %d", index)
	}
	return nil
}

func blockName(b []byte) (string, error) {
	length := int(b[432])
	if length == 0 || length > 30 {
		return "", fmt.Errorf("invalid name length %d", length)
	}
	raw := b[433 : 433+length]
	// Convert Latin-1 to UTF-8 before JSON serialization or host extraction.
	var name strings.Builder
	for _, c := range raw {
		if c < 32 || c == 127 || c == '/' || c == ':' || c == '\\' {
			return "", fmt.Errorf("unsafe character in filesystem name")
		}
		name.WriteRune(rune(c))
	}
	if name.String() == "." || name.String() == ".." {
		return "", fmt.Errorf("unsafe filesystem name %q", name.String())
	}
	return name.String(), nil
}

func (d *Disk) key(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || d.dos >= 2 && r >= 0xe0 && r <= 0xfe && r != 0xf7 {
			return r - 32
		}
		return r
	}, name)
}

func word(b []byte, index int) uint32 {
	return binary.BigEndian.Uint32(b[index*4 : index*4+4])
}
