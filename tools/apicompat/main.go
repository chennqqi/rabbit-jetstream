package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

func main() {
	baseline := flag.String("baseline", "", "previous OpenAPI document")
	current := flag.String("current", "api/openapi.yaml", "current OpenAPI document")
	flag.Parse()
	if *baseline == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: apicompat --baseline FILE [--current FILE]")
		os.Exit(2)
	}
	issues, err := compareFiles(*baseline, *current)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, "breaking:", issue)
		}
		os.Exit(1)
	}
	fmt.Println("OpenAPI contract is backward compatible")
}

func compareFiles(baselinePath, currentPath string) ([]string, error) {
	baseline, err := readDocument(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	current, err := readDocument(currentPath)
	if err != nil {
		return nil, fmt.Errorf("current: %w", err)
	}
	return compare(baseline, current), nil
}

func readDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if document["openapi"] == nil || object(document["paths"]) == nil {
		return nil, errors.New("not an OpenAPI document")
	}
	return document, nil
}

func compare(old, current map[string]any) []string {
	issues := make([]string, 0)
	oldPaths, newPaths := object(old["paths"]), object(current["paths"])
	for path, oldPathValue := range oldPaths {
		newPath := object(newPaths[path])
		if newPath == nil {
			issues = append(issues, "removed path "+path)
			continue
		}
		for method, oldOperationValue := range object(oldPathValue) {
			if !httpMethod(method) {
				continue
			}
			newOperation := object(newPath[method])
			if newOperation == nil {
				issues = append(issues, fmt.Sprintf("removed operation %s %s", method, path))
				continue
			}
			oldResponses := object(object(oldOperationValue)["responses"])
			newResponses := object(newOperation["responses"])
			for status := range oldResponses {
				if newResponses[status] == nil {
					issues = append(issues, fmt.Sprintf("removed response %s from %s %s", status, method, path))
				}
			}
		}
	}
	compareSchemas(object(object(old["components"])["schemas"]), object(object(current["components"])["schemas"]), &issues)
	sort.Strings(issues)
	return issues
}

func compareSchemas(oldSchemas, newSchemas map[string]any, issues *[]string) {
	for name, oldValue := range oldSchemas {
		oldSchema, newSchema := object(oldValue), object(newSchemas[name])
		if newSchema == nil {
			*issues = append(*issues, "removed schema "+name)
			continue
		}
		oldProperties, newProperties := object(oldSchema["properties"]), object(newSchema["properties"])
		for property, oldPropertyValue := range oldProperties {
			newProperty := object(newProperties[property])
			if newProperty == nil {
				*issues = append(*issues, fmt.Sprintf("removed property %s.%s", name, property))
				continue
			}
			oldType, _ := object(oldPropertyValue)["type"].(string)
			newType, _ := newProperty["type"].(string)
			if oldType != "" && newType != oldType {
				*issues = append(*issues, fmt.Sprintf("changed type of %s.%s from %s to %s", name, property, oldType, newType))
			}
		}
		for _, required := range stringsList(oldSchema["required"]) {
			if !contains(stringsList(newSchema["required"]), required) {
				*issues = append(*issues, fmt.Sprintf("removed required field %s.%s", name, required))
			}
		}
	}
}

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func stringsList(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func httpMethod(value string) bool {
	switch value {
	case "get", "put", "post", "patch", "delete":
		return true
	default:
		return false
	}
}
