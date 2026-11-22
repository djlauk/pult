package brand

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

func TestIconPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("size = %v", b)
	}
}

func TestIconICO(t *testing.T) {
	ico := ICO()
	le := binary.LittleEndian
	if le.Uint16(ico[0:]) != 0 || le.Uint16(ico[2:]) != 1 || le.Uint16(ico[4:]) != 3 {
		t.Fatalf("bad header % x", ico[:6])
	}
	for i, want := range []int{16, 32, 48} {
		e := ico[6+16*i:]
		size, offset := le.Uint32(e[8:]), le.Uint32(e[12:])
		if int(e[0]) != want || int(offset+size) > len(ico) {
			t.Errorf("entry %d: width %d, offset %d, size %d, file %d", i, e[0], offset, size, len(ico))
		}
		// Bitmap: 40-byte header + n*n BGRA pixels + 1-bit mask padded to 4 bytes.
		mask := (want + 31) / 32 * 4 * want
		if int(size) != 40+want*want*4+mask {
			t.Errorf("entry %d: size %d", i, size)
		}
	}
}
