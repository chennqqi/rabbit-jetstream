package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxConnectionResponseBytes = 2 << 20
const maxConnectionIdentityMatches = 1000

// ConnectionSample deliberately excludes free-form client names, addresses,
// authentication metadata, certificates and subscription subjects. It is an
// internal transport projection, not the complete public diagnostic contract.
type ConnectionSample struct {
	CID           uint64  `json:"cid"`
	PendingBytes  *int64  `json:"pending_bytes,omitempty"`
	InMessages    *int64  `json:"in_msgs,omitempty"`
	OutMessages   *int64  `json:"out_msgs,omitempty"`
	InBytes       *int64  `json:"in_bytes,omitempty"`
	OutBytes      *int64  `json:"out_bytes,omitempty"`
	Subscriptions *uint32 `json:"subscriptions,omitempty"`
}

type ConnectionPage struct {
	NodeID     string             `json:"node_id"`
	ObservedAt time.Time          `json:"observed_at"`
	ReadAt     time.Time          `json:"read_at"`
	Offset     int                `json:"offset"`
	Limit      int                `json:"limit"`
	Total      int                `json:"total"`
	Items      []ConnectionSample `json:"items"`
}

// ConnectionPage reads one open-connection page from a configured endpoint.
// The caller must anchor expectedID to a previously observed node identity.
// No endpoint supplied by an HTTP caller may be substituted for endpointIndex.
// Pagination is a live observation, not a cross-page snapshot or global search.
func (c *Client) ConnectionPage(ctx context.Context, endpointIndex int, expectedID string, offset, limit int) (*ConnectionPage, error) {
	if endpointIndex < 0 || endpointIndex >= len(c.endpoints) || strings.TrimSpace(expectedID) == "" || offset < 0 || offset > 1_000_000 || limit < 1 || limit > 200 {
		return nil, errors.New("invalid connection page request")
	}
	body, err := c.connectionRead(ctx, endpointIndex, "/connz", url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}, "sort": {"cid"}, "state": {"open"}, "auth": {"false"}, "subs": {"false"}})
	if err != nil {
		return nil, err
	}
	return decodeConnectionPage(body, expectedID, offset, limit)
}

// ConnectionCIDPage performs an exact server-side CID search. It returns a
// search total of zero or one; the upstream node-wide total is intentionally
// not reused because it is not the filtered total.
func (c *Client) ConnectionCIDPage(ctx context.Context, endpointIndex int, expectedID string, cid uint64, limit int) (*ConnectionPage, error) {
	if endpointIndex < 0 || endpointIndex >= len(c.endpoints) || !validConnectionNodeID(expectedID) || cid == 0 || limit < 1 || limit > 200 {
		return nil, ErrConnectionQuery
	}
	body, err := c.connectionRead(ctx, endpointIndex, "/connz", url.Values{"cid": {strconv.FormatUint(cid, 10)}, "offset": {"0"}, "limit": {"1"}, "sort": {"cid"}, "state": {"open"}, "auth": {"false"}, "subs": {"false"}})
	if err != nil {
		return nil, ErrConnectionUnavailable
	}
	page, err := decodeConnectionPage(body, expectedID, 0, 1)
	if err != nil || len(page.Items) > 1 || len(page.Items) == 1 && page.Items[0].CID != cid {
		return nil, ErrConnectionUnavailable
	}
	page.Offset, page.Limit, page.Total = 0, limit, len(page.Items)
	return page, nil
}

