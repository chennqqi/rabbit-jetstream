package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

var alertNames = map[string]bool{
	"RabbitJetStreamJetStreamUnavailable": true, "RabbitJetStreamControllerStalled": true,
	"RabbitJetStreamNATSNodeUnavailable": true, "RabbitJetStreamDeadLetterFailures": true,
	"RabbitJetStreamQueueBacklogHigh": true, "RabbitJetStreamManagementHighErrorRate": true,
}

type AlertSnapshot struct {
	Schema       string      `json:"schema"`
	Source       string      `json:"source"`
	ObservedAt   time.Time   `json:"observedAt"`
	Rules        []AlertRule `json:"rules"`
	MissingRules []string    `json:"missingRules"`
	ConsoleURL   string      `json:"consoleUrl,omitempty"`
}
type AlertRule struct {
	Name        string     `json:"name"`
	Query       string     `json:"query"`
	Duration    string     `json:"duration"`
	Severity    string     `json:"severity"`
	Summary     string     `json:"summary"`
	Description string     `json:"description"`
	State       string     `json:"state"`
	ActiveAt    *time.Time `json:"activeAt,omitempty"`
	RecoveredAt *time.Time `json:"recoveredAt,omitempty"`
}

type alertMemory struct {
	sync.Mutex
	firing    map[string]bool
	recovered map[string]time.Time
}

func (c *Client) AlertRules(ctx context.Context, now time.Time) (AlertSnapshot, error) {
	if c == nil {
		return AlertSnapshot{}, ErrNotConfigured
	}
	endpoint := *c.base
	endpoint.Path = "/api/v1/rules"
	endpoint.RawQuery = url.Values{"type": {"alert"}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return AlertSnapshot{}, err
	}
	request.Header.Set("Accept", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return AlertSnapshot{}, errors.New("query Prometheus alert rules")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil || len(body) > responseLimit {
		return AlertSnapshot{}, errors.New("invalid Prometheus alert response size")
	}
	if response.StatusCode != http.StatusOK || strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])) != "application/json" {
		return AlertSnapshot{}, errors.New("Prometheus alert rules unavailable")
	}
	rules, err := decodeAlertRules(body)
	if err != nil {
		return AlertSnapshot{}, err
	}
	now = now.UTC().Truncate(time.Second)
	missing := make([]string, 0, len(alertNames)-len(rules))
	returned := make(map[string]bool, len(rules))
	for _, rule := range rules {
		returned[rule.Name] = true
	}
	for name := range alertNames {
		if !returned[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	c.alertState.Lock()
	defer c.alertState.Unlock()
	for index := range rules {
		name, current := rules[index].Name, rules[index].State == "firing"
		if current {
			delete(c.alertState.recovered, name)
		} else if c.alertState.firing[name] && rules[index].State == "inactive" {
			recovered := now
			c.alertState.recovered[name] = recovered
		}
		if recovered, ok := c.alertState.recovered[name]; ok && rules[index].State == "inactive" {
			rules[index].State, rules[index].RecoveredAt = "recovered", &recovered
		}
		c.alertState.firing[name] = current
	}
	return AlertSnapshot{Schema: "rjs.operational-alerts.v1", Source: "prometheus", ObservedAt: now, Rules: rules, MissingRules: missing, ConsoleURL: c.publicURL}, nil
}

func decodeAlertRules(body []byte) ([]AlertRule, error) {
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Groups []struct {
				Name           string  `json:"name"`
				File           string  `json:"file"`
				Interval       float64 `json:"interval"`
				Limit          int     `json:"limit"`
				EvaluationTime float64 `json:"evaluationTime"`
				LastEvaluation string  `json:"lastEvaluation"`
				Rules          []struct {
					Type           string            `json:"type"`
					State          string            `json:"state"`
					Name           string            `json:"name"`
					Query          string            `json:"query"`
					Duration       json.Number       `json:"duration"`
					KeepFiringFor  json.Number       `json:"keepFiringFor"`
					Health         string            `json:"health"`
					LastError      string            `json:"lastError"`
					EvaluationTime float64           `json:"evaluationTime"`
					LastEvaluation string            `json:"lastEvaluation"`
					Labels         map[string]string `json:"labels"`
					Annotations    map[string]string `json:"annotations"`
					Alerts         []struct {
						State           string            `json:"state"`
						ActiveAt        string            `json:"activeAt"`
						Value           string            `json:"value"`
						KeepFiringSince string            `json:"keepFiringSince"`
						Labels          map[string]string `json:"labels"`
						Annotations     map[string]string `json:"annotations"`
					} `json:"alerts"`
				} `json:"rules"`
			} `json:"groups"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&envelope) != nil || envelope.Status != "success" || len(envelope.Data.Groups) > 32 {
		return nil, errors.New("invalid Prometheus alert rules response")
	}
	result, seen := []AlertRule{}, map[string]bool{}
	for _, group := range envelope.Data.Groups {
		if len(group.Rules) > 100 {
			return nil, errors.New("Prometheus alert rule limit exceeded")
		}
		for _, wire := range group.Rules {
			if !alertNames[wire.Name] {
				continue
			}
			duration := wire.Duration.String()
			if seen[wire.Name] || len(wire.Query) > 2048 || len(duration) > 32 {
				return nil, errors.New("invalid Prometheus alert rule")
			}
			seen[wire.Name] = true
			rule := AlertRule{Name: wire.Name, Query: wire.Query, Duration: duration, Severity: wire.Labels["severity"], Summary: wire.Annotations["summary"], Description: wire.Annotations["description"], State: "inactive"}
			for _, instance := range wire.Alerts {
				switch instance.State {
				case "firing":
					rule.State = "firing"
				case "pending":
					if rule.State != "firing" {
						rule.State = "pending"
					}
				default:
					return nil, errors.New("invalid Prometheus alert state")
				}
				if instance.ActiveAt != "" {
					parsed, parseErr := time.Parse(time.RFC3339Nano, instance.ActiveAt)
					if parseErr != nil {
						return nil, errors.New("invalid Prometheus alert time")
					}
					parsed = parsed.UTC()
					if rule.ActiveAt == nil || parsed.Before(*rule.ActiveAt) {
						rule.ActiveAt = &parsed
					}
				}
			}
			result = append(result, rule)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
