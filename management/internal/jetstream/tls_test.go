package jetstream

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

	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
)

func TestClientTLSConfigDisabled(t *testing.T) {
	got, err := clientTLSConfig(config.Config{})
	if err != nil || got != nil {
		t.Fatalf("config=%v err=%v", got, err)
	}
}

func TestClientTLSConfigRequiresCertificatePair(t *testing.T) {
	for _, cfg := range []config.Config{{NATSTLSCert: "client.crt"}, {NATSTLSKey: "client.key"}} {
		if _, err := clientTLSConfig(cfg); err == nil {
			t.Fatalf("accepted incomplete certificate pair: %#v", cfg)
		}
	}
}

func TestClientTLSConfigLoadsCAAndCertificate(t *testing.T) {
	directory := t.TempDir()
	certPath := filepath.Join(directory, "tls.crt")
	keyPath := filepath.Join(directory, "tls.key")
	certificatePEM, keyPEM := testCertificate(t)
	if err := os.WriteFile(certPath, certificatePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	tlsConfig, err := clientTLSConfig(config.Config{NATSTLSCA: certPath, NATSTLSCert: certPath, NATSTLSKey: keyPath, NATSTLSServerName: "nats.example", NATSTLSInsecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 || tlsConfig.ServerName != "nats.example" || !tlsConfig.InsecureSkipVerify || tlsConfig.RootCAs == nil || len(tlsConfig.Certificates) != 1 {
		t.Fatalf("unexpected TLS config: %#v", tlsConfig)
	}
}

func TestClientTLSConfigRejectsInvalidCA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := clientTLSConfig(config.Config{NATSTLSCA: path}); err == nil {
		t.Fatal("accepted invalid CA")
	}
}

func TestClientTLSConfigRejectsMissingCA(t *testing.T) {
	if _, err := clientTLSConfig(config.Config{NATSTLSCA: filepath.Join(t.TempDir(), "missing.crt")}); err == nil {
		t.Fatal("accepted missing CA")
	}
}

func TestClientTLSConfigRejectsInvalidCertificatePair(t *testing.T) {
	directory := t.TempDir()
	certPath := filepath.Join(directory, "tls.crt")
	keyPath := filepath.Join(directory, "tls.key")
	if err := os.WriteFile(certPath, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := clientTLSConfig(config.Config{NATSTLSCert: certPath, NATSTLSKey: keyPath}); err == nil {
		t.Fatal("accepted invalid certificate pair")
	}
}

func testCertificate(t *testing.T) ([]byte, []byte) {
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
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encodedKey})
}
