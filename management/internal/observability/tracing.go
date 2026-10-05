package observability

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

type Config struct {
	Endpoint        string
	MetricsEndpoint string
	MetricInterval  time.Duration
	ServiceName     string
	ServiceVersion  string
	SampleRatio     float64
	AllowInsecure   bool
}

// Init installs optional OTLP/HTTP trace and metric providers.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.Endpoint == "" && cfg.MetricsEndpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	if err := validateEndpoint(cfg.Endpoint, cfg.AllowInsecure); err != nil {
		return nil, fmt.Errorf("OTLP trace endpoint: %w", err)
	}
	if err := validateEndpoint(cfg.MetricsEndpoint, cfg.AllowInsecure); err != nil {
		return nil, fmt.Errorf("OTLP metrics endpoint: %w", err)
	}
	if cfg.Endpoint != "" && (cfg.SampleRatio < 0 || cfg.SampleRatio > 1) {
		return nil, errors.New("trace sample ratio must be between 0 and 1")
	}
	res, err := newResource(cfg)
	if err != nil {
		return nil, err
	}
	shutdowns := make([]func(context.Context) error, 0, 2)
	if cfg.Endpoint != "" {
		exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(cfg.Endpoint))
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		provider := newTraceProvider(exporter, res, cfg.SampleRatio)
		otel.SetTracerProvider(provider)
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	if cfg.MetricsEndpoint != "" {
		exporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(cfg.MetricsEndpoint))
		if err != nil {
			for i := len(shutdowns) - 1; i >= 0; i-- {
				_ = shutdowns[i](ctx)
			}
			return nil, fmt.Errorf("create OTLP metrics exporter: %w", err)
		}
		interval := cfg.MetricInterval
		if interval <= 0 {
			interval = 30 * time.Second
		}
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(interval))))
		otel.SetMeterProvider(provider)
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func(ctx context.Context) error {
		var result error
		for i := len(shutdowns) - 1; i >= 0; i-- {
			result = errors.Join(result, shutdowns[i](ctx))
		}
		return result
	}, nil
}

func newResource(cfg Config) (*resource.Resource, error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(cfg.ServiceName), semconv.ServiceVersion(cfg.ServiceVersion)))
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}
	return res, nil
}

func newTraceProvider(exporter sdktrace.SpanExporter, res *resource.Resource, ratio float64) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))))
}

func validateEndpoint(value string, allowInsecure bool) error {
	if value == "" {
		return nil
	}
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && !(allowInsecure && endpoint.Scheme == "http")) {
		return errors.New("must be an absolute HTTPS URL (HTTP requires RJS_OTEL_ALLOW_INSECURE=true)")
	}
	return nil
}
