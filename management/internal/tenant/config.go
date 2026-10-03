package tenant

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
)

const configVersion = "rjs.tenants.v1"

// Definition describes one isolated NATS account managed as a tenant. Empty
// optional fields inherit the process-wide defaults.
type Definition struct {
	ID               string `json:"id"`
	NATSURL          string `json:"nats_url"`
	NATSUser         string `json:"nats_user,omitempty"`
	NATSPassword     string `json:"nats_password,omitempty"`
	NATSCreds        string `json:"nats_creds,omitempty"`
	NATSTLSCA        string `json:"nats_tls_ca,omitempty"`
	NATSTLSCert      string `json:"nats_tls_cert,omitempty"`
	NATSTLSKey       string `json:"nats_tls_key,omitempty"`
	NATSServerName   string `json:"nats_tls_server_name,omitempty"`
	MonitorURLs      string `json:"monitor_urls,omitempty"`
	MetadataBucket   string `json:"metadata_bucket,omitempty"`
	MetadataReplicas int    `json:"metadata_replicas,omitempty"`
}

type document struct {
	Version string       `json:"version"`
	Tenants []Definition `json:"tenants"`
}

func Load(path string) ([]Definition, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tenants file: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 1<<20 {
		return nil, errors.New("tenants file exceeds 1 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var value document
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("decode tenants file")
	}
	if value.Version != configVersion {
		return nil, errors.New("unsupported tenants file version")
	}
	if len(value.Tenants) == 0 {
		return nil, errors.New("tenants file must define at least one tenant")
	}
	seen := make(map[string]struct{}, len(value.Tenants))
	for i := range value.Tenants {
		t := &value.Tenants[i]
		if !ValidID(t.ID) || strings.TrimSpace(t.NATSURL) == "" {
			return nil, fmt.Errorf("invalid tenant at index %d", i)
		}
		if _, ok := seen[t.ID]; ok {
			return nil, fmt.Errorf("duplicate tenant %q", t.ID)
		}
		seen[t.ID] = struct{}{}
		if t.MetadataReplicas != 0 && t.MetadataReplicas != 1 && t.MetadataReplicas != 3 && t.MetadataReplicas != 5 {
			return nil, fmt.Errorf("tenant %q has invalid metadata_replicas", t.ID)
		}
		if (t.NATSUser == "") != (t.NATSPassword == "") {
			return nil, fmt.Errorf("tenant %q must provide both nats_user and nats_password", t.ID)
		}
		if t.NATSCreds != "" && t.NATSUser != "" {
			return nil, fmt.Errorf("tenant %q cannot combine credentials file and user/password", t.ID)
		}
	}
	return value.Tenants, nil
}

func ValidID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if !(ch == '-' || ch == '_' || ch == '.' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
			return false
		}
	}
	return true
}

func (d Definition) Apply(base config.Config) config.Config {
	base.NATSURL, base.NATSUser, base.NATSPassword, base.NATSCreds = d.NATSURL, d.NATSUser, d.NATSPassword, d.NATSCreds
	if d.NATSTLSCA != "" {
		base.NATSTLSCA = d.NATSTLSCA
	}
	if d.NATSTLSCert != "" {
		base.NATSTLSCert = d.NATSTLSCert
	}
	if d.NATSTLSKey != "" {
		base.NATSTLSKey = d.NATSTLSKey
	}
	if d.NATSServerName != "" {
		base.NATSTLSServerName = d.NATSServerName
	}
	if d.MonitorURLs != "" {
		base.NATSMonitorURLs = d.MonitorURLs
	}
	if d.MetadataBucket != "" {
		base.MetadataBucket = d.MetadataBucket
	}
	if d.MetadataReplicas != 0 {
		base.MetadataReplicas = d.MetadataReplicas
	}
	base.Name += ".tenant." + d.ID
	base.InstanceID += "." + d.ID
	return base
}
