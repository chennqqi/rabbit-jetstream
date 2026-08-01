package natsclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromEnv(t *testing.T) {
	t.Setenv("RJS_NATS_USER", "operator")
	t.Setenv("RJS_NATS_PASSWORD", "secret")
	t.Setenv("RJS_NATS_CREDS", "/run/nats.creds")
	t.Setenv("RJS_NATS_TLS_CA", "/tls/ca.crt")
	t.Setenv("RJS_NATS_TLS_CERT", "/tls/client.crt")
	t.Setenv("RJS_NATS_TLS_KEY", "/tls/client.key")
	t.Setenv("RJS_NATS_TLS_SERVER_NAME", "nats.messaging.svc")
	t.Setenv("RJS_NATS_TLS_INSECURE_SKIP_VERIFY", "true")
	got := FromEnv()
	if got.User != "operator" || got.Password != "secret" || got.Credentials != "/run/nats.creds" || got.TLSCA != "/tls/ca.crt" || got.TLSCert != "/tls/client.crt" || got.TLSKey != "/tls/client.key" || got.TLSServerName != "nats.messaging.svc" || !got.TLSInsecure {
		t.Fatalf("config=%#v", got)
	}
}

func TestTLSConfigDisabled(t *testing.T) {
	got, err := TLSConfig(Config{})
	if err != nil || got != nil {
		t.Fatalf("config=%v err=%v", got, err)
	}
}

func TestTLSConfigRequiresCertificatePair(t *testing.T) {
	for _, cfg := range []Config{{TLSCert: "client.crt"}, {TLSKey: "client.key"}} {
		if _, err := TLSConfig(cfg); err == nil {
			t.Fatalf("accepted incomplete pair: %#v", cfg)
		}
	}
}

func TestTLSConfigLoadsCAAndCertificate(t *testing.T) {
	directory := t.TempDir()
	certPath, keyPath := writeTestCertificate(t, directory)
	got, err := TLSConfig(Config{TLSCA: certPath, TLSCert: certPath, TLSKey: keyPath, TLSServerName: "nats.example", TLSInsecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.MinVersion != tls.VersionTLS12 || got.ServerName != "nats.example" || !got.InsecureSkipVerify || got.RootCAs == nil || len(got.Certificates) != 1 {
		t.Fatalf("TLS config=%#v", got)
	}
}

func TestTLSConfigRejectsBadFiles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.crt")
	if _, err := TLSConfig(Config{TLSCA: missing}); err == nil {
		t.Fatal("accepted missing CA")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := TLSConfig(Config{TLSCA: bad}); err == nil {
		t.Fatal("accepted invalid CA")
	}
	if _, err := TLSConfig(Config{TLSCert: bad, TLSKey: bad}); err == nil {
		t.Fatal("accepted invalid certificate pair")
	}
}

func TestOptionsAcceptsAuthModes(t *testing.T) {
	for _, cfg := range []Config{{}, {User: "user", Password: "password"}, {Credentials: "account.creds"}, {TLSServerName: "nats.example"}} {
		options, err := Options(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(options) == 0 && cfg != (Config{}) {
			t.Fatalf("no option for %#v", cfg)
		}
	}
}

func writeTestCertificate(t *testing.T, directory string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(directory, "tls.crt"), filepath.Join(directory, "tls.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encodedKey}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}
