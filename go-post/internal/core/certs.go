package core

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

func readPFX(path, password string) ([]byte, interface{}, *x509.Certificate, []*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	key, cert, chain, err := pkcs12.DecodeChain(b, password)
	if err != nil {
		return b, nil, nil, nil, err
	}
	if key == nil || cert == nil {
		return b, nil, nil, nil, errors.New("this PFX has no private key and certificate")
	}
	return b, key, cert, chain, nil
}

func certInfo(cert *x509.Certificate, host string) Certificate {
	sum := sha256.Sum256(cert.Raw)
	fp := make([]string, len(sum))
	for i, b := range sum {
		fp[i] = fmt.Sprintf("%02X", b)
	}
	issuer := cert.Issuer.CommonName
	if issuer == "" {
		issuer = cert.Issuer.String()
	}
	return Certificate{
		Name:        cert.Subject.CommonName,
		Subject:     cert.Subject.String(),
		Issuer:      issuer,
		NotBefore:   cert.NotBefore.Format(time.RFC3339),
		NotAfter:    cert.NotAfter.Format(time.RFC3339),
		Serial:      cert.SerialNumber.Text(16),
		Fingerprint: strings.Join(fp, ":"),
		Host:        host,
		CreatedAt:   time.Now().Format(time.RFC3339),
		HasPassword: true,
	}
}

func (s *Store) TestPFX(path, password string) (Certificate, error) {
	_, _, cert, _, err := readPFX(path, password)
	if err != nil {
		return Certificate{}, friendlyPFXError(err)
	}
	info := certInfo(cert, "")
	info.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return info, nil
}

func (s *Store) ImportPFX(path, password, host string) (Certificate, error) {
	_, _, cert, _, err := readPFX(path, password)
	if err != nil {
		return Certificate{}, friendlyPFXError(err)
	}
	id := NewID("cert")
	dst := filepath.Join(s.CertDir(), id+".pfx")
	if err := copyFile(path, dst); err != nil {
		return Certificate{}, err
	}
	info := certInfo(cert, strings.TrimSpace(host))
	info.ID = id
	info.FileName = id + ".pfx"
	if info.Name == "" {
		info.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if err := s.SavePassword(id, password); err != nil {
		return Certificate{}, err
	}
	return info, nil
}

func (s *Store) TestStoredCert(c Certificate, password string) error {
	path := filepath.Join(s.CertDir(), c.FileName)
	_, _, _, _, err := readPFX(path, password)
	if err != nil {
		return friendlyPFXError(err)
	}
	return nil
}

func (s *Store) ClientCertificate(c *Certificate) (*tls.Certificate, []*x509.Certificate, error) {
	if c == nil {
		return nil, nil, errors.New("certificate not found")
	}
	password, err := s.Password(c.ID)
	if err != nil {
		return nil, nil, errors.New("certificate password is not available")
	}
	path := filepath.Join(s.CertDir(), c.FileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	blocks, err := pkcs12.ToPEM(b, password)
	if err != nil {
		return nil, nil, friendlyPFXError(err)
	}
	var all []byte
	for _, block := range blocks {
		all = append(all, pem.EncodeToMemory(block)...)
	}
	tc, err := tls.X509KeyPair(all, all)
	if err != nil {
		return nil, nil, err
	}
	var chain []*x509.Certificate
	_, _, _, cas, err := readPFX(path, password)
	if err == nil {
		chain = cas
	}
	return &tc, chain, nil
}

func RemoveFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func friendlyPFXError(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "decryption"), strings.Contains(msg, "password"), strings.Contains(msg, "mac"):
		return errors.New("could not open the PFX. Check the password and try again")
	case strings.Contains(msg, "pkcs12"):
		return errors.New("this file is not a valid PFX/P12 certificate")
	default:
		return err
	}
}

// CertificateForURL picks the client certificate bound to the request host.
// An exact host wins over a wildcard, and a wildcard wins over a certificate
// imported without a host (the "any host" fallback).
func CertificateForURL(certs []Certificate, rawURL string) *Certificate {
	host := requestURLHost(rawURL)
	if host == "" {
		return nil
	}
	var exact, wildcard, fallback *Certificate
	fallbackCount := 0
	wildcardLen := -1
	for i := range certs {
		cert := &certs[i]
		pattern := normalizeCertificateHost(cert.Host)
		switch {
		case pattern == "":
			fallbackCount++
			if fallback == nil {
				fallback = cert
			}
		case pattern == host:
			if exact == nil {
				exact = cert
			}
		case strings.HasPrefix(pattern, "*."):
			suffix := pattern[1:]
			if len(host) > len(suffix) && strings.HasSuffix(host, suffix) && len(pattern) > wildcardLen {
				wildcard = cert
				wildcardLen = len(pattern)
			}
		}
	}
	if exact != nil {
		return exact
	}
	if wildcard != nil {
		return wildcard
	}
	if fallbackCount == 1 {
		return fallback
	}
	return nil
}

func requestURLHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	} else if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
}

func normalizeCertificateHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		if u, err := url.Parse(host); err == nil {
			host = u.Host
		}
	} else {
		host = strings.TrimPrefix(host, "//")
		if i := strings.IndexByte(host, '/'); i >= 0 {
			host = host[:i]
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return strings.TrimSuffix(host, ".")
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o600)
}
