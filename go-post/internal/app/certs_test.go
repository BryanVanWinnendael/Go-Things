package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"go-post/internal/core"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

func makeTestPFX(t *testing.T, password string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "Go Post Test Client", Organization: []string{"Go Post"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	data, err := pkcs12.Modern2023.Encode(key, cert, nil, password)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCertificateForURL(t *testing.T) {
	certs := []core.Certificate{
		{ID: "fallback"},
		{ID: "exact", Host: "API.EXAMPLE.COM"},
		{ID: "wildcard", Host: "*.example.com"},
		{ID: "deep", Host: "*.api.example.com"},
		{ID: "with-scheme", Host: "https://secure.example.net:8443/path"},
	}
	cases := []struct {
		url  string
		want string
	}{
		{"https://api.example.com/users", "exact"},
		{"https://v2.api.example.com/users", "deep"},
		{"https://other.example.com/users", "wildcard"},
		{"secure.example.net/users", "with-scheme"},
		{"https://unrelated.test/users", "fallback"},
		{"", ""},
	}
	for _, tc := range cases {
		got := core.CertificateForURL(certs, tc.url)
		if tc.want == "" {
			if got != nil {
				t.Fatalf("%q chose %q, want no certificate", tc.url, got.ID)
			}
			continue
		}
		if got == nil || got.ID != tc.want {
			t.Fatalf("%q chose %+v, want %q", tc.url, got, tc.want)
		}
	}

	if got := core.CertificateForURL([]core.Certificate{{ID: "wildcard", Host: "*.example.com"}}, "https://unrelated.test"); got != nil {
		t.Fatalf("wildcard matched an unrelated host: %+v", got)
	}
	if got := core.CertificateForURL([]core.Certificate{{ID: "one"}, {ID: "two"}}, "https://unrelated.test"); got != nil {
		t.Fatalf("ambiguous fallback chose %q", got.ID)
	}
}

func TestPFXImportAndPassword(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "certificates"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := core.NewStore(dir, filepath.Join(dir, "gopost.json"))
	path := filepath.Join(dir, "client.pfx")
	if err := os.WriteFile(path, makeTestPFX(t, "correct horse"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := st.TestPFX(path, "correct horse")
	if err != nil {
		t.Fatalf("testing the right password: %v", err)
	}
	if info.Subject == "" || info.NotAfter == "" {
		t.Fatalf("certificate details missing: %+v", info)
	}
	if _, err := st.TestPFX(path, "wrong"); err == nil {
		t.Fatal("wrong password was accepted")
	}

	cert, err := st.ImportPFX(path, "correct horse", "api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if cert.ID == "" || cert.FileName == "" || !cert.HasPassword {
		t.Fatalf("bad imported certificate: %+v", cert)
	}
	if cert.Host != "api.example.com" {
		t.Fatalf("host %q", cert.Host)
	}
	if _, err := os.Stat(filepath.Join(st.CertDir(), cert.FileName)); err != nil {
		t.Fatalf("PFX was not copied: %v", err)
	}

	if err := st.TestStoredCert(cert, "correct horse"); err != nil {
		t.Fatalf("stored certificate password: %v", err)
	}
	if err := st.TestStoredCert(cert, "nope"); err == nil {
		t.Fatal("stored certificate accepted a wrong password")
	}
	tlsCert, _, err := st.ClientCertificate(&cert)
	if err != nil {
		t.Fatalf("loading client certificate: %v", err)
	}
	if len(tlsCert.Certificate) == 0 {
		t.Fatal("client certificate has no certificate chain")
	}
}
