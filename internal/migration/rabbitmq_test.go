package migration

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestConvertRabbitMQDefinitions(t *testing.T) {
	file, err := os.Open("../../tests/fixtures/rabbitmq-definitions.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	result, err := ConvertRabbitMQ(file, "/", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Report.Compatible || result.Report.Queues != 3 || result.Report.Converted != 3 || len(result.Queues) != 3 {
		t.Fatalf("result=%#v", result)
	}
	var ordersFound bool
	for _, queue := range result.Queues {
		if err := queue.Validate(); err != nil {
			t.Fatalf("Queue %s invalid: %v", queue.Metadata.Name, err)
		}
		if queue.Metadata.Name == "orders" {
			ordersFound = true
			if len(queue.Spec.Bindings) != 2 || time.Duration(queue.Spec.Retention.MaxAge) != time.Minute || queue.Spec.Retention.MaxMessages != 10000 || queue.Spec.Retention.MaxBytes != 1048576 || queue.Spec.DeadLetter == nil || queue.Spec.DeadLetter.Queue != "orders_dlq" {
				t.Fatalf("orders=%#v", queue)
			}
		}
	}
	if !ordersFound {
		t.Fatal("orders Queue was not converted")
	}
}

func TestConvertRabbitMQReportsUnsafeSemantics(t *testing.T) {
	input := `{
"queues":[{"name":"bad.name","vhost":"/","durable":false,"auto_delete":true,"arguments":{"x-queue-type":"stream","x-expires":1000}}],
"exchanges":[{"name":"headers","vhost":"/","type":"headers","durable":true,"auto_delete":false,"arguments":{}}],
"bindings":[
 {"source":"headers","vhost":"/","destination":"bad.name","destination_type":"queue","routing_key":"","arguments":{"x-match":"all"}},
 {"source":"headers","vhost":"/","destination":"next","destination_type":"exchange","routing_key":"","arguments":{}}
],"policies":[{"name":"effective-policy","vhost":"/"}]}`
	result, err := ConvertRabbitMQ(strings.NewReader(input), "/", 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Report.Compatible || result.Report.Converted != 0 {
		t.Fatalf("report=%#v", result.Report)
	}
	encoded := ""
	for _, issue := range result.Report.Issues {
		encoded += issue.Code + " "
	}
	for _, code := range []string{"exchange_binding_unsupported", "policies_unsupported", "invalid_name", "transient_queue_unsupported", "exchange_type_unsupported", "rabbit_stream_unsupported", "queue_argument_unsupported"} {
		if !strings.Contains(encoded, code) {
			t.Fatalf("issues=%s missing %s", encoded, code)
		}
	}
}

func TestConvertRabbitMQRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		input    string
		replicas int
	}{{"not-json", 3}, {`{} {}`, 3}, {`{}`, 2}} {
		if _, err := ConvertRabbitMQ(strings.NewReader(test.input), "/", test.replicas); err == nil {
			t.Fatalf("input=%q replicas=%d succeeded", test.input, test.replicas)
		}
	}
}

func TestMigrationHelpers(t *testing.T) {
	if value, ok := integer(nil); ok || value != 0 {
		t.Fatal("nil integer succeeded")
	}
	if stringValue(1) != "" || len(deadLetterTargets("missing", "", nil, nil)) != 0 {
		t.Fatal("helper behavior mismatch")
	}
}

func TestConvertRabbitMQReportsNoQueuesAndInvalidArguments(t *testing.T) {
	result, err := ConvertRabbitMQ(strings.NewReader(`{"queues":[],"policies":[{"name":"other","vhost":"other"}]}`), "/", 3)
	if err != nil || result.Report.Compatible || len(result.Report.Issues) != 1 || result.Report.Issues[0].Code != "no_queues" {
		t.Fatalf("report=%#v err=%v", result.Report, err)
	}
	input := `{"queues":[{"name":"orders","vhost":"/","durable":true,"auto_delete":false,"arguments":{"x-message-ttl":9223372036854775807,"x-queue-type":1,"x-dead-letter-routing-key":"failed"}}]}`
	result, err = ConvertRabbitMQ(strings.NewReader(input), "/", 3)
	if err != nil || result.Report.Compatible {
		t.Fatalf("report=%#v err=%v", result.Report, err)
	}
	codes := ""
	for _, issue := range result.Report.Issues {
		codes += issue.Code + " "
	}
	for _, code := range []string{"invalid_message_ttl", "queue_type_unsupported", "dead_letter_invalid"} {
		if !strings.Contains(codes, code) {
			t.Fatalf("codes=%s missing %s", codes, code)
		}
	}
}

func TestConvertRabbitMQPrunesInvalidAndCyclicDeadLetterDependencies(t *testing.T) {
	dangling := `{
"queues":[
 {"name":"source","vhost":"/","durable":true,"auto_delete":false,"arguments":{"x-dead-letter-exchange":"failed"}},
 {"name":"target","vhost":"/","durable":false,"auto_delete":false,"arguments":{}}
],
"exchanges":[{"name":"failed","vhost":"/","type":"fanout","durable":true,"auto_delete":false,"arguments":{}}],
"bindings":[{"source":"failed","vhost":"/","destination":"target","destination_type":"queue","routing_key":"","arguments":{}}]}`
	result, err := ConvertRabbitMQ(strings.NewReader(dangling), "/", 3)
	if err != nil || result.Report.Converted != 0 || !hasIssue(result.Report, "dead_letter_target_incompatible") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	cycle := `{
"queues":[
 {"name":"a","vhost":"/","durable":true,"auto_delete":false,"arguments":{"x-dead-letter-exchange":"to_b"}},
 {"name":"b","vhost":"/","durable":true,"auto_delete":false,"arguments":{"x-dead-letter-exchange":"to_a"}}
],
"exchanges":[
 {"name":"to_a","vhost":"/","type":"fanout","durable":true,"auto_delete":false,"arguments":{}},
 {"name":"to_b","vhost":"/","type":"fanout","durable":true,"auto_delete":false,"arguments":{}}
],
"bindings":[
 {"source":"to_a","vhost":"/","destination":"a","destination_type":"queue","routing_key":"","arguments":{}},
 {"source":"to_b","vhost":"/","destination":"b","destination_type":"queue","routing_key":"","arguments":{}}
]}`
	result, err = ConvertRabbitMQ(strings.NewReader(cycle), "/", 3)
	if err != nil || result.Report.Converted != 0 || !hasIssue(result.Report, "dead_letter_cycle") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func hasIssue(report Report, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
