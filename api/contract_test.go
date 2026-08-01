package contract

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIIsEmbedded(t *testing.T) {
	if len(OpenAPI) < 100 || !strings.HasPrefix(string(OpenAPI), "openapi:") {
		t.Fatal("OpenAPI contract was not embedded")
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
