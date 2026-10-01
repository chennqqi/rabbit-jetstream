package jetstream

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

var auditTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.[0-9]{1,9})?(Z|[+-][0-9]{2}:[0-9]{2})$`)

func (f AuditFilter) TimeBounds() (*time.Time, *time.Time, error) {
	var bounds [2]*time.Time
	for i, value := range []string{f.From, f.Until} {
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || !auditTimePattern.MatchString(value) {
			return nil, nil, fmt.Errorf("audit times must be RFC3339 with at most nine fractional digits")
		}
		// Require conventional timezone offset components, consistently with the UI.
		if value[len(value)-1] != 'Z' {
			suffix := value[len(value)-6:]
			if suffix[1:3] > "23" || suffix[4:6] > "59" {
				return nil, nil, fmt.Errorf("invalid audit timezone offset")
			}
		}
		bounds[i] = &parsed
	}
	if bounds[0] != nil && bounds[1] != nil && !bounds[0].Before(*bounds[1]) {
		return nil, nil, fmt.Errorf("from must precede until")
	}
	return bounds[0], bounds[1], nil
}

// AuditFilter fields are exact, case-sensitive conjunctions, not substring or
// regex queries. An empty field imposes no constraint.
type AuditFilter struct {
	RequestID string `json:"requestId"`
	Resource  string `json:"resource"`
	Actor     string `json:"actor"`
	Phase     string `json:"phase"`
	Action    string `json:"action"`
	Outcome   string `json:"outcome"`
	From      string `json:"from"`
	Until     string `json:"until"`
}

type AuditWindowPage struct {
	Items         []AuditEvent `json:"items"`
	StreamPresent bool         `json:"streamPresent"`
	FirstSequence uint64       `json:"firstSequence"`
	LastSequence  uint64       `json:"lastSequence"`
	Scanned       int          `json:"scanned"`
	Missing       int          `json:"missing"`
	NextBefore    *uint64      `json:"nextBefore"`
	Filter        AuditFilter  `json:"filter"`
}

func (c *Client) AuditWindow(ctx context.Context, filter AuditFilter, before *uint64) (AuditWindowPage, error) {
	from, until, err := filter.TimeBounds()
	if err != nil {
		return AuditWindowPage{}, err
	}
	page, err := c.scanAudit(ctx, "", before, func(event AuditEvent) bool {
		return ((from == nil && until == nil) || !event.Time.IsZero()) && (from == nil || !event.Time.Before(*from)) && (until == nil || event.Time.Before(*until)) &&
			(filter.RequestID == "" || event.RequestID == filter.RequestID) &&
			(filter.Resource == "" || event.ResourceName == filter.Resource) &&
			(filter.Actor == "" || event.Actor == filter.Actor) &&
			(filter.Phase == "" || event.Phase == filter.Phase) &&
			(filter.Action == "" || event.Action == filter.Action) &&
			(filter.Outcome == "" || event.Outcome == filter.Outcome)
	})
	if err != nil {
		return AuditWindowPage{}, err
	}
	return AuditWindowPage{Items: page.Items, StreamPresent: page.StreamPresent, FirstSequence: page.FirstSequence, LastSequence: page.LastSequence, Scanned: page.Scanned, Missing: page.Missing, NextBefore: page.NextBefore, Filter: filter}, nil
}
