package observability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestProviderExportsSampledSpansWithServiceResource(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	res, err := newResource(Config{ServiceName: "rabbit-jetstream", ServiceVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	provider := newTraceProvider(exporter, res, 1)
	_, span := provider.Tracer("test").Start(context.Background(), "queue.apply")
	span.End()
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 || spans[0].Name != "queue.apply" {
		t.Fatalf("spans=%+v", spans)
	}
	attrs := spans[0].Resource.Attributes()
	foundName, foundVersion := false, false
	for _, attr := range attrs {
		foundName = foundName || attr.Key == "service.name" && attr.Value.AsString() == "rabbit-jetstream"
		foundVersion = foundVersion || attr.Key == "service.version" && attr.Value.AsString() == "test"
	}
	if !foundName || !foundVersion {
		t.Fatalf("resource attributes=%+v", attrs)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestInitExportsOTLPHTTP(t *testing.T) {
	received := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/traces" || r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Errorf("path=%s content-type=%s", r.URL.Path, r.Header.Get("Content-Type"))
		}
		received <- len(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	shutdown, err := Init(context.Background(), Config{Endpoint: server.URL + "/v1/traces", ServiceName: "integration", ServiceVersion: "test", SampleRatio: 1, AllowInsecure: true})
	if err != nil {
		t.Fatal(err)
	}
	_, span := otel.Tracer("integration").Start(context.Background(), "exported")
	span.End()
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	otel.SetTracerProvider(trace.NewNoopTracerProvider())
	select {
	case size := <-received:
		if size == 0 {
			t.Fatal("empty OTLP request")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OTLP request was not received")
	}
}

func TestProviderDropsUnsampledSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	res, err := newResource(Config{ServiceName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	provider := newTraceProvider(exporter, res, 0)
	_, span := provider.Tracer("test").Start(context.Background(), "dropped")
	span.End()
	_ = provider.ForceFlush(context.Background())
	if spans := exporter.GetSpans(); len(spans) != 0 {
		t.Fatalf("unsampled spans=%+v", spans)
	}
	_ = provider.Shutdown(context.Background())
}

func TestInitExportsOTLPMetrics(t *testing.T) {
	received := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/metrics" || r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Errorf("path=%s content-type=%s", r.URL.Path, r.Header.Get("Content-Type"))
		}
		received <- len(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	shutdown, err := Init(context.Background(), Config{MetricsEndpoint: server.URL + "/v1/metrics", ServiceName: "integration", ServiceVersion: "test", SampleRatio: 0.1, MetricInterval: time.Hour, AllowInsecure: true})
	if err != nil {
		t.Fatal(err)
	}
	counter, err := otel.Meter("integration").Int64Counter("rjs.test.operations")
	if err != nil {
		t.Fatal(err)
	}
	counter.Add(context.Background(), 1)
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case size := <-received:
		if size == 0 {
			t.Fatal("empty OTLP metrics request")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OTLP metrics request was not received")
	}
}

func TestInitRejectsUnsafeEndpointAndInvalidRatio(t *testing.T) {
	if _, err := Init(context.Background(), Config{Endpoint: "http://collector:4318/v1/traces", SampleRatio: 1}); err == nil {
		t.Fatal("insecure endpoint was accepted")
	}
	if _, err := Init(context.Background(), Config{Endpoint: "https://collector.example/v1/traces", SampleRatio: 2}); err == nil {
		t.Fatal("invalid sample ratio was accepted")
	}
	shutdown, err := Init(context.Background(), Config{})
	if err != nil || shutdown(context.Background()) != nil {
		t.Fatalf("disabled tracing err=%v", err)
	}
}
