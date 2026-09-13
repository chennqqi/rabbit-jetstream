package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBundleEmbedsQueueSchemaAndRewritesLocalRefs(t *testing.T) {
	openapi := []byte("openapi: 3.1.0\ncomponents:\n  schemas:\n    Queue:\n      $ref: /api/v1/console/queue-schema\n")
	queue := []byte(`{"type":"object","$defs":{"name":{"type":"string"}},"properties":{"name":{"$ref":"#/$defs/name"}}}`)
	result, err := bundle(openapi, queue)
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	if strings.Contains(text, queueSchemaRef) || !strings.Contains(text, "#/components/schemas/Queue/$defs/name") {
		t.Fatalf("unexpected bundle:\n%s", text)
	}
	var parsed any
	if err := yaml.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("generated YAML is invalid: %v", err)
	}
}

func TestRepositoryContractsBundleWithoutRuntimeReferences(t *testing.T) {
	openapi, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	queue, err := os.ReadFile("../../internal/topology/queue-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := bundle(openapi, queue)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "$ref: "+queueSchemaRef) || strings.Contains(string(result), "$ref: '#/$defs/") {
		t.Fatal("bundled contract retained an unresolved runtime or Queue-local reference")
	}
}

func TestBundleRejectsUnexpectedQueueSource(t *testing.T) {
	_, err := bundle([]byte("components:\n  schemas:\n    Queue:\n      type: object\n"), []byte(`{"type":"object"}`))
	if err == nil {
		t.Fatal("unexpected Queue source was accepted")
	}
}
