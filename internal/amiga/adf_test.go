package amiga

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func syntheticDisk(ffs bool, length int) ([]byte, []byte) {
	capacity := 488
	if ffs {
		capacity = 512
	}
	count := (length + capacity - 1) / capacity
	extensions := (max(1, count)+71)/72 - 1
	firstData := 4 + extensions
	data := make([]byte, (firstData+count)*512)
	copy(data, "DOS")
	if ffs {
		data[3] = 1
	}
	binary.BigEndian.PutUint32(data[8:], 2)
	payload := make([]byte, length)
	for i := range payload {
		payload[i] = byte(i*73 + 19)
	}
	put := func(block, index int, value uint32) { binary.BigEndian.PutUint32(data[block*512+index*4:], value) }
	name := func(block int, value string) {
		data[block*512+432] = byte(len(value))
		copy(data[block*512+433:], value)
	}
	put(2, 0, 2)
	put(2, 3, 72)
	put(2, 6, 3)
	put(2, 127, 1)
	name(2, "TEST")
	for e := 0; e <= extensions; e++ {
		block := 3 + e
		kind := uint32(16)
		if e == 0 {
			kind = 2
		}
		put(block, 0, kind)
		put(block, 1, uint32(block))
		put(block, 127, 0xfffffffd)
		local := min(72, count-e*72)
		put(block, 2, uint32(max(0, local)))
		if e == 0 {
			put(block, 125, 2)
			put(block, 81, uint32(length))
			name(block, "Demo.BIN")
			if count > 0 {
				put(block, 4, uint32(firstData))
			}
		} else {
			put(block, 125, 3)
		}
		if e < extensions {
			put(block, 126, uint32(block+1))
		}
		for n := 0; n < local; n++ {
			put(block, 77-n, uint32(firstData+e*72+n))
		}
		fixChecksum(data, block)
	}
	for n := 0; n < count; n++ {
		block := firstData + n
		offset := block * 512
		part := payload[n*capacity : min(length, (n+1)*capacity)]
		if ffs {
			copy(data[offset:], part)
		} else {
			put(block, 0, 8)
			put(block, 1, 3)
			put(block, 2, uint32(n+1))
			put(block, 3, uint32(len(part)))
			if n+1 < count {
				put(block, 4, uint32(block+1))
			}
			copy(data[offset+24:], part)
			fixChecksum(data, block)
		}
	}
	fixChecksum(data, 2)
	return data, payload
}

func fixChecksum(data []byte, block int) {
	b := data[block*512 : (block+1)*512]
	binary.BigEndian.PutUint32(b[20:], 0)
	var sum uint32
	for i := 0; i < 128; i++ {
		sum += word(b, i)
	}
	binary.BigEndian.PutUint32(b[20:], -sum)
}

func TestDiskReadsOFSAndFFSExtensions(t *testing.T) {
	for _, ffs := range []bool{false, true} {
		for _, length := range []int{0, 53, 491, 75*512 + 3} {
			data, payload := syntheticDisk(ffs, length)
			d, err := ParseDisk(data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.ReadFile("demo.bin")
			if err != nil {
				t.Fatalf("FFS=%t length=%d: %v", ffs, length, err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("FFS=%t length=%d: payload mismatch", ffs, length)
			}
			if d.Name != "TEST" || len(d.Entries()) != 1 {
				t.Fatal("incorrect volume inventory")
			}
			// Mutating the caller's image cannot corrupt the parsed disk.
			clear(data)
			again, err := d.ReadFile("DEMO.BIN")
			if err != nil || !bytes.Equal(again, payload) {
				t.Fatal("disk aliases caller's buffer")
			}
		}
	}
}

func TestDiskRejectsBrokenHeadersAndChains(t *testing.T) {
	cases := []struct {
		name   string
		mutate func([]byte)
		atRead bool
	}{
		{"checksum", func(b []byte) { b[2*512+10] ^= 1 }, false},
		{"directory cycle", func(b []byte) { binary.BigEndian.PutUint32(b[3*512+496:], 3); fixChecksum(b, 3) }, false},
		{"unsafe name", func(b []byte) { copy(b[3*512+433:], "../oops!"); fixChecksum(b, 3) }, false},
		{"data checksum", func(b []byte) { b[4*512+25] ^= 1 }, true},
		{"outside data", func(b []byte) { binary.BigEndian.PutUint32(b[3*512+308:], 10000); fixChecksum(b, 3) }, true},
		{"wrong sequence", func(b []byte) { binary.BigEndian.PutUint32(b[4*512+8:], 7); fixChecksum(b, 4) }, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, _ := syntheticDisk(false, 500)
			test.mutate(data)
			d, err := ParseDisk(data)
			if test.atRead && err == nil {
				_, err = d.ReadFile("Demo.BIN")
			}
			if err == nil {
				t.Fatal("corrupt image was accepted")
			}
		})
	}
}

func TestDiskUnsupportedFormatsAndMissingPaths(t *testing.T) {
	for _, data := range [][]byte{nil, make([]byte, 512), append([]byte("DOS\x06"), make([]byte, 2044)...)} {
		if _, err := ParseDisk(data); err == nil {
			t.Fatal("unsupported image accepted")
		}
	}
	data, _ := syntheticDisk(false, 1)
	d, err := ParseDisk(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"missing", "../Demo.BIN", "/Demo.BIN"} {
		if _, err := d.ReadFile(p); err == nil {
			t.Fatalf("invalid path %q accepted", p)
		}
	}
}

func FuzzDisk(f *testing.F) {
	data, _ := syntheticDisk(false, 31)
	f.Add(data)
	f.Add([]byte("DOS"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2<<20 {
			t.Skip()
		}
		d, err := ParseDisk(data)
		if err != nil {
			return
		}
		for _, entry := range d.Entries() {
			if !entry.Directory {
				_, _ = d.ReadFile(entry.Path)
			}
		}
	})
}

func TestBlockNameLatin1(t *testing.T) {
	b := make([]byte, 512)
	b[432] = 4
	copy(b[433:], []byte{0xe9, 't', 'e', 's'})
	name, err := blockName(b)
	if err != nil || !strings.HasPrefix(name, "é") {
		t.Fatalf("Latin-1 name: %q %v", name, err)
	}
}