// ConnectionIdentityPage uses the pinned server's exact identity filters before
// pagination, but computes the filtered total itself because connz reports the
// unfiltered node total. Sensitive identity fields are never decoded into the
// returned projection.
func (c *Client) ConnectionIdentityPage(ctx context.Context, endpointIndex int, expectedID, kind, value string, offset, limit int) (*ConnectionPage, error) {
	if endpointIndex < 0 || endpointIndex >= len(c.endpoints) || !validConnectionNodeID(expectedID) || !validConnectionIdentity(kind, value) || offset < 0 || offset > 1_000_000 || limit < 1 || limit > 200 {
		return nil, ErrConnectionQuery
	}
	if kind == "name" {
		return c.connectionNamePage(ctx, endpointIndex, expectedID, value, offset, limit)
	}
	query := url.Values{"offset": {"0"}, "limit": {"200"}, "sort": {"cid"}, "state": {"open"}, "auth": {"false"}, "subs": {"false"}}
	switch kind {
	case "user":
		query.Set("auth", "true")
		query.Set("user", value)
	case "account":
		query.Set("auth", "true")
		query.Set("acc", value)
	case "mqtt_client":
		query.Set("mqtt_client", value)
	}
	collect := func() ([]ConnectionSample, time.Time, error) {
		items := make([]ConnectionSample, 0)
		var observed time.Time
		var previous uint64
		for upstreamOffset := 0; ; upstreamOffset += 200 {
			query.Set("offset", strconv.Itoa(upstreamOffset))
			body, err := c.connectionRead(ctx, endpointIndex, "/connz", query)
			if err != nil {
				return nil, time.Time{}, ErrConnectionUnavailable
			}
			page, err := decodeConnectionPage(body, expectedID, upstreamOffset, 200)
			if err != nil {
				return nil, time.Time{}, ErrConnectionUnavailable
			}
			observed = page.ObservedAt
			for _, item := range page.Items {
				if item.CID <= previous {
					return nil, time.Time{}, ErrConnectionUnavailable
				}
				previous = item.CID
			}
			items = append(items, page.Items...)
			if len(items) > maxConnectionIdentityMatches {
				return nil, time.Time{}, ErrConnectionSearchLimit
			}
			if len(page.Items) < 200 {
				return items, observed, nil
			}
		}
	}
	first, _, err := collect()
	if err != nil {
		return nil, err
	}
	items, observed, err := collect()
	if err != nil || len(first) != len(items) {
		return nil, ErrConnectionUnavailable
	}
	for index := range first {
		if first[index].CID != items[index].CID {
			return nil, ErrConnectionUnavailable
		}
	}
	start := min(offset, len(items))
	end := min(start+limit, len(items))
	return &ConnectionPage{NodeID: expectedID, ObservedAt: observed, ReadAt: time.Now().UTC(), Offset: offset, Limit: limit, Total: len(items), Items: items[start:end]}, nil
}

func validConnectionIdentity(kind, value string) bool {
	if kind != "name" && kind != "user" && kind != "account" && kind != "mqtt_client" || value == "" || len(value) > 256 {
		return false
	}
	return strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) < 0
}

type namedConnectionSample struct {
	ConnectionSample
	Name string `json:"name"`
}

func (c *Client) connectionNamePage(ctx context.Context, endpointIndex int, expectedID, value string, offset, limit int) (*ConnectionPage, error) {
	query := url.Values{"offset": {"0"}, "limit": {"200"}, "sort": {"cid"}, "state": {"open"}, "auth": {"false"}, "subs": {"false"}}
	collect := func() ([]namedConnectionSample, time.Time, error) {
		all := make([]namedConnectionSample, 0)
		var observed time.Time
		var previous uint64
		for upstreamOffset := 0; ; upstreamOffset += 200 {
			query.Set("offset", strconv.Itoa(upstreamOffset))
			body, err := c.connectionRead(ctx, endpointIndex, "/connz", query)
			if err != nil {
				return nil, time.Time{}, ErrConnectionUnavailable
			}
			var wire struct {
				ID     string                  `json:"server_id"`
				Now    time.Time               `json:"now"`
				Count  *int                    `json:"num_connections"`
				Total  *int                    `json:"total"`
				Offset *int                    `json:"offset"`
				Limit  *int                    `json:"limit"`
				Items  []namedConnectionSample `json:"connections"`
			}
			if json.Unmarshal(body, &wire) != nil || wire.ID != expectedID || wire.Now.IsZero() || wire.Count == nil || wire.Total == nil || wire.Offset == nil || wire.Limit == nil || wire.Items == nil || *wire.Total < 0 || *wire.Total > maxConnectionIdentityMatches || *wire.Count != len(wire.Items) || *wire.Count > 200 || *wire.Offset != upstreamOffset || *wire.Limit != 200 {
				if wire.Total != nil && *wire.Total > maxConnectionIdentityMatches {
					return nil, time.Time{}, ErrConnectionSearchLimit
				}
				return nil, time.Time{}, ErrConnectionUnavailable
			}
			for _, item := range wire.Items {
				if item.CID <= previous {
					return nil, time.Time{}, ErrConnectionUnavailable
				}
				for _, counter := range []*int64{item.PendingBytes, item.InMessages, item.OutMessages, item.InBytes, item.OutBytes} {
					if counter != nil && *counter < 0 {
						return nil, time.Time{}, ErrConnectionUnavailable
					}
				}
				previous = item.CID
			}
			all = append(all, wire.Items...)
			observed = wire.Now
			if len(wire.Items) < 200 {
				if len(all) != *wire.Total {
					return nil, time.Time{}, ErrConnectionUnavailable
				}
				return all, observed, nil
			}
		}
	}
	first, _, err := collect()
	if err != nil {
		return nil, err
	}
	second, observed, err := collect()
	if err != nil || len(first) != len(second) {
		return nil, ErrConnectionUnavailable
	}
	items := make([]ConnectionSample, 0)
	for index := range first {
		if first[index].CID != second[index].CID || first[index].Name != second[index].Name {
			return nil, ErrConnectionUnavailable
		}
		if second[index].Name == value {
			items = append(items, second[index].ConnectionSample)
		}
	}
	start := min(offset, len(items))
	end := min(start+limit, len(items))
	return &ConnectionPage{NodeID: expectedID, ObservedAt: observed, ReadAt: time.Now().UTC(), Offset: offset, Limit: limit, Total: len(items), Items: items[start:end]}, nil
}

