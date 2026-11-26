package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// Sender logos: the mail list, search results and reader show the sender
// domain's brand logo in place of the initials avatar, as Apple Mail does.
// BIMI (Brand Indicators for Message Identification) is the preferred
// source: a brand publishes an SVG in a DNS TXT record at
// default._bimi.<domain>. When there is no BIMI record we fall back to the
// domain's favicon. Fetches happen in the background and are cached on
// disk; anything unusable falls back to the initials avatar.

type logoStatus uint8

const (
	logoQueued logoStatus = iota
	logoReady
	logoFailed
)

type logoEntry struct {
	status logoStatus
	png    []byte
	svg    []byte
	bitmap *ui.Bitmap
	vector *ui.SVG
}

// logoCache maps a sender domain to its cached logo. The bytes are shared
// with the fetch goroutine; the decoded bitmap and parsed SVG are only
// touched by the UI thread.
type logoCache struct {
	mu      sync.Mutex
	entries map[string]*logoEntry
}

type logoAsset struct {
	png []byte
	svg []byte
}

var (
	logoClient     = &http.Client{Timeout: 7 * time.Second}
	logoFetchSlots = make(chan struct{}, 4)
	pngSignature   = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

// senderAvatar shows the sender's brand logo when BIMI or its favicon
// provides one, and the usual initials avatar otherwise.
func (a *App) senderAvatar(c *ui.Context, name, addr string, size float32) ui.Element {
	src := a.senderLogo(addr)
	if src == nil {
		return ui.Avatar(c, name, nil).Size(size, size)
	}
	scale := float32(0.6)
	if _, ok := src.(*ui.SVG); ok {
		// BIMI logos are designed to fill their square; favicons usually
		// carry padding of their own and look better smaller.
		scale = 0.82
	}
	return ui.Box(c).Size(size, size).Radius(size / 2).
		Background(ui.Hex("#ffffff")).TextColor(ui.Hex("#111111")).
		Clip().Shrink(0).Center().Label(name).
		Children(func() {
			ui.Image(c, src).Fit(ui.Contain).Size(size*scale, size*scale)
		})
}

// senderLogo returns the cached logo for addr's domain, starting a
// background load the first time it is seen. It returns nil while the
// logo is queued, failed, or when the domain should keep its initials.
func (a *App) senderLogo(addr string) ui.ImageSource {
	domain := senderDomain(addr)
	if domain == "" || isPersonalDomain(domain) {
		return nil
	}

	a.logos.mu.Lock()
	if a.logos.entries == nil {
		a.logos.entries = make(map[string]*logoEntry)
	}
	ent := a.logos.entries[domain]
	if ent == nil {
		a.logos.entries[domain] = &logoEntry{status: logoQueued}
		a.logos.mu.Unlock()
		go a.loadLogo(domain)
		return nil
	}
	if ent.bitmap == nil && ent.vector == nil && ent.status == logoReady {
		switch {
		case len(ent.svg) > 0:
			if v, err := ui.ParseSVG(ent.svg); err == nil {
				ent.vector = v
			} else {
				ent.svg = nil
			}
		case len(ent.png) > 0:
			if bmp, err := ui.DecodeBitmap(ent.png); err == nil {
				ent.bitmap = bmp
			} else {
				ent.png = nil
			}
		}
		if ent.bitmap == nil && ent.vector == nil {
			ent.status = logoFailed
		}
	}
	var src ui.ImageSource
	switch {
	case ent.vector != nil:
		src = ent.vector
	case ent.bitmap != nil:
		src = ent.bitmap
	}
	a.logos.mu.Unlock()
	return src
}

// loadLogo prefers a cached BIMI logo, then shows a cached favicon, then
// looks for BIMI, and finally fetches the favicon when neither exists.
func (a *App) loadLogo(domain string) {
	logoFetchSlots <- struct{}{}
	defer func() { <-logoFetchSlots }()

	if data := readLogoCache(domain, "svg"); len(data) > 0 {
		a.finishLogo(domain, logoAsset{svg: data}, logoReady)
		return
	}

	haveFavicon := false
	if data := readLogoCache(domain, "png"); len(data) > 0 {
		a.finishLogo(domain, logoAsset{png: data}, logoReady)
		haveFavicon = true
	}

	if data := fetchBIMILogo(domain); len(data) > 0 {
		writeLogoCache(domain, "svg", data)
		a.finishLogo(domain, logoAsset{svg: data}, logoReady)
		return
	}
	if haveFavicon {
		return
	}

	img := fetchFaviconImage(domain)
	if img == nil {
		a.finishLogo(domain, logoAsset{}, logoFailed)
		return
	}
	data, err := encodeLogoPNG(img)
	if err != nil {
		a.finishLogo(domain, logoAsset{}, logoFailed)
		return
	}
	writeLogoCache(domain, "png", data)
	a.finishLogo(domain, logoAsset{png: data}, logoReady)
}

func (a *App) finishLogo(domain string, asset logoAsset, status logoStatus) {
	a.logos.mu.Lock()
	if ent := a.logos.entries[domain]; ent != nil {
		ent.png, ent.svg = asset.png, asset.svg
		// The bytes changed: let the UI thread decode or parse the new
		// source. This also replaces a favicon with a BIMI logo when the
		// DNS lookup finishes after the favicon was shown.
		ent.bitmap, ent.vector = nil, nil
		ent.status = status
	}
	a.logos.mu.Unlock()
	if status == logoReady {
		a.invalidate()
	}
}

// fetchBIMILogo looks up the domain's BIMI record and returns its SVG
// logo, if one is published and understood by the SVG renderer.
func fetchBIMILogo(domain string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	records, err := net.DefaultResolver.LookupTXT(ctx, "default._bimi."+domain)
	if err != nil {
		return nil
	}
	for _, record := range records {
		url := bimiLogoURL(record)
		if url == "" {
			continue
		}
		data, ok := logoGet(url)
		if !ok {
			continue
		}
		if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
			if unzipped, err := gunzipLogo(data); err == nil {
				data = unzipped
			} else {
				continue
			}
		}
		if _, err := ui.ParseSVG(data); err != nil {
			continue
		}
		return data
	}
	return nil
}

