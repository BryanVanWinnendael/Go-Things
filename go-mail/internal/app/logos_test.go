package app

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestSenderDomain(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"billing@amazon.com.be", "amazon.com.be"},
		{"No-reply@GitHub.com", "github.com"},
		{"info@sub.example.co.uk", "sub.example.co.uk"},
		{"a@Example.COM.", "example.com"},
		{"a@127.0.0.1", ""},
		{"a@localhost", ""},
		{"a@foo..bar", ""},
		{"a@foo_bar.com", ""},
		{"a@-bad.com", ""},
		{"a@foo.c", ""},
		{"no-at-sign", ""},
		{"trailing@", ""},
	}
	for _, tt := range tests {
		if got := senderDomain(tt.addr); got != tt.want {
			t.Errorf("senderDomain(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}

func TestIsPersonalDomain(t *testing.T) {
	for _, d := range []string{"gmail.com", "outlook.com", "skynet.be", "skylnet.be", "example.com", "box.local"} {
		if !isPersonalDomain(d) {
			t.Errorf("isPersonalDomain(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"amazon.com.be", "github.com", "ypto.be"} {
		if isPersonalDomain(d) {
			t.Errorf("isPersonalDomain(%q) = true, want false", d)
		}
	}
}

func TestBIMILogoURL(t *testing.T) {
	tests := []struct {
		record string
		want   string
	}{
		{"v=BIMI1; l=https://example.com/logo.svg; a=https://example.com/vmc.pem", "https://example.com/logo.svg"},
		{"v=bimi1;l=https://cdn.example.com/a.svg", "https://cdn.example.com/a.svg"},
		{"l=https://example.com/logo.svg", "https://example.com/logo.svg"},
		{"v=BIMI2; l=https://example.com/logo.svg", ""},
		{"v=BIMI1; l=http://example.com/logo.svg", ""},
		{"v=BIMI1; l=", ""},
		{"v=BIMI1; a=https://example.com/vmc.pem", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := bimiLogoURL(tt.record); got != tt.want {
			t.Errorf("bimiLogoURL(%q) = %q, want %q", tt.record, got, tt.want)
		}
	}
}

func TestSafeLogoHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"example.com", true},
		{"cdn.example.com", true},
		{"localhost", false},
		{"localhost.", false},
		{"box.local", false},
		{"127.0.0.1", false},
		{"10.0.0.8", false},
		{"169.254.169.254", false},
		{"::1", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := safeLogoHost(tt.host); got != tt.want {
			t.Errorf("safeLogoHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestDecodeICO32Bit(t *testing.T) {
	// 2x2, 32bpp BGRA. ICO rows are bottom-up.
	pixels := []byte{
		255, 0, 0, 255, 255, 255, 255, 0, // bottom: blue, transparent white
		0, 0, 255, 255, 0, 255, 0, 255, // top: red, green
	}
	ico := makeBMPICO(2, 2, 32, pixels, nil)
	img, err := decodeICO(ico)
	if err != nil {
		t.Fatalf("decodeICO: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 2 || got.Dy() != 2 {
		t.Fatalf("size = %v, want 2x2", got)
	}
	checkColor(t, img, 0, 0, color.RGBA{R: 255, A: 255})
	checkColor(t, img, 1, 0, color.RGBA{G: 255, A: 255})
	checkColor(t, img, 0, 1, color.RGBA{B: 255, A: 255})
	checkColor(t, img, 1, 1, color.RGBA{})
}

func TestDecodeICO24BitMask(t *testing.T) {
	// 2x1, 24bpp BGR with an AND mask that hides the right pixel.
	pixels := []byte{
		0, 0, 255, // left: red
		0, 255, 0, // right: green
		0, 0, // stride padding
	}
	mask := []byte{0x40, 0, 0, 0} // bit 1 set: right pixel transparent
	ico := makeBMPICO(2, 1, 24, pixels, mask)
	img, err := decodeICO(ico)
	if err != nil {
		t.Fatalf("decodeICO: %v", err)
	}
	checkColor(t, img, 0, 0, color.RGBA{R: 255, A: 255})
	checkColor(t, img, 1, 0, color.RGBA{})
}

func TestDecodeICOPNGEntry(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	ico := makePNGICO(1, 1, buf.Bytes())
	img, err := decodeICO(ico)
	if err != nil {
		t.Fatalf("decodeICO: %v", err)
	}
	checkColor(t, img, 0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
}

func TestDecodeLogoRejectsGarbage(t *testing.T) {
	if _, err := decodeLogo([]byte("not an image")); err == nil {
		t.Fatal("decodeLogo accepted garbage")
	}
}

func checkColor(t *testing.T, img image.Image, x, y int, want color.RGBA) {
	t.Helper()
	r, g, b, a := img.At(x, y).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	if got != want {
		t.Errorf("pixel (%d,%d) = %#v, want %#v", x, y, got, want)
	}
}

func makeBMPICO(w, h, bpp int, pixels, mask []byte) []byte {
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:4], 40)
	binary.LittleEndian.PutUint32(header[4:8], uint32(w))
	binary.LittleEndian.PutUint32(header[8:12], uint32(h*2))
	binary.LittleEndian.PutUint16(header[12:14], 1)
	binary.LittleEndian.PutUint16(header[14:16], uint16(bpp))

	blob := append(header, pixels...)
	blob = append(blob, mask...)
	return makeICO(w, h, blob)
}

func makePNGICO(w, h int, data []byte) []byte {
	return makeICO(w, h, data)
}

func makeICO(w, h int, blob []byte) []byte {
	out := make([]byte, 6+16+len(blob))
	binary.LittleEndian.PutUint16(out[0:2], 0)
	binary.LittleEndian.PutUint16(out[2:4], 1)
	binary.LittleEndian.PutUint16(out[4:6], 1)
	out[6] = byte(w)
	out[7] = byte(h)
	binary.LittleEndian.PutUint16(out[6+4:6+6], 1)
	binary.LittleEndian.PutUint16(out[6+6:6+8], 32)
	binary.LittleEndian.PutUint32(out[6+8:6+12], uint32(len(blob)))
	binary.LittleEndian.PutUint32(out[6+12:6+16], 22)
	copy(out[22:], blob)
	return out
}
