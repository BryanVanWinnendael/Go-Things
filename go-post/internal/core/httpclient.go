package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

type HeaderLine struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SendResult struct {
	StatusCode int          `json:"statusCode"`
	Status     string       `json:"status"`
	Headers    []HeaderLine `json:"headers"`
	Body       string       `json:"body"`
	Duration   int64        `json:"durationMs"`
	Size       int64        `json:"size"`
	Error      string       `json:"error"`
	FinalURL   string       `json:"finalUrl"`
	// JSON says the body is JSON that DisplayBody indented, so the response
	// pane can color it without parsing it again on the UI thread.
	JSON bool `json:"json"`
	// Bytes, MIME, Filename and Bitmap describe a binary response: an image
	// previews from Bitmap, and anything else keeps its bytes so the pane can
	// save or open it. They are in-memory only; history stores the text body.
	Bytes    []byte     `json:"-"`
	MIME     string     `json:"-"`
	Filename string     `json:"-"`
	Bitmap   *ui.Bitmap `json:"-"`
}

func (s *Store) Send(req Request, vars map[string]string, settings Settings, cert *Certificate) SendResult {
	start := time.Now()
	rawURL := strings.TrimSpace(Interpolate(req.URL, vars))
	if rawURL == "" {
		return SendResult{Error: "Enter a URL to send this request"}
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	bodyText := Interpolate(req.Body, vars)
	if req.BodyType != "none" && bodyText != "" {
		body = strings.NewReader(bodyText)
	}
	rawURL = applyQueryParams(rawURL, req.Query, vars)
	r, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return SendResult{Error: "That URL is not valid: " + err.Error()}
	}
	hasCT := false
	for _, h := range req.Headers {
		if !h.Enabled || strings.TrimSpace(h.Key) == "" {
			continue
		}
		key := Interpolate(h.Key, vars)
		val := Interpolate(h.Value, vars)
		if strings.EqualFold(key, "Content-Type") {
			hasCT = true
		}
		r.Header.Set(key, val)
	}
	if body != nil && !hasCT {
		switch req.BodyType {
		case "json":
			r.Header.Set("Content-Type", "application/json")
		case "xml":
			r.Header.Set("Content-Type", "application/xml")
		default:
			r.Header.Set("Content-Type", "text/plain; charset=utf-8")
		}
	}
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", "Go Post/1.0")
	}
	switch req.AuthType {
	case "bearer":
		r.Header.Set("Authorization", "Bearer "+Interpolate(req.Bearer, vars))
	case "basic":
		r.SetBasicAuth(Interpolate(req.Username, vars), Interpolate(req.Password, vars))
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: settings.SkipVerify}
	if cert != nil && strings.EqualFold(r.URL.Scheme, "https") {
		tlsCert, chain, err := s.ClientCertificate(cert)
		if err != nil {
			return SendResult{Error: "Certificate: " + err.Error()}
		}
		tlsConfig.Certificates = []tls.Certificate{*tlsCert}
		if len(chain) > 0 {
			pool, _ := x509.SystemCertPool()
			if pool == nil {
				pool = x509.NewCertPool()
			}
			for _, c := range chain {
				pool.AddCert(c)
			}
			tlsConfig.RootCAs = pool
		}
	}
	transport.TLSClientConfig = tlsConfig
	client := &http.Client{Transport: transport, Timeout: time.Duration(settings.TimeoutMS) * time.Millisecond}
	if !settings.Follow {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	resp, err := client.Do(r)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "deadline exceeded") {
			msg = "The request timed out after " + fmt.Sprintf("%.0fs", float64(settings.TimeoutMS)/1000)
		}
		return SendResult{Error: msg, Duration: time.Since(start).Milliseconds(), FinalURL: rawURL}
	}
	defer resp.Body.Close()
	const maxBody = 8 << 20
	limited := io.LimitReader(resp.Body, maxBody+1)
	data, readErr := io.ReadAll(limited)
	if readErr != nil {
		return SendResult{Error: "Could not read the response: " + readErr.Error(), Duration: time.Since(start).Milliseconds(), FinalURL: resp.Request.URL.String()}
	}
	truncated := len(data) > maxBody
	if truncated {
		data = data[:maxBody]
	}
	out := SendResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Duration:   time.Since(start).Milliseconds(),
		Size:       int64(len(data)),
		FinalURL:   resp.Request.URL.String(),
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			out.Headers = append(out.Headers, HeaderLine{Key: k, Value: v})
		}
	}
	sort.Slice(out.Headers, func(i, j int) bool { return out.Headers[i].Key < out.Headers[j].Key })
	contentType := resp.Header.Get("Content-Type")
	mimeType := responseMediaType(contentType)
	if mimeType == "" {
		mimeType = responseMediaType(http.DetectContentType(data))
	}
	out.MIME = mimeType
	out.Filename = responseFilename(resp, mimeType)

	// Text keeps its readable form, with JSON indented for coloring. Anything
	// else is binary: the raw bytes stay so the pane can show an image or a
	// file card, and the text body only records what the response was.
	if responseIsText(data, mimeType) {
		text, isJSON := DisplayBody(data, contentType)
		if truncated {
			text += "\n\n… response cut off after 8 MB"
		}
		out.Body = text
		out.JSON = isJSON
	} else {
		out.Bytes = data
		if bitmap, err := ui.DecodeBitmap(data); err == nil {
			out.Bitmap = bitmap
		}
		out.Body = "[binary response: " + fmt.Sprintf("%d bytes", len(data)) + "]"
	}
	return out
}

