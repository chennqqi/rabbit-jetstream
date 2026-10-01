package contract

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestConnectionOpenAPIExactCountersAndProjection(t *testing.T) {
	var document yaml.Node
	if err := yaml.Unmarshal(OpenAPI, &document); err != nil {
		t.Fatal(err)
	}
	lookup := func(node *yaml.Node, key string) *yaml.Node {
		t.Helper()
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
		}
		t.Fatalf("missing key %s", key)
		return nil
	}
	get := lookup(lookup(lookup(document.Content[0], "paths"), "/api/v1/nodes/{node}/connections"), "get")
	responses := lookup(get, "responses")
	for _, status := range []string{"200", "400", "401", "403", "404", "409", "503"} {
		lookup(responses, status)
	}
	search := lookup(lookup(lookup(document.Content[0], "paths"), "/api/v1/nodes/{node}/connections/search"), "post")
	searchResponses := lookup(search, "responses")
	for _, status := range []string{"200", "400", "401", "403", "404", "409", "415", "422", "503"} {
		lookup(searchResponses, status)
	}
	searchBody := lookup(lookup(lookup(search, "requestBody"), "content"), "application/json")
	searchProperties := lookup(lookup(searchBody, "schema"), "properties")
	for _, field := range []string{"kind", "value", "offset", "limit"} {
		lookup(searchProperties, field)
	}
	schema := lookup(lookup(lookup(lookup(responses, "200"), "content"), "application/json"), "schema")
	properties := lookup(schema, "properties")
	for _, field := range []string{"node_id", "observed_at", "read_at", "offset", "limit", "total", "items"} {
		lookup(properties, field)
	}
	items := lookup(lookup(properties, "items"), "items")
	if lookup(items, "additionalProperties").Value != "false" {
		t.Fatal("unbounded connection projection")
	}
	fields := lookup(items, "properties")
	if len(fields.Content) != 14 {
		t.Fatal("unexpected connection fields")
	}
	for field, maximum := range map[string]string{"cid": "18446744073709551615", "pending_bytes": "9223372036854775807", "in_msgs": "9223372036854775807", "out_msgs": "9223372036854775807", "in_bytes": "9223372036854775807", "out_bytes": "9223372036854775807", "subscriptions": "4294967295"} {
		if lookup(lookup(fields, field), "maximum").Value != maximum {
			t.Fatalf("lost exact bound for %s", field)
		}
	}
	detail := lookup(lookup(lookup(document.Content[0], "paths"), "/api/v1/nodes/{node}/connections/{cid}"), "get")
	if len(lookup(detail, "parameters").Content) != 2 {
		t.Fatal("unexpected detail parameters")
	}
	detailResponses := lookup(detail, "responses")
	for _, status := range []string{"200", "400", "401", "403", "404", "409", "503"} {
		lookup(detailResponses, status)
	}
	detailSchema := lookup(lookup(lookup(lookup(detailResponses, "200"), "content"), "application/json"), "schema")
	detailProperties := lookup(detailSchema, "properties")
	if len(detailProperties.Content) != 8 {
		t.Fatal("detail has unexpected fields or total")
	}
	for _, field := range []string{"node_id", "observed_at", "read_at", "item"} {
		lookup(detailProperties, field)
	}
	detailItem := lookup(detailProperties, "item")
	if lookup(detailItem, "additionalProperties").Value != "false" {
		t.Fatal("unbounded detail projection")
	}
	detailFields := lookup(detailItem, "properties")
	if len(detailFields.Content) != len(fields.Content) {
		t.Fatal("detail counter projection differs")
	}
	for i := 0; i < len(fields.Content); i += 2 {
		name := fields.Content[i].Value
		if lookup(lookup(detailFields, name), "maximum").Value != lookup(fields.Content[i+1], "maximum").Value {
			t.Fatalf("detail bound differs: %s", name)
		}
	}
}
