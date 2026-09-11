package contract

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIReadSecurityDefaults(t *testing.T) {
	var document struct {
		Security []map[string][]string `yaml:"security"`
		Paths    map[string]struct {
			Get *struct {
				Security *[]map[string][]string `yaml:"security"`
			} `yaml:"get"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(OpenAPI, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Security) != 1 {
		t.Fatal("expected global authentication requirement")
	}
	if _, ok := document.Security[0]["bearerAuth"]; !ok {
		t.Fatal("expected bearer authentication")
	}
	public := map[string]bool{
		"/healthz": true, "/readyz": true,
		"/api/v1/openapi.yaml": true, "/api/v1/native-sdk-contract.json": true,
		"/api/v1/oidc/config": true,
	}
	for path, methods := range document.Paths {
		operation := methods.Get
		if operation == nil {
			continue
		}
		anonymous := operation.Security != nil && len(*operation.Security) == 0
		if anonymous != public[path] {
			t.Errorf("GET %s anonymous=%v want=%v", path, anonymous, public[path])
		}
	}
}
