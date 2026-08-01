package observability

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

type Config struct {
	Endpoint       string
	ServiceName    string
	ServiceVersion string
	SampleRatio    float64
	AllowInsecure  bool
}

// Init installs an OTLP/HTTP trace provider. An empty endpoint keeps the SDK disabled.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && !(cfg.AllowInsecure && endpoint.Scheme == "http")) {
		return nil, errors.New("OTLP trace endpoint must be an absolute HTTPS URL (HTTP requires RJS_OTEL_ALLOW_INSECURE=true)")
	}
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return nil, errors.New("trace sample ratio must be between 0 and 1")
	}
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	provider, err := newProvider(exporter, cfg)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}

func newProvider(exporter sdktrace.SpanExporter, cfg Config) (*sdktrace.TracerProvider, error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(cfg.ServiceName), semconv.ServiceVersion(cfg.ServiceVersion)))
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}
	return sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio)))), nil
}
