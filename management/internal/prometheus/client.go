// Package prometheus provides a bounded client for the repository's declared
// history backend. Callers select fixed query identifiers; browser-supplied
// PromQL and arbitrary upstream URLs are deliberately impossible.
package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const responseLimit = 4 << 20

type MetricID string
type Window string

const (
	MetricJetStreamMemory  MetricID = "jetstream-memory-bytes"
	MetricJetStreamStorage MetricID = "jetstream-storage-bytes"
	MetricQueueMessages    MetricID = "queue-messages"
	MetricQueueBytes       MetricID = "queue-bytes"
	MetricNATSNodes        MetricID = "nats-nodes"
	MetricUptime           MetricID = "management-uptime-seconds"

	Window15Minutes Window = "15m"
	Window1Hour     Window = "1h"
	Window24Hours   Window = "24h"
)

type Config struct {
	BaseURL       string
	PublicURL     string
	BearerToken   string
	AllowInsecure bool
	HTTPClient    *http.Client
}

type Client struct {
	base       *url.URL
	publicURL  string
	token      string
	http       *http.Client
	alertState alertMemory
}

type Range struct {
	Schema      string    `json:"schema"`
	Source      string    `json:"source"`
	Metric      MetricID  `json:"metric"`
	Window      Window    `json:"window"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	StepSeconds int       `json:"stepSeconds"`
	Series      []Series  `json:"series"`
}

type Series struct {
	Labels  map[string]string `json:"labels"`
	Samples []Sample          `json:"samples"`
}
type Sample struct {
	Time      time.Time `json:"time"`
	Value     string    `json:"value"`
	GapBefore bool      `json:"gapBefore,omitempty"`
	Reset     bool      `json:"reset,omitempty"`
}

type querySpec struct {
	expression string
	labels     map[string]bool
	queue      bool
	reset      bool
}

var specs = map[MetricID]querySpec{
	MetricJetStreamMemory:  {expression: "rjs_jetstream_memory_bytes", labels: map[string]bool{"instance": true}},
	MetricJetStreamStorage: {expression: "rjs_jetstream_storage_bytes", labels: map[string]bool{"instance": true}},
	MetricQueueMessages:    {expression: `rjs_queue_messages{queue=%s}`, labels: map[string]bool{"queue": true, "instance": true}, queue: true},
	MetricQueueBytes:       {expression: `rjs_queue_bytes{queue=%s}`, labels: map[string]bool{"queue": true, "instance": true}, queue: true},
	MetricNATSNodes:        {expression: "rjs_nats_nodes", labels: map[string]bool{"status": true, "instance": true}},
	MetricUptime:           {expression: "rjs_uptime_seconds", labels: map[string]bool{"instance": true}, reset: true},
}

var queuePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func New(cfg Config) (*Client, error) {
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("Prometheus URL must be an origin without credentials, query, fragment, or path")
	}
	if parsed.Scheme != "https" && !(cfg.AllowInsecure && parsed.Scheme == "http") {
		return nil, errors.New("Prometheus URL must use HTTPS")
	}
	if strings.TrimSpace(cfg.BearerToken) != cfg.BearerToken || strings.IndexFunc(cfg.BearerToken, func(r rune) bool { return r < ' ' || r == 127 }) >= 0 {
		return nil, errors.New("Prometheus bearer token contains whitespace or controls")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	parsed.Path = ""
	publicURL := ""
	if cfg.PublicURL != "" {
		public, parseErr := url.Parse(cfg.PublicURL)
		if parseErr != nil || public.Scheme == "" || public.Host == "" || public.User != nil || public.RawQuery != "" || public.Fragment != "" || (public.Path != "" && public.Path != "/") || public.Scheme != "https" && !(cfg.AllowInsecure && public.Scheme == "http") {
			return nil, errors.New("Prometheus public URL must be an HTTPS origin without credentials, query, fragment, or path")
		}
		publicURL = strings.TrimSuffix(public.String(), "/") + "/alerts"
	}
	return &Client{base: parsed, publicURL: publicURL, token: cfg.BearerToken, http: &copyClient, alertState: alertMemory{firing: map[string]bool{}, recovered: map[string]time.Time{}}}, nil
}

func (c *Client) QueryRange(ctx context.Context, metric MetricID, queue string, window Window, end time.Time) (Range, error) {
	if err := ValidateQuery(metric, queue, window); err != nil {
		return Range{}, err
	}
	spec := specs[metric]
	duration, step, _ := windowBounds(window)
	end = end.UTC().Truncate(time.Second)
	start := end.Add(-duration)
	expression := spec.expression
	if spec.queue {
		expression = fmt.Sprintf(expression, strconv.Quote(queue))
	}
	endpoint := *c.base
	endpoint.Path = "/api/v1/query_range"
	values := endpoint.Query()
	values.Set("query", expression)
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", strconv.Itoa(int(step.Seconds())))
	values.Set("timeout", "5s")
	endpoint.RawQuery = values.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Range{}, err
	}
	request.Header.Set("Accept", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return Range{}, fmt.Errorf("query Prometheus history: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return Range{}, errors.New("read Prometheus history response")
	}
	if len(body) > responseLimit {
		return Range{}, errors.New("Prometheus history response exceeds limit")
	}
	if response.StatusCode != 200 {
		return Range{}, fmt.Errorf("Prometheus history returned HTTP %d", response.StatusCode)
	}
	if media := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])); media != "application/json" {
		return Range{}, errors.New("Prometheus history returned non-JSON content")
	}
	series, err := decodeRange(body, spec, step, start, end)
	if err != nil {
		return Range{}, err
	}
	return Range{Schema: "rjs.metric-history.v1", Source: "prometheus", Metric: metric, Window: window, Start: start, End: end, StepSeconds: int(step.Seconds()), Series: series}, nil
}

func ValidateQuery(metric MetricID, queue string, window Window) error {
	spec, ok := specs[metric]
	if !ok {
		return errors.New("unsupported history metric")
	}
	if _, _, ok := windowBounds(window); !ok {
		return errors.New("unsupported history window")
	}
	if spec.queue {
		if !queuePattern.MatchString(queue) {
			return errors.New("invalid Queue identity")
		}
	} else if queue != "" {
		return errors.New("Queue is not valid for this metric")
	}
	return nil
}

func windowBounds(window Window) (time.Duration, time.Duration, bool) {
	switch window {
	case Window15Minutes:
		return 15 * time.Minute, 15 * time.Second, true
	case Window1Hour:
		return time.Hour, time.Minute, true
	case Window24Hours:
		return 24 * time.Hour, 5 * time.Minute, true
	default:
		return 0, 0, false
	}
}

func decodeRange(body []byte, spec querySpec, step time.Duration, start, end time.Time) ([]Series, error) {
	var envelope struct {
		Status   string   `json:"status"`
		Warnings []string `json:"warnings,omitempty"`
		Infos    []string `json:"infos,omitempty"`
		Data     struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string   `json:"metric"`
				Values [][]json.RawMessage `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, errors.New("invalid Prometheus history response")
	}
	if envelope.Status != "success" || envelope.Data.ResultType != "matrix" || len(envelope.Data.Result) > 100 {
		return nil, errors.New("incompatible Prometheus history response")
	}
	if len(envelope.Warnings) > 32 || len(envelope.Infos) > 32 {
		return nil, errors.New("Prometheus diagnostic limit exceeded")
	}
	result := make([]Series, 0, len(envelope.Data.Result))
	identities := make(map[string]bool)
	for _, wire := range envelope.Data.Result {
		labels := map[string]string{}
		for key, value := range wire.Metric {
			if spec.labels[key] {
				if len(value) > 256 || strings.IndexFunc(value, func(r rune) bool { return r < ' ' || r == 127 }) >= 0 {
					return nil, errors.New("invalid Prometheus series label")
				}
				labels[key] = value
			}
		}
		identityBytes, _ := json.Marshal(labels)
		identity := string(identityBytes)
		if identities[identity] {
			return nil, errors.New("duplicate projected Prometheus series")
		}
		identities[identity] = true
		if len(wire.Values) > 6000 {
			return nil, errors.New("Prometheus history sample limit exceeded")
		}
		samples := make([]Sample, 0, len(wire.Values))
		var previous time.Time
		var previousValue float64
		for index, pair := range wire.Values {
			if len(pair) != 2 {
				return nil, errors.New("invalid Prometheus sample")
			}
			var timestamp json.Number
			var value string
			if err := json.Unmarshal(pair[0], &timestamp); err != nil || json.Unmarshal(pair[1], &value) != nil {
				return nil, errors.New("invalid Prometheus sample")
			}
			seconds, err := strconv.ParseFloat(timestamp.String(), 64)
			if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < float64(start.Unix()) || seconds > float64(end.Unix()+1) {
				return nil, errors.New("invalid Prometheus timestamp")
			}
			if value == "NaN" {
				continue
			}
			numeric, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(numeric) || math.IsInf(numeric, 0) {
				return nil, errors.New("invalid Prometheus value")
			}
			observed := time.Unix(0, int64(seconds*float64(time.Second))).UTC()
			if observed.Before(start) || observed.After(end) || index > 0 && !observed.After(previous) {
				return nil, errors.New("out-of-range or unordered Prometheus sample")
			}
			sample := Sample{Time: observed, Value: value}
			if index > 0 {
				sample.GapBefore = observed.Sub(previous) > step+step/2
				sample.Reset = spec.reset && numeric < previousValue
			}
			samples = append(samples, sample)
			previous, previousValue = observed, numeric
		}
		result = append(result, Series{Labels: labels, Samples: samples})
	}
	return result, nil
}
