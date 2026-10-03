package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAlertRules_FixedEndpointProjectionAndRecovery(t *testing.T) {
	state := "firing"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules" || r.URL.RawQuery != "type=alert" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("request=%s?%s auth=%q", r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		alerts := `[]`
		if state == "firing" {
			alerts = `[{"labels":{"queue":"orders"},"annotations":{},"state":"firing","activeAt":"2026-09-11T00:00:00Z","value":"100001e0"}]`
		}
		fmt.Fprintf(w, `{"status":"success","data":{"groups":[{"name":"message","file":"rules.yml","rules":[{"state":"%s","name":"RabbitJetStreamQueueBacklogHigh","query":"rjs_queue_messages > 100000","duration":900,"labels":{"severity":"warning"},"annotations":{"summary":"Backlog","description":"Threshold"},"alerts":%s,"health":"ok","lastError":"","evaluationTime":0.01,"lastEvaluation":"2026-09-11T00:01:00Z","type":"alerting"}],"interval":15,"limit":0,"evaluationTime":0.02,"lastEvaluation":"2026-09-11T00:01:00Z"}]}}`, state, alerts)
	}))
	defer server.Close()
	client, err := New(Config{BaseURL: server.URL, PublicURL: server.URL, BearerToken: "secret", AllowInsecure: true})
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.AlertRules(context.Background(), time.Date(2026, 9, 11, 0, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if first.ConsoleURL != server.URL+"/alerts" || len(first.Rules) != 1 || len(first.MissingRules) != 5 || first.Rules[0].State != "firing" || first.Rules[0].Query != "rjs_queue_messages > 100000" || first.Rules[0].Duration != "900" {
		t.Fatalf("first=%+v", first)
	}
	state = "inactive"
	second, err := client.AlertRules(context.Background(), time.Date(2026, 9, 11, 0, 3, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if second.Rules[0].State != "recovered" || second.Rules[0].RecoveredAt == nil {
		t.Fatalf("second=%+v", second)
	}
}

func TestNewRejectsUnsafePublicAlertURL(t *testing.T) {
	if _, err := New(Config{BaseURL: "https://prometheus.example", PublicURL: "https://user:secret@metrics.example/path"}); err == nil {
		t.Fatal("expected unsafe public URL rejection")
	}
}

func TestDecodeAlertRulesRejectsDuplicateAllowlistedRule(t *testing.T) {
	body := []byte(`{"status":"success","data":{"groups":[{"rules":[{"name":"RabbitJetStreamJetStreamUnavailable","query":"a","duration":0,"labels":{},"annotations":{},"alerts":[]},{"name":"RabbitJetStreamJetStreamUnavailable","query":"b","duration":0,"labels":{},"annotations":{},"alerts":[]}]}]}}`)
	if _, err := decodeAlertRules(body); err == nil {
		t.Fatal("expected duplicate rejection")
	}
}
