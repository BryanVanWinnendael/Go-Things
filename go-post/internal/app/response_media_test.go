package app

import (
	"bytes"
	"go-post/internal/core"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egoist/mygo/ui"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An image response keeps its bytes, decodes once off the UI thread, and takes
// its file name from Content-Disposition.
func TestSendReadsImageResponse(t *testing.T) {
	data := testPNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", `inline; filename="chart.png"`)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	res := (&core.Store{}).Send(core.Request{Method: "GET", URL: srv.URL + "/render"}, nil, core.DefaultData().Settings, nil)
	if res.Error != "" {
		t.Fatalf("send failed: %s", res.Error)
	}
	if res.Bitmap == nil {
		t.Fatal("image response has no bitmap")
	}
	if w, h := res.Bitmap.Size(); w != 4 || h != 3 {
		t.Fatalf("bitmap is %dx%d, want 4x3", w, h)
	}
	if res.MIME != "image/png" {
		t.Fatalf("MIME is %q", res.MIME)
	}
	if res.Filename != "chart.png" {
		t.Fatalf("file name is %q", res.Filename)
	}
	if !bytes.Equal(res.Bytes, data) {
		t.Fatal("image bytes were not kept")
	}
	if res.JSON {
		t.Fatal("an image was flagged as JSON")
	}
}

// A binary body that is not an image keeps its bytes and gets a name from the
// URL, so the pane can offer a file card instead of mojibake.
func TestSendClassifiesBinaryResponse(t *testing.T) {
	data := []byte("%PDF-1.4\nnot really a pdf")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	res := (&core.Store{}).Send(core.Request{Method: "GET", URL: srv.URL + "/files/report.pdf"}, nil, core.DefaultData().Settings, nil)
	if res.Error != "" {
		t.Fatalf("send failed: %s", res.Error)
	}
	if res.Bitmap != nil {
		t.Fatal("a PDF was decoded as an image")
	}
	if !bytes.Equal(res.Bytes, data) {
		t.Fatal("binary bytes were not kept")
	}
	if res.Filename != "report.pdf" {
		t.Fatalf("file name is %q", res.Filename)
	}
	if res.Body == "" || res.Body[:1] != "[" {
		t.Fatalf("binary body marker is %q", res.Body)
	}
}

// Text still reads as text even when the server names a binary type it is not:
// what matters is that the bytes are readable.
func TestTextResponseStaysText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	res := (&core.Store{}).Send(core.Request{Method: "GET", URL: srv.URL + "/greeting"}, nil, core.DefaultData().Settings, nil)
	if res.Error != "" || res.Body != "hello" {
		t.Fatalf("text response is %q (%s)", res.Body, res.Error)
	}
	if len(res.Bytes) != 0 || res.Bitmap != nil {
		t.Fatal("a text response kept binary media")
	}
}

// The pane shows an image preview with its size and a save action.
func TestResponsePaneShowsImagePreview(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	bitmap, err := ui.DecodeBitmap(testPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	a.responses[r.ID] = core.SendResult{
		StatusCode: 200, Status: "200 OK", MIME: "image/png",
		Filename: "chart.png", Size: 120, Bitmap: bitmap, Bytes: testPNG(t),
	}

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	for _, want := range []string{"image/png", "4 × 3", "Save…"} {
		if !tt.HasText(want) {
			t.Fatalf("image preview does not show %q", want)
		}
	}
}

// A non-image binary response shows a file card naming the download.
func TestResponsePaneShowsFileCard(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	a.responses[r.ID] = core.SendResult{
		StatusCode: 200, Status: "200 OK", MIME: "application/pdf",
		Filename: "report.pdf", Size: 2048, Bytes: []byte("%PDF-1.4"),
	}

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	for _, want := range []string{"report.pdf", "application/pdf", "2.0 KB", "Save…", "Open"} {
		if !tt.HasText(want) {
			t.Fatalf("file card does not show %q", want)
		}
	}
}
