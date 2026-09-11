package prometheus

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientUsesFixedQueryAndPreservesGapsAndResets(t *testing.T) {
	end := time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC)
	var requested *http.Request
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requested = r.Clone(r.Context())
		body := `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"rjs_uptime_seconds","secret":"drop"},"values":[[1789087500,"100.5"],[1789087560,"160.5"],[1789087680,"2.5"]]}]}}`
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	client, err := New(Config{BaseURL: "https://prometheus.example", BearerToken: "secret", HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.QueryRange(context.Background(), MetricUptime, "", Window1Hour, end)
	if err != nil {
		t.Fatal(err)
	}
	if requested.URL.Query().Get("query") != "rjs_uptime_seconds" || requested.URL.Query().Get("step") != "60" || requested.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("request=%v headers=%v", requested.URL, requested.Header)
	}
	if len(result.Series) != 1 || len(result.Series[0].Labels) != 0 || len(result.Series[0].Samples) != 3 || !result.Series[0].Samples[2].GapBefore || !result.Series[0].Samples[2].Reset {
		t.Fatalf("result=%#v", result)
	}
}

func TestQueueQueryIsExactAndCannotBecomePromQL(t *testing.T) {
	var query string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		query = r.URL.Query().Get("query")
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))}, nil
	})
	client, _ := New(Config{BaseURL: "http://prometheus:9090", AllowInsecure: true, HTTPClient: &http.Client{Transport: transport}})
	end := time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC)
	if _, err := client.QueryRange(context.Background(), MetricQueueMessages, "orders", Window15Minutes, end); err != nil {
		t.Fatal(err)
	}
	if query != `rjs_queue_messages{queue="orders"}` {
		t.Fatalf("query=%q", query)
	}
	for _, queue := range []string{"", `x"} or vector(1)`, "a/b"} {
		if _, err := client.QueryRange(context.Background(), MetricQueueMessages, queue, Window15Minutes, end); err == nil {
			t.Fatalf("accepted %q", queue)
		}
	}
}

func TestClientRejectsUnsafeConfigurationAndUnboundedResponses(t *testing.T) {
	for _, cfg := range []Config{{BaseURL: "http://prometheus:9090"}, {BaseURL: "https://user:pass@example"}, {BaseURL: "https://example/path"}, {BaseURL: "https://example?x=1"}, {BaseURL: "https://example", BearerToken: " bad"}} {
		if _, err := New(cfg); err == nil {
			t.Fatalf("accepted %#v", cfg)
		}
	}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(io.LimitReader(strings.NewReader(strings.Repeat("x", responseLimit+1)), responseLimit+1))}, nil
	})
	client, _ := New(Config{BaseURL: "https://example", HTTPClient: &http.Client{Transport: transport}})
	if _, err := client.QueryRange(context.Background(), MetricUptime, "", Window24Hours, time.Now()); err == nil {
		t.Fatal("accepted oversized response")
	}
}

func TestClientNeverFollowsPrometheusRedirects(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"http://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	client, _ := New(Config{BaseURL: "https://example", HTTPClient: &http.Client{Transport: transport}})
	if _, err := client.QueryRange(context.Background(), MetricUptime, "", Window15Minutes, time.Now()); err == nil {
		t.Fatal("accepted redirect")
	}
	if calls != 1 {
		t.Fatalf("redirect followed: calls=%d", calls)
	}
}

func TestDecoderRejectsFabricatedOrUnsafeSamples(t *testing.T) {
	step := time.Minute
	start := time.Unix(100, 0)
	end := time.Unix(500, 0)
	for _, body := range []string{`{"status":"error","data":{"resultType":"matrix","result":[]}}`, `{"status":"success","data":{"resultType":"vector","result":[]}}`, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[200,"+Inf"]]}]}}`, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[200,"1"],[199,"2"]]}]}}`} {
		if _, err := decodeRange([]byte(body), querySpec{}, step, start, end); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestDecoderTreatsNaNAsMissingAndAcceptsBoundedDiagnostics(t *testing.T) {
	body := `{"status":"success","warnings":["partial scrape"],"infos":[],"data":{"resultType":"matrix","result":[{"metric":{},"values":[[200,"1"],[260,"NaN"],[320,"2"]]}]}}`
	series, err := decodeRange([]byte(body), querySpec{}, time.Minute, time.Unix(100, 0), time.Unix(500, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(series[0].Samples) != 2 || !series[0].Samples[1].GapBefore {
		t.Fatalf("series=%#v", series)
	}
}

func TestDecoderRejectsDuplicateProjectedSeries(t *testing.T) {
	body := `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"one"},"values":[]},{"metric":{"pod":"two"},"values":[]}]}}`
	if _, err := decodeRange([]byte(body), querySpec{labels: map[string]bool{"instance": true}}, time.Minute, time.Unix(100, 0), time.Unix(500, 0)); err == nil {
		t.Fatal("accepted duplicate projected series")
	}
}
