// Command tls-fixture creates short-lived test-only mutual TLS material.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func main() {
	output := flag.String("output", "", "output directory")
	flag.Parse()
	if *output == "" {
		log.Fatal("--output is required")
	}
	if err := os.MkdirAll(*output, 0o700); err != nil {
		log.Fatal(err)
	}
	caKey := key()
	now := time.Now().UTC()
	ca := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "rabbit-jetstream test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER := certificate(ca, ca, &caKey.PublicKey, caKey)
	leafKey := key()
	leaf := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "production-rabbit-jetstream-nats"}, DNSNames: []string{"production-rabbit-jetstream-nats", "production-rabbit-jetstream-nats-0.production-rabbit-jetstream-nats-headless", "production-rabbit-jetstream-nats-1.production-rabbit-jetstream-nats-headless", "production-rabbit-jetstream-nats-2.production-rabbit-jetstream-nats-headless"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	leafDER := certificate(leaf, ca, &leafKey.PublicKey, caKey)
	write(filepath.Join(*output, "ca.crt"), "CERTIFICATE", caDER)
	write(filepath.Join(*output, "tls.crt"), "CERTIFICATE", leafDER)
	encodedKey, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		log.Fatal(err)
	}
	write(filepath.Join(*output, "tls.key"), "EC PRIVATE KEY", encodedKey)
}

func key() *ecdsa.PrivateKey {
	value, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func serial() *big.Int {
	value, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func certificate(template, parent *x509.Certificate, publicKey any, signer *ecdsa.PrivateKey) []byte {
	value, err := x509.CreateCertificate(rand.Reader, template, parent, publicKey, signer)
	if err != nil {
		log.Fatal(err)
	}
	return value
}

func write(path, blockType string, value []byte) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: value}); err != nil {
		log.Fatal(err)
	}
}
