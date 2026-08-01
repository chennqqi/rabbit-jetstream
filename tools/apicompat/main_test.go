package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareDetectsBreakingContractChanges(t *testing.T) {
	old := parse(t, `openapi: 3.1.0
paths:
  /api/v1/items:
    get: {responses: {'200': {description: ok}, '404': {description: missing}}}
components:
  schemas:
    Item: {type: object, required: [id], properties: {id: {type: string}, count: {type: integer}}}
`)
	current := parse(t, `openapi: 3.1.0
paths:
  /api/v1/items:
    get: {responses: {'200': {description: ok}}}
components:
  schemas:
    Item: {type: object, properties: {id: {type: integer}}}
`)
	issues := strings.Join(compare(old, current), "\n")
	for _, wanted := range []string{"removed response 404", "removed property Item.count", "changed type of Item.id", "removed required field Item.id"} {
		if !strings.Contains(issues, wanted) {
			t.Fatalf("issues=%s missing %s", issues, wanted)
		}
	}
}

func TestCompareAcceptsAdditiveChangesAndReadsFiles(t *testing.T) {
	document := "openapi: 3.1.0\npaths: {}\ncomponents: {schemas: {}}\n"
	directory := t.TempDir()
	baseline, current := filepath.Join(directory, "old.yaml"), filepath.Join(directory, "new.yaml")
	if err := os.WriteFile(baseline, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := compareFiles(baseline, current)
	if err != nil || len(issues) != 0 {
		t.Fatalf("issues=%v err=%v", issues, err)
	}
	if _, err := readDocument(filepath.Join(directory, "missing")); err == nil {
		t.Fatal("missing document succeeded")
	}
	invalid := filepath.Join(directory, "invalid.yaml")
	if err := os.WriteFile(invalid, []byte("openapi: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDocument(invalid); err == nil {
		t.Fatal("invalid YAML succeeded")
	}
	notOpenAPI := filepath.Join(directory, "other.yaml")
	if err := os.WriteFile(notOpenAPI, []byte("value: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDocument(notOpenAPI); err == nil {
		t.Fatal("non-OpenAPI document succeeded")
	}
}

func TestCompareDetectsRemovedPathsOperationsAndSchemas(t *testing.T) {
	old := parse(t, `openapi: 3.1.0
paths:
  /removed: {get: {responses: {'200': {description: ok}}}}
  /changed:
    get: {responses: {'200': {description: ok}}}
    put: {responses: {'200': {description: ok}}}
components: {schemas: {Removed: {type: object}}}
`)
	current := parse(t, `openapi: 3.1.0
paths:
  /changed: {get: {responses: {'200': {description: ok}}}}
components: {schemas: {}}
`)
	issues := strings.Join(compare(old, current), "\n")
	for _, wanted := range []string{"removed path /removed", "removed operation put /changed", "removed schema Removed"} {
		if !strings.Contains(issues, wanted) {
			t.Fatalf("issues=%s missing %s", issues, wanted)
		}
	}
}

func TestCompatibilityHelpers(t *testing.T) {
	if object("not-an-object") != nil || httpMethod("options") || !httpMethod("patch") || contains([]string{"a"}, "b") || !contains([]string{"a"}, "a") {
		t.Fatal("helper behavior mismatch")
	}
	values := stringsList([]any{"one", 2, "two"})
	if len(values) != 2 || values[1] != "two" || len(stringsList("invalid")) != 0 {
		t.Fatalf("values=%v", values)
	}
}

func TestCompareFilesReportsCurrentAndBaselineErrors(t *testing.T) {
	directory := t.TempDir()
	valid := filepath.Join(directory, "valid.yaml")
	if err := os.WriteFile(valid, []byte("openapi: 3.1.0\npaths: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := compareFiles(filepath.Join(directory, "missing"), valid); err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("baseline err=%v", err)
	}
	if _, err := compareFiles(valid, filepath.Join(directory, "missing")); err == nil || !strings.Contains(err.Error(), "current") {
		t.Fatalf("current err=%v", err)
	}
}

func parse(t *testing.T, input string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	document, err := readDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	return document
}
