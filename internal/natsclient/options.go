// Package natsclient provides one production connection policy for repository tools.
package natsclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/nats-io/nats.go"
)

// Config contains authentication and transport-security inputs for a NATS client.
type Config struct {
	User          string
	Password      string
	Credentials   string
	TLSCA         string
	TLSCert       string
	TLSKey        string
	TLSServerName string
	TLSInsecure   bool
}

// FromEnv reads the RJS_NATS_* connection contract shared by management and tools.
func FromEnv() Config {
	return Config{
		User:          os.Getenv("RJS_NATS_USER"),
		Password:      os.Getenv("RJS_NATS_PASSWORD"),
		Credentials:   os.Getenv("RJS_NATS_CREDS"),
		TLSCA:         os.Getenv("RJS_NATS_TLS_CA"),
		TLSCert:       os.Getenv("RJS_NATS_TLS_CERT"),
		TLSKey:        os.Getenv("RJS_NATS_TLS_KEY"),
		TLSServerName: os.Getenv("RJS_NATS_TLS_SERVER_NAME"),
		TLSInsecure:   boolean(os.Getenv("RJS_NATS_TLS_INSECURE_SKIP_VERIFY")),
	}
}

// Options returns NATS options with credentials taking precedence over user/password.
func Options(cfg Config) ([]nats.Option, error) {
	options := make([]nats.Option, 0, 2)
	if cfg.Credentials != "" {
		options = append(options, nats.UserCredentials(cfg.Credentials))
	} else if cfg.User != "" {
		options = append(options, nats.UserInfo(cfg.User, cfg.Password))
	}
	tlsConfig, err := TLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		options = append(options, nats.Secure(tlsConfig))
	}
	return options, nil
}

// TLSConfig builds a TLS 1.2+ configuration and fails closed on incomplete material.
func TLSConfig(cfg Config) (*tls.Config, error) {
	enabled := cfg.TLSCA != "" || cfg.TLSCert != "" || cfg.TLSKey != "" || cfg.TLSServerName != "" || cfg.TLSInsecure
	if !enabled {
		return nil, nil
	}
	if (cfg.TLSCert == "") != (cfg.TLSKey == "") {
		return nil, errors.New("configure both RJS_NATS_TLS_CERT and RJS_NATS_TLS_KEY")
	}
	result := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.TLSServerName, InsecureSkipVerify: cfg.TLSInsecure} // #nosec G402 -- explicit break-glass setting.
	if cfg.TLSCA != "" {
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		encoded, err := os.ReadFile(cfg.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("read NATS TLS CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(encoded) {
			return nil, errors.New("NATS TLS CA contains no valid certificates")
		}
		result.RootCAs = roots
	}
	if cfg.TLSCert != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("load NATS TLS client certificate: %w", err)
		}
		result.Certificates = []tls.Certificate{certificate}
	}
	return result, nil
}

func boolean(value string) bool {
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
