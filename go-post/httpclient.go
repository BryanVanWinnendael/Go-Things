package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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
	// JSON says the body is JSON that displayBody indented, so the response
	// pane can color it without parsing it again on the UI thread.
	JSON bool `json:"json"`
}

func (s *Store) Send(req Request, vars map[string]string, settings Settings, cert *Certificate) SendResult {
	start := time.Now()
	rawURL := strings.TrimSpace(interpolate(req.URL, vars))
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
	bodyText := interpolate(req.Body, vars)
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
		key := interpolate(h.Key, vars)
		val := interpolate(h.Value, vars)
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
		r.Header.Set("Authorization", "Bearer "+interpolate(req.Bearer, vars))
	case "basic":
		r.SetBasicAuth(interpolate(req.Username, vars), interpolate(req.Password, vars))
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: settings.SkipVerify}
	if cert != nil && strings.EqualFold(r.URL.Scheme, "https") {
		tlsCert, chain, err := s.clientCertificate(cert)
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
	text, isJSON := displayBody(data, resp.Header.Get("Content-Type"))
	if truncated {
		text += "\n\n… response cut off after 8 MB"
	}
	if !utf8.ValidString(text) {
		text = "[binary response: " + fmt.Sprintf("%d bytes", len(data)) + "]"
		isJSON = false
	}
	out.Body = text
	out.JSON = isJSON
	return out
}

// displayBody returns the body as shown, and whether it is JSON that was
// indented for reading.
func displayBody(data []byte, contentType string) (string, bool) {
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

func headersText(h []HeaderLine) string {
	var b strings.Builder
	for _, line := range h {
		b.WriteString(line.Key)
		b.WriteString(": ")
		b.WriteString(line.Value)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func statusKind(code int) string {
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

func shortSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func shortDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	return fmt.Sprintf("%.2f s", float64(ms)/1000)
}