func gunzipLogo(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(io.LimitReader(zr, 512<<10))
}

// bimiLogoURL returns the HTTPS logo URL from a BIMI TXT record, or "".
// The record looks like "v=BIMI1; l=https://example.com/logo.svg; a=...".
func bimiLogoURL(record string) string {
	version, logo := "", ""
	for _, part := range strings.Split(record, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "v":
			version = strings.ToUpper(strings.TrimSpace(value))
		case "l":
			logo = strings.TrimSpace(value)
		}
	}
	if version != "" && version != "BIMI1" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(logo), "https://") {
		return ""
	}
	return logo
}

func fetchFaviconImage(domain string) image.Image {
	hosts := []string{domain}
	if strings.HasPrefix(domain, "www.") {
		if h := strings.TrimPrefix(domain, "www."); h != "" {
			hosts = append(hosts, h)
		}
	} else {
		hosts = append(hosts, "www."+domain)
	}
	for _, host := range hosts {
		for _, path := range []string{"/favicon.ico", "/favicon.png"} {
			data, ok := logoGet("https://" + host + path)
			if !ok {
				continue
			}
			if img, err := decodeLogo(data); err == nil {
				return img
			}
		}
	}
	return nil
}

func logoGet(rawURL string) ([]byte, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || !safeLogoHost(u.Hostname()) {
		return nil, false
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GoMail/1.0)")
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	resp, err := logoClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// safeLogoHost keeps a BIMI or favicon URL from reaching local and private
// addresses, where a crafted record could probe the machine's network.
func safeLogoHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !(ip.IsPrivate() || ip.IsLoopback() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified())
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host != "localhost" && !strings.HasSuffix(host, ".local")
}

