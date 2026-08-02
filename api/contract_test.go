package contract

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIIsEmbedded(t *testing.T) {
	if len(OpenAPI) < 100 || !strings.HasPrefix(string(OpenAPI), "openapi:") {
		t.Fatal("OpenAPI contract was not embedded")
	}
}

func TestNativeSDKContractIsStrictAndMatchesTopology(t *testing.T) {
	type header struct {
		Name      string `json:"name"`
		Required  bool   `json:"required"`
		Semantics string `json:"semantics"`
	}
	var document struct {
		Schema        string `json:"schema"`
		Availability  string `json:"availability"`
		ResourceNames struct {
			QueuePattern     string `json:"queue_name_pattern"`
			Stream           string `json:"stream_template"`
			Consumer         string `json:"consumer_template"`
			Ingress          string `json:"ingress_subject_template"`
			PrioritySubject  string `json:"priority_subject_template"`
			PriorityConsumer string `json:"priority_consumer_template"`
		} `json:"resource_names"`
		MessageHeaders []header `json:"message_headers"`
		Priority       struct {
			Minimum                    int    `json:"minimum"`
			Maximum                    int    `json:"maximum"`
			HigherValueFirst           bool   `json:"higher_value_first"`
			InvalidPublish             string `json:"invalid_publish"`
			AlreadyDeliveredPreemption bool   `json:"already_delivered_preemption"`
			ConsumerMode               string `json:"consumer_mode"`
			OneDurablePerLevel         bool   `json:"one_durable_consumer_per_level"`
			Scheduler                  struct {
				Algorithm         string `json:"algorithm"`
				HighPriorityBurst int    `json:"default_high_priority_burst"`
				LowPriorityProbe  int    `json:"default_low_priority_probe"`
				SelectSufficient  bool   `json:"select_is_sufficient"`
				Guarantee         string `json:"guarantee"`
			} `json:"scheduler"`
		} `json:"priority"`
		Delivery struct {
			PublisherConfirm  string `json:"publisher_confirm"`
			DeliveryGuarantee string `json:"delivery_guarantee"`
			AckPolicy         string `json:"ack_policy"`
			Backpressure      string `json:"backpressure"`
		} `json:"delivery"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(NativeSDK)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if decoder.Decode(&struct{}{}) == nil {
		t.Fatal("native SDK contract accepted trailing JSON")
	}
	if document.Schema != "rabbit-jetstream.io/native-sdk-contract/v1alpha1" || document.Availability != "native-sdk-implemented-unreleased" {
		t.Fatalf("unsafe native SDK contract identity: %q %q", document.Schema, document.Availability)
	}
	prioritySubject, _ := topology.QueuePrioritySubject("orders", 7)
	priorityConsumer, _ := topology.PriorityConsumerName("orders", 7)
	replacements := strings.NewReplacer("{queue}", "orders", "{priority}", "7")
	checks := map[string]string{
		document.ResourceNames.Stream:           topology.StreamName("orders"),
		document.ResourceNames.Consumer:         topology.ConsumerName("orders"),
		document.ResourceNames.Ingress:          topology.QueueIngressSubject("orders"),
		document.ResourceNames.PrioritySubject:  prioritySubject,
		document.ResourceNames.PriorityConsumer: priorityConsumer,
	}
	for template, want := range checks {
		if got := replacements.Replace(template); got != want {
			t.Errorf("resource template %q produced %q, want %q", template, got, want)
		}
	}
	if document.ResourceNames.QueuePattern != "^[A-Za-z0-9_-]+$" || document.Priority.Minimum != topology.MinimumPriority || document.Priority.Maximum != topology.MaximumPriority || !document.Priority.HigherValueFirst || document.Priority.InvalidPublish != "reject" || document.Priority.AlreadyDeliveredPreemption || document.Priority.ConsumerMode != "pull" || !document.Priority.OneDurablePerLevel {
		t.Fatalf("unsafe priority contract: %+v", document.Priority)
	}
	if document.Priority.Scheduler.Algorithm != "bounded-strict-priority" || document.Priority.Scheduler.HighPriorityBurst < 1 || document.Priority.Scheduler.LowPriorityProbe < 1 || document.Priority.Scheduler.SelectSufficient || document.Priority.Scheduler.Guarantee == "" {
		t.Fatalf("incomplete starvation protection: %+v", document.Priority.Scheduler)
	}
	requiredHeaders := map[string]bool{"Nats-Msg-Id": false, "Rjs-Contract-Version": false, "Rjs-Queue": false, "Rjs-Policy-Revision": false}
	seen := map[string]bool{}
	for _, value := range document.MessageHeaders {
		if value.Name == "" || value.Semantics == "" || seen[value.Name] {
			t.Fatalf("invalid message header: %+v", value)
		}
		seen[value.Name] = true
		if _, required := requiredHeaders[value.Name]; required {
			requiredHeaders[value.Name] = value.Required
		}
	}
	for name, required := range requiredHeaders {
		if !required {
			t.Errorf("required message header %s is missing or optional", name)
		}
	}
	if document.Delivery.PublisherConfirm != "JetStream PubAck" || document.Delivery.DeliveryGuarantee != "at-least-once" || document.Delivery.AckPolicy != "explicit" || document.Delivery.Backpressure == "" {
		t.Fatalf("incomplete delivery contract: %+v", document.Delivery)
	}
}

func TestOpenAPIMatchesRegisteredV1Routes(t *testing.T) {
	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(OpenAPI, &document); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../management/internal/api/handler.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`mux\.HandleFunc\("(GET|PUT|POST|PATCH|DELETE) (/api/v1/[^" ]+)"`)
	registered := make(map[string]bool)
	for _, match := range pattern.FindAllStringSubmatch(string(source), -1) {
		registered[strings.ToLower(match[1])+" "+match[2]] = true
	}
	documented := make(map[string]bool)
	for path, item := range document.Paths {
		if !strings.HasPrefix(path, "/api/v1/") {
			continue
		}
		for method, operation := range item {
			if operation == nil || !isHTTPMethod(method) {
				continue
			}
			documented[method+" "+path] = true
		}
	}
	for route := range registered {
		if !documented[route] {
			t.Errorf("registered route is undocumented: %s", route)
		}
	}
	for route := range documented {
		if !registered[route] {
			t.Errorf("documented route is not registered: %s", route)
		}
	}
}

