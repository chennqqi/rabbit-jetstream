package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// JSON redacts sensitive field names and absolute URL user information without
// converting numeric tokens to float64. Invalid JSON is never returned as data.
// Callers must bound input size and must not publish the original on error.
// This is not a general secret detector for free text or operational metadata.
func JSON(value []byte) ([]byte, error) {
	if !json.Valid(value) {
		return nil, errors.New("invalid diagnostic JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("invalid diagnostic JSON")
	}
	document = jsonValue(document, "")
	result, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, errors.New("cannot encode redacted diagnostic JSON")
	}
	return append(result, '\n'), nil
}

func jsonValue(value any, key string) any {
	for _, fragment := range []string{"authorization", "credential", "password", "secret", "token"} {
		if strings.Contains(strings.ToLower(key), fragment) {
			return "[REDACTED]"
		}
	}
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			typed[childKey] = jsonValue(child, childKey)
		}
	case []any:
		for index := range typed {
			typed[index] = jsonValue(typed[index], "")
		}
	case string:
		return URL(typed)
	}
	return value
}
