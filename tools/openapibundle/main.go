// Command openapibundle makes the published OpenAPI contract self-contained
// for offline generators. The runtime Queue schema remains authoritative; this
// command embeds it under components.schemas.Queue and rewrites its local refs.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const queueSchemaRef = "/api/v1/console/queue-schema"

func main() {
	openapi := flag.String("openapi", "api/openapi.yaml", "source OpenAPI document")
	queue := flag.String("queue-schema", "internal/topology/queue-schema.json", "source Queue JSON Schema")
	out := flag.String("out", "", "output path")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "-out is required")
		os.Exit(2)
	}
	openapiData, err := os.ReadFile(*openapi)
	if err != nil {
		fail("read OpenAPI", err)
	}
	queueData, err := os.ReadFile(*queue)
	if err != nil {
		fail("read Queue schema", err)
	}
	bundled, err := bundle(openapiData, queueData)
	if err != nil {
		fail("bundle contracts", err)
	}
	if err := os.WriteFile(*out, bundled, 0o644); err != nil {
		fail("write bundled OpenAPI", err)
	}
}

func fail(operation string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", operation, err)
	os.Exit(1)
}

func bundle(openapiData, queueData []byte) ([]byte, error) {
	var document, queue any
	if err := yaml.Unmarshal(openapiData, &document); err != nil {
		return nil, fmt.Errorf("decode OpenAPI: %w", err)
	}
	if err := json.Unmarshal(queueData, &queue); err != nil {
		return nil, fmt.Errorf("decode Queue schema: %w", err)
	}
	root, ok := document.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI root is not an object")
	}
	components, ok := root["components"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI components are missing")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI component schemas are missing")
	}
	current, ok := schemas["Queue"].(map[string]any)
	if !ok || current["$ref"] != queueSchemaRef {
		return nil, fmt.Errorf("Queue schema is not the expected runtime reference")
	}
	rewriteLocalRefs(queue)
	schemas["Queue"] = queue
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("encode OpenAPI: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close encoder: %w", err)
	}
	return output.Bytes(), nil
}

func rewriteLocalRefs(value any) {
	switch value := value.(type) {
	case map[string]any:
		if ref, ok := value["$ref"].(string); ok && len(ref) >= len("#/$defs/") && ref[:len("#/$defs/")] == "#/$defs/" {
			value["$ref"] = "#/components/schemas/Queue/$defs/" + ref[len("#/$defs/"):]
		}
		for _, child := range value {
			rewriteLocalRefs(child)
		}
	case []any:
		for _, child := range value {
			rewriteLocalRefs(child)
		}
	}
}
