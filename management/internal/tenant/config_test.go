package tenant

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/management/internal/config"
)

func TestLoadTenantDefinitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tenants.json")
	body := `{"version":"rjs.tenants.v1","tenants":[{"id":"blue","nats_url":"nats://blue:4222","monitor_urls":"http://blue:8222","metadata_replicas":3}]}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := Load(path)
	if err != nil || len(items) != 1 || items[0].ID != "blue" {
		t.Fatalf("Load() = %#v, %v", items, err)
	}
	got := items[0].Apply(config.Config{Name: "rjs", InstanceID: "one", MetadataBucket: "RJS_META", MetadataReplicas: 1})
	if got.NATSURL != "nats://blue:4222" || got.NATSMonitorURLs != "http://blue:8222" || got.MetadataReplicas != 3 || got.Name != "rjs.tenant.blue" {
		t.Fatalf("Apply() = %#v", got)
	}
}

func TestLoadTenantDefinitionsRejectsInvalidDocuments(t *testing.T) {
	for name, body := range map[string]string{
		"unknown field":     `{"version":"rjs.tenants.v1","tenants":[],"extra":true}`,
		"trailing":          `{"version":"rjs.tenants.v1","tenants":[]} {}`,
		"empty":             `{"version":"rjs.tenants.v1","tenants":[]}`,
		"duplicate":         `{"version":"rjs.tenants.v1","tenants":[{"id":"a","nats_url":"nats://a"},{"id":"a","nats_url":"nats://b"}]}`,
		"partial password":  `{"version":"rjs.tenants.v1","tenants":[{"id":"a","nats_url":"nats://a","nats_user":"u"}]}`,
		"mixed credentials": `{"version":"rjs.tenants.v1","tenants":[{"id":"a","nats_url":"nats://a","nats_user":"u","nats_password":"p","nats_creds":"x"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tenants.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load() accepted invalid document")
			}
		})
	}
}

func TestValidID(t *testing.T) {
	if !ValidID("tenant-1.prod") || ValidID("bad/id") || ValidID("") {
		t.Fatal("unexpected tenant ID validation")
	}
}
