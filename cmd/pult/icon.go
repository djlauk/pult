package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
)

// The tray icon is a 2×2 grid of tiles, drawn in code so there is no image
// asset to maintain.
// Four shades of green, light to dark in reading order.
var tileColors = [2][2]color.NRGBA{
	{{0x86, 0xd1, 0x8f, 0xff}, {0x4c, 0xb0, 0x5c, 0xff}},
	{{0x2e, 0x8b, 0x43, 0xff}, {0x1b, 0x5e, 0x2c, 0xff}},
}

func drawIcon(n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	margin := n / 8
	gap := max(1, n/12)
	tile := (n - 2*margin - gap) / 2
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			c := tileColors[row][col]
			x0 := margin + col*(tile+gap)
			y0 := margin + row*(tile+gap)
			for y := y0; y < y0+tile; y++ {
				for x := x0; x < x0+tile; x++ {
					img.SetNRGBA(x, y, c)
				}
			}
		}
	}
	return img
}

var faviconICO = iconICO()

// handleFavicon serves the tray icon as the web UI's favicon, so both
// always look the same.
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "max-age=86400")
	w.Write(faviconICO)
}

// iconPNG returns the icon as PNG (used by the Linux tray).
func iconPNG() []byte {
	var buf bytes.Buffer
	png.Encode(&buf, drawIcon(32))
	return buf.Bytes()
}

// iconICO returns the icon as a Windows .ico with 16, 32 and 48 px images,
// each a 32-bit BGRA bitmap.
func iconICO() []byte {
	sizes := []int{16, 32, 48}
	var images [][]byte
	for _, n := range sizes {
		images = append(images, icoBitmap(drawIcon(n)))
	}

	var buf bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&buf, le, [3]uint16{0, 1, uint16(len(sizes))}) // reserved, type=icon, count
	offset := 6 + 16*len(sizes)
	for i, n := range sizes {
		binary.Write(&buf, le, struct {
			W, H, Colors, Reserved uint8
			Planes, BitCount       uint16
			Size, Offset           uint32
		}{uint8(n), uint8(n), 0, 0, 1, 32, uint32(len(images[i])), uint32(offset)})
		offset += len(images[i])
	}
	for _, b := range images {
		buf.Write(b)
	}
	return buf.Bytes()
}

// icoBitmap encodes an image as the BITMAPINFOHEADER + pixels + AND mask
// that an .ico entry holds (rows bottom-up, height doubled for the mask).
func icoBitmap(img *image.NRGBA) []byte {
	n := img.Bounds().Dx()
	var buf bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&buf, le, struct {
		Size                  uint32
		Width, Height         int32
		Planes, BitCount      uint16
		Compression, ImgSize  uint32
		XPels, YPels          int32
		ClrUsed, ClrImportant uint32
	}{40, int32(n), int32(2 * n), 1, 32, 0, 0, 0, 0, 0, 0})
	for y := n - 1; y >= 0; y-- {
		for x := 0; x < n; x++ {
			c := img.NRGBAAt(x, y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	maskRow := (n + 31) / 32 * 4       // 1 bit per pixel, rows padded to 4 bytes
	buf.Write(make([]byte, maskRow*n)) // all zero: alpha channel decides
	return buf.Bytes()
}