func decodeLogo(data []byte) (image.Image, error) {
	if isICO(data) {
		if img, err := decodeICO(data); err == nil {
			return img, nil
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func encodeLogoPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// senderDomain returns the lower-case host of an email address, or "" for
// anything that is not a plausible public domain.
func senderDomain(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at < 0 || at == len(addr)-1 {
		return ""
	}
	domain := strings.ToLower(strings.TrimSpace(addr[at+1:]))
	domain = strings.TrimSuffix(domain, ".")
	if !validDomain(domain) {
		return ""
	}
	return domain
}

func validDomain(domain string) bool {
	if len(domain) < 4 || len(domain) > 253 {
		return false
	}
	if net.ParseIP(domain) != nil {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			r := label[i]
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	dot := strings.LastIndex(domain, ".")
	return dot >= 0 && len(domain)-dot-1 >= 2
}

// personalDomains are free and personal mail providers: their favicon is
// the provider's, not the sender's company, so these keep their initials.
var personalDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true,
	"outlook.com": true, "outlook.be": true, "hotmail.com": true, "hotmail.be": true,
	"live.com": true, "live.be": true, "msn.com": true,
	"icloud.com": true, "me.com": true, "mac.com": true,
	"aol.com": true, "aol.be": true,
	"yahoo.com": true, "yahoo.be": true, "yahoo.co.uk": true, "yahoo.fr": true,
	"yahoo.de": true, "yahoo.es": true, "yahoo.it": true, "yahoo.nl": true,
	"ymail.com": true, "rocketmail.com": true,
	"gmx.com": true, "gmx.net": true, "gmx.de": true, "gmx.at": true,
	"web.de": true, "t-online.de": true,
	"mail.ru": true, "yandex.ru": true, "yandex.com": true,
	"protonmail.com": true, "proton.me": true, "pm.me": true,
	"tutanota.com": true, "tuta.io": true,
	"zoho.com": true, "fastmail.com": true, "hey.com": true,
	"skynet.be": true, "skylnet.be": true, "telenet.be": true,
	"proximus.be": true, "scarlet.be": true, "edpnet.be": true, "base.be": true,
	"example.com": true, "example.org": true, "example.net": true,
	"localhost": true,
}

func isPersonalDomain(domain string) bool {
	if personalDomains[domain] {
		return true
	}
	switch {
	case strings.HasSuffix(domain, ".local"),
		strings.HasSuffix(domain, ".localhost"),
		strings.HasSuffix(domain, ".invalid"),
		strings.HasSuffix(domain, ".test"),
		strings.HasSuffix(domain, ".example"):
		return true
	}
	return false
}

func logoCacheDir() string {
	if dir := os.Getenv("GO_MAIL_LOGOS"); dir != "" {
		return dir
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, appDataDir, "logos")
}

func logoCachePath(domain, ext string) string {
	dir := logoCacheDir()
	if dir == "" || !validDomain(domain) {
		return ""
	}
	return filepath.Join(dir, domain+"."+ext)
}

func readLogoCache(domain, ext string) []byte {
	path := logoCachePath(domain, ext)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	switch ext {
	case "png":
		if !bytes.HasPrefix(data, pngSignature) {
			return nil
		}
	case "svg":
		if !bytes.Contains(data, []byte("<svg")) {
			return nil
		}
	}
	return data
}

func writeLogoCache(domain, ext string, data []byte) {
	path := logoCachePath(domain, ext)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func isICO(data []byte) bool {
	return len(data) >= 6 &&
		binary.LittleEndian.Uint16(data[0:2]) == 0 &&
		binary.LittleEndian.Uint16(data[2:4]) == 1 &&
		binary.LittleEndian.Uint16(data[4:6]) > 0
}

// decodeICO decodes the best (largest) image in a Windows icon file,
// including PNG-compressed entries and the usual 24/32bpp BMP entries.
func decodeICO(data []byte) (image.Image, error) {
	if !isICO(data) {
		return nil, errors.New("not an ICO file")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	bestW, bestH, bestOff, bestSize := 0, 0, 0, 0
	for i := 0; i < count; i++ {
		p := 6 + i*16
		if p+16 > len(data) {
			break
		}
		w, h := int(data[p]), int(data[p+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		size := int(binary.LittleEndian.Uint32(data[p+8 : p+12]))
		off := int(binary.LittleEndian.Uint32(data[p+12 : p+16]))
		if size <= 0 || off < 0 || off > len(data) || size > len(data)-off {
			continue
		}
		if w*h > bestW*bestH {
			bestW, bestH, bestOff, bestSize = w, h, off, size
		}
	}
	if bestSize == 0 {
		return nil, errors.New("ICO has no usable image")
	}
	blob := data[bestOff : bestOff+bestSize]
	if bytes.HasPrefix(blob, pngSignature) {
		img, _, err := image.Decode(bytes.NewReader(blob))
		return img, err
	}
	return decodeICOBitmap(blob)
}

func decodeICOBitmap(b []byte) (image.Image, error) {
	if len(b) < 40 {
		return nil, errors.New("ICO bitmap too short")
	}
	hdrSize := int(binary.LittleEndian.Uint32(b[0:4]))
	if hdrSize < 40 || hdrSize > len(b) {
		return nil, errors.New("bad ICO bitmap header")
	}
	w := int(int32(binary.LittleEndian.Uint32(b[4:8])))
	h := int(int32(binary.LittleEndian.Uint32(b[8:12])))
	bpp := int(binary.LittleEndian.Uint16(b[14:16]))
	compression := binary.LittleEndian.Uint32(b[16:20])
	if w <= 0 || w > 1024 || h == 0 {
		return nil, errors.New("bad ICO bitmap size")
	}
	if h < 0 {
		h = -h
	}
	h /= 2
	if h <= 0 || h > 1024 {
		return nil, errors.New("bad ICO bitmap height")
	}
	if compression != 0 && compression != 3 {
		return nil, fmt.Errorf("unsupported ICO compression %d", compression)
	}
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("unsupported ICO bit depth %d", bpp)
	}

	pixelOff := hdrSize
	if compression == 3 {
		pixelOff += 12
	}
	xorStride := ((w*bpp + 31) / 32) * 4
	xorSize := xorStride * h
	if pixelOff+xorSize > len(b) {
		return nil, errors.New("short ICO pixel data")
	}
	andStride := ((w + 31) / 32) * 4
	andOff := pixelOff + xorSize
	hasMask := andOff+andStride*h <= len(b)

	hasAlpha := false
	if bpp == 32 {
		for i := 3; i < xorSize; i += 4 {
			if b[pixelOff+i] != 0 {
				hasAlpha = true
				break
			}
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := h - 1 - y // ICO bitmaps are bottom-up.
		srcRow := pixelOff + srcY*xorStride
		dstRow := y * img.Stride
		for x := 0; x < w; x++ {
			s := srcRow + x*(bpp/8)
			var r, g, bl, a uint8
			if bpp == 32 {
				bl, g, r, a = b[s], b[s+1], b[s+2], b[s+3]
				if !hasAlpha {
					a = 255
				}
			} else {
				bl, g, r, a = b[s], b[s+1], b[s+2], 255
			}
			if hasMask && maskBit(b, andOff, srcY, andStride, x) {
				a = 0
			}
			d := dstRow + x*4
			img.Pix[d], img.Pix[d+1], img.Pix[d+2], img.Pix[d+3] = r, g, bl, a
		}
	}
	return img, nil
}

func maskBit(b []byte, off, row, stride, x int) bool {
	i := off + row*stride + x/8
	if i < 0 || i >= len(b) {
		return false
	}
	return b[i]&(0x80>>uint(x%8)) != 0
}