// responseMediaType is the media type without its parameters, lowercased, or
// "" when the header does not hold one.
func responseMediaType(contentType string) string {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		if i := strings.IndexByte(contentType, ';'); i >= 0 {
			contentType = contentType[:i]
		}
		mt = strings.TrimSpace(contentType)
	}
	return strings.ToLower(mt)
}

// textualMediaType says a body reads as text even when its bytes could decode
// as an image, as SVG does.
func textualMediaType(mt string) bool {
	switch {
	case strings.HasPrefix(mt, "text/"):
		return true
	case strings.HasSuffix(mt, "+json"), strings.HasSuffix(mt, "+xml"):
		return true
	}
	switch mt {
	case "application/json", "application/xml", "application/javascript",
		"application/x-javascript", "application/ecmascript", "application/graphql",
		"application/x-www-form-urlencoded", "image/svg+xml":
		return true
	}
	return false
}

// binaryMediaType says a body is meant to be downloaded rather than read,
// whatever its bytes.
func binaryMediaType(mt string) bool {
	switch {
	case strings.HasPrefix(mt, "image/"), strings.HasPrefix(mt, "audio/"),
		strings.HasPrefix(mt, "video/"), strings.HasPrefix(mt, "font/"):
		return true
	}
	switch mt {
	case "application/pdf", "application/zip", "application/gzip", "application/x-gzip",
		"application/x-tar", "application/x-7z-compressed", "application/x-rar-compressed",
		"application/msword", "application/vnd.ms-excel", "application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/octet-stream", "application/wasm":
		return true
	}
	return false
}

func responseIsText(data []byte, mt string) bool {
	if !utf8.Valid(data) {
		return false
	}
	return textualMediaType(mt) || !binaryMediaType(mt)
}

// responseFilename names a downloaded body: Content-Disposition first, then
// the last path segment when it looks like a file, then a name from the type.
func responseFilename(resp *http.Response, mt string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if name := safeFilename(params["filename"]); name != "" {
				return name
			}
		}
	}
	if u := resp.Request.URL; u != nil {
		if base := safeFilename(path.Base(u.Path)); base != "" && path.Ext(base) != "" {
			return base
		}
	}
	name := "response"
	if ext := responseExtension(mt); ext != "" {
		name += ext
	}
	return name
}

// safeFilename keeps a server-provided name to a single harmless file name.
func safeFilename(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if name == "." || name == "/" || name == "" {
		return ""
	}
	return name
}

func responseExtension(mt string) string {
	if ext, ok := commonExtensions[mt]; ok {
		return ext
	}
	if exts, _ := mime.ExtensionsByType(mt); len(exts) > 0 {
		return exts[0]
	}
	return ""
}

var commonExtensions = map[string]string{
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/gif":                ".gif",
	"image/webp":               ".webp",
	"image/bmp":                ".bmp",
	"image/svg+xml":            ".svg",
	"application/pdf":          ".pdf",
	"application/zip":          ".zip",
	"application/gzip":         ".gz",
	"application/json":         ".json",
	"text/plain":               ".txt",
	"text/html":                ".html",
	"application/octet-stream": ".bin",
}

// DisplayBody returns the body as shown, and whether it is JSON that was
// indented for reading.
func DisplayBody(data []byte, contentType string) (string, bool) {
	s := string(data)
	if len(data) > 0 && !utf8.Valid(data) {
		return "", false
	}
	trimmed := strings.TrimSpace(s)
	if (strings.Contains(strings.ToLower(contentType), "json") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed)) {
		var buf bytes.Buffer
		if json.Indent(&buf, []byte(trimmed), "", "  ") == nil {
			return buf.String(), true
		}
	}
	return s, false
}

func HeadersText(h []HeaderLine) string {
	var b strings.Builder
	for _, line := range h {
		b.WriteString(line.Key)
		b.WriteString(": ")
		b.WriteString(line.Value)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func StatusKind(code int) string {
	switch {
	case code == 0:
		return "idle"
	case code < 300:
		return "ok"
	case code < 400:
		return "redirect"
	case code < 500:
		return "warn"
	default:
		return "error"
	}
}

func ShortSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func ShortDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	return fmt.Sprintf("%.2f s", float64(ms)/1000)
}
