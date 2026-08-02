package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type compatibilityContract struct {
	Schema  string `json:"schema"`
	Scope   string `json:"scope"`
	Summary struct {
		AMQP      bool `json:"amqp_0_9_1_wire_compatible"`
		Unchanged bool `json:"rabbitmq_clients_connect_unchanged"`
		SDK       bool `json:"native_sdk_available"`
		NATS      bool `json:"direct_nats_client_supported"`
	} `json:"summary"`
	ClientPaths []struct {
		ID, Status, Action string
		Examples           []string
	} `json:"client_paths"`
	Features []struct {
		ID, RabbitMQ, Server, Status string
		Evidence                     []string
	} `json:"features"`
}

func TestClientCompatibilityContractIsCompleteAndEvidenceBacked(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "api", "client-compatibility.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var contract compatibilityContract
	if err := decoder.Decode(&contract); err != nil {
		t.Fatal(err)
	}
	if decoder.Decode(&struct{}{}) == nil {
		t.Fatal("trailing JSON accepted")
	}
	if contract.Schema != "rabbit-jetstream.io/client-compatibility/v1alpha1" || contract.Summary.AMQP || contract.Summary.Unchanged || contract.Summary.SDK || !contract.Summary.NATS {
		t.Fatalf("unsafe summary: %+v", contract.Summary)
	}
	statuses := map[string]bool{"supported": true, "partial": true, "planned": true, "not_supported": true}
	clients := map[string]bool{}
	for _, client := range contract.ClientPaths {
		if client.ID == "" || clients[client.ID] || !statuses[client.Status] || len(client.Examples) == 0 || client.Action == "" {
			t.Fatalf("invalid client path: %+v", client)
		}
		clients[client.ID] = true
	}
	if len(clients) < 6 {
		t.Fatalf("client paths=%d", len(clients))
	}
	document, err := os.ReadFile(filepath.Join(root, "docs", "client-migration.md"))
	if err != nil {
		t.Fatal(err)
	}
	features := map[string]bool{}
	for _, feature := range contract.Features {
		if feature.ID == "" || features[feature.ID] || !statuses[feature.Status] || feature.RabbitMQ == "" || feature.Server == "" || len(feature.Evidence) == 0 {
			t.Fatalf("invalid feature: %+v", feature)
		}
		features[feature.ID] = true
		if !strings.Contains(string(document), "`"+feature.ID+"`") {
			t.Errorf("guide does not reference feature %q", feature.ID)
		}
		for _, evidence := range feature.Evidence {
			clean := filepath.Clean(evidence)
			if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
				t.Fatalf("unsafe evidence path %q", evidence)
			}
			info, err := os.Stat(filepath.Join(root, clean))
			if err != nil || info.IsDir() {
				t.Errorf("missing evidence %q", evidence)
			}
		}
	}
	for _, required := range []string{"amqp-wire", "publisher-confirm", "manual-ack", "priority-queue", "queue-ttl", "dead-letter", "transactions"} {
		if !features[required] {
			t.Errorf("missing required feature %q", required)
		}
	}
}

func TestFirstReleaseDoesNotRequireAMQPCompatibility(t *testing.T) {
	root := filepath.Join("..", "..")
	requirements := map[string][]string{
		"README.md":            {"首个正式版本", "rabbit-jetstream-go", "不是首版发布门槛"},
		"docs/architecture.md": {"首版验收目标", "并不要求 SDK 基于 AMQP", "不阻塞首个正式版本发布"},
		"docs/roadmap.md":      {"## 版本范围", "不要求基于 AMQP", "## M5：AMQP 0-9-1 协议网关（未来研究）", "不属于首个正式版本"},
	}
	for relative, expected := range requirements {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range expected {
			if !strings.Contains(string(raw), phrase) {
				t.Errorf("%s lost first-release boundary %q", relative, phrase)
			}
		}
	}
}
