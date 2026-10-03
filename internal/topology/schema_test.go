package topology

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestQueueSchemaCorpusAgainstParser(t *testing.T) {
	data, err := os.ReadFile("testdata/queue-schema-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name        string
		ParserValid bool
		Document    json.RawMessage
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := ParseQueue(bytes.NewReader(tc.Document))
			if (err == nil) != tc.ParserValid {
				t.Fatalf("parser valid=%v: %v", tc.ParserValid, err)
			}
		})
	}
}

func TestQueueSchemaCoversDocumentFields(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(QueueSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	if schema["$id"] != QueueSchemaID || schema["x-rjs-version"] != QueueSchemaVersion {
		t.Fatal("schema identity drift")
	}
	var check func(reflect.Type, map[string]any)
	check = func(typ reflect.Type, node map[string]any) {
		properties := node["properties"].(map[string]any)
		if node["additionalProperties"] != false || len(properties) != typ.NumField() {
			t.Fatalf("schema fields diverged for %v", typ)
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			value, ok := properties[name].(map[string]any)
			if !ok {
				t.Fatalf("missing %v.%s", typ, name)
			}
			ft := field.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct {
				check(ft.Elem(), value["items"].(map[string]any))
			} else if ft.Kind() == reflect.Struct {
				check(ft, value)
			}
		}
	}
	check(reflect.TypeOf(Queue{}), schema)
	spec := schema["properties"].(map[string]any)["spec"].(map[string]any)["properties"].(map[string]any)
	var defaults Queue
	defaults.Default()
	if spec["storage"].(map[string]any)["default"] != defaults.Spec.Storage {
		t.Fatal("storage default drift")
	}
	delivery := spec["delivery"].(map[string]any)["properties"].(map[string]any)
	ack, _ := defaults.Spec.Delivery.AckWait.MarshalText()
	if delivery["ackWait"].(map[string]any)["default"] != string(ack) || delivery["maxDeliver"].(map[string]any)["default"] != float64(*defaults.Spec.Delivery.MaxDeliver) {
		t.Fatal("delivery default drift")
	}
	priority := spec["maxPriority"].(map[string]any)
	if priority["minimum"] != float64(MinimumPriority) || priority["maximum"] != float64(MaximumPriority) {
		t.Fatal("priority drift")
	}
	if _, ok := spec["replicas"].(map[string]any)["default"]; ok {
		t.Fatal("invented replica default")
	}
	original, tag := QueueSchema(), QueueSchemaETag()
	copy := QueueSchema()
	copy[0] = '!'
	if !bytes.Equal(original, QueueSchema()) || tag != QueueSchemaETag() {
		t.Fatal("caller mutated schema")
	}
}
