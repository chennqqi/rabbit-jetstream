package redact

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONPreservesExactNumbersAndRedactsNestedValues(t *testing.T) {
	input := []byte(`{"uint":18446744073709551615,"int":9223372036854775807,"negative":-9223372036854775808,"large":9007199254740993,"decimal":1.234567890123456789,"exponent":1e400,"negativeZero":-0,"items":[{"APIcredential":{"value":"private"},"AUTHORIZATION":["private"],"accessToken":123,"url":"https://user:private@example.test/path"}],"status":"ok"}`)
	before := append([]byte(nil), input...)
	got, err := JSON(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, before) || strings.Contains(string(got), "private") {
		t.Fatal("input changed or private value retained")
	}
	decoder := json.NewDecoder(bytes.NewReader(got))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	for key, expected := range map[string]string{"uint": "18446744073709551615", "int": "9223372036854775807", "negative": "-9223372036854775808", "large": "9007199254740993", "decimal": "1.234567890123456789", "exponent": "1e400", "negativeZero": "-0"} {
		if document[key] != json.Number(expected) {
			t.Errorf("numeric token changed for %s", key)
		}
	}
	item := document["items"].([]any)[0].(map[string]any)
	for _, key := range []string{"APIcredential", "AUTHORIZATION", "accessToken"} {
		if item[key] != "[REDACTED]" {
			t.Errorf("field not redacted: %s", key)
		}
	}
	if item["url"] != "https://example.test/path" || document["status"] != "ok" {
		t.Fatal("non-sensitive fields changed unexpectedly")
	}
}

func TestJSONRejectsInvalidOrMultipleDocumentsWithoutReturningInput(t *testing.T) {
	for _, input := range []string{"", `{"password":"private"`, `{} {"secret":"private"}`, `{"number":NaN}`, `{"x":01}`, `null trailing`} {
		got, err := JSON([]byte(input))
		if err == nil || got != nil || strings.Contains(err.Error(), "private") {
			t.Fatal("invalid input did not fail closed")
		}
	}
	for _, input := range []string{"null", "true", `"ordinary text"`, `18446744073709551615`} {
		got, err := JSON([]byte(input))
		if err != nil || !json.Valid(got) {
			t.Fatal("valid scalar rejected")
		}
	}
}