// Only private callers select the fixed varz/connz paths; no arbitrary URL is accepted.
func (c *Client) connectionRead(ctx context.Context, endpointIndex int, path string, query url.Values) ([]byte, error) {
	u, err := url.Parse(c.endpoints[endpointIndex])
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid configured monitoring endpoint")
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawPath = ""
	u.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("create connection monitoring request")
	}
	client := *c.http
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("connection monitoring unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("connection monitoring returned non-success status")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxConnectionResponseBytes+1))
	if err != nil || len(body) > maxConnectionResponseBytes {
		return nil, errors.New("connection monitoring response incomplete or too large")
	}
	return body, nil
}

func decodeConnectionPage(body []byte, expectedID string, offset, limit int) (*ConnectionPage, error) {
	var wire struct {
		ID     string             `json:"server_id"`
		Now    time.Time          `json:"now"`
		Count  *int               `json:"num_connections"`
		Total  *int               `json:"total"`
		Offset *int               `json:"offset"`
		Limit  *int               `json:"limit"`
		Items  []ConnectionSample `json:"connections"`
	}
	if json.Unmarshal(body, &wire) != nil || wire.ID != expectedID || wire.Now.IsZero() || wire.Count == nil || wire.Total == nil || wire.Offset == nil || wire.Limit == nil || wire.Items == nil {
		return nil, errors.New("invalid connection monitoring response")
	}
	if *wire.Total < 0 || *wire.Count != len(wire.Items) || *wire.Count > limit || *wire.Count > *wire.Total || *wire.Offset != offset || *wire.Limit != limit {
		return nil, errors.New("inconsistent connection page metadata")
	}
	if len(wire.Items) > max(0, *wire.Total-offset) {
		return nil, errors.New("connection rows exceed remaining total")
	}
	// Churn can shorten a page. Do not invent missing rows or assume its total
	// is the number of rows returned; an out-of-range offset may return empty.
	seen := make(map[uint64]bool, len(wire.Items))
	var previous uint64
	for _, row := range wire.Items {
		if row.CID == 0 || seen[row.CID] || row.CID < previous {
			return nil, errors.New("invalid connection identity ordering")
		}
		for _, value := range []*int64{row.PendingBytes, row.InMessages, row.OutMessages, row.InBytes, row.OutBytes} {
			if value != nil && *value < 0 {
				return nil, errors.New("invalid connection counter")
			}
		}
		seen[row.CID], previous = true, row.CID
	}
	return &ConnectionPage{NodeID: wire.ID, ObservedAt: wire.Now, ReadAt: time.Now().UTC(), Offset: offset, Limit: limit, Total: *wire.Total, Items: wire.Items}, nil
}