func TestOpenAPIOperationsAndLocalReferencesAreComplete(t *testing.T) {
	var document map[string]any
	if err := yaml.Unmarshal(OpenAPI, &document); err != nil {
		t.Fatal(err)
	}
	operationIDs := make(map[string]bool)
	for path, pathValue := range asObject(document["paths"]) {
		for method, operationValue := range asObject(pathValue) {
			if !isHTTPMethod(method) {
				continue
			}
			operation := asObject(operationValue)
			operationID, _ := operation["operationId"].(string)
			if operationID == "" || operationIDs[operationID] {
				t.Errorf("%s %s has missing or duplicate operationId %q", method, path, operationID)
			}
			operationIDs[operationID] = true
			if asObject(operation["responses"])["200"] == nil {
				t.Errorf("%s %s has no documented 200 response", method, path)
			}
		}
	}
	walkReferences(t, document, document)
}

func walkReferences(t *testing.T, root map[string]any, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if reference, ok := typed["$ref"].(string); ok && strings.HasPrefix(reference, "#/") {
			current := any(root)
			for _, part := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
				current = asObject(current)[part]
				if current == nil {
					t.Errorf("unresolved reference %s", reference)
					break
				}
			}
		}
		for _, child := range typed {
			walkReferences(t, root, child)
		}
	case []any:
		for _, child := range typed {
			walkReferences(t, root, child)
		}
	}
}

func asObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func isHTTPMethod(value string) bool {
	switch value {
	case "get", "put", "post", "patch", "delete":
		return true
	default:
		return false
	}
}
