package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maxConnectionSubscriptions = 1000

var ErrConnectionSubscriptionLimit = errors.New("connection subscription count exceeds expansion limit")

type ConnectionSubscription struct {
	SID      string `json:"sid"`
	Subject  string `json:"subject"`
	Queue    string `json:"queue,omitempty"`
	Messages int64  `json:"messages"`
	Maximum  *int64 `json:"maximum,omitempty"`
}

type ConnectionSubscriptions struct {
	NodeID     string                   `json:"node_id"`
	CID        uint64                   `json:"cid"`
	ObservedAt time.Time                `json:"observed_at"`
	ReadAt     time.Time                `json:"read_at"`
	Items      []ConnectionSubscription `json:"items"`
}

// ConnectionSubscriptions performs a numeric preflight and only then expands
// one exact CID. The two observations must agree; churn is unavailable rather
// than a partial snapshot. Upstream has no independently paged subscription API.
func (c *Client) ConnectionSubscriptions(ctx context.Context, endpointIndex int, expectedID string, cid uint64) (*ConnectionSubscriptions, error) {
	preflight, err := c.ConnectionDetail(ctx, endpointIndex, expectedID, cid)
	if err != nil {
		return nil, err
	}
	if preflight.Item.Subscriptions == nil {
		return nil, ErrConnectionUnavailable
	}
	count := int(*preflight.Item.Subscriptions)
	if count > maxConnectionSubscriptions {
		return nil, ErrConnectionSubscriptionLimit
	}
	body, err := c.connectionRead(ctx, endpointIndex, "/connz", url.Values{
		"cid": {strconv.FormatUint(cid, 10)}, "offset": {"0"}, "limit": {"1"},
		"sort": {"cid"}, "state": {"open"}, "auth": {"false"}, "subs": {"detail"},
	})
	if err != nil {
		return nil, ErrConnectionUnavailable
	}
	var wire struct {
		ID    string    `json:"server_id"`
		Now   time.Time `json:"now"`
		Count *int      `json:"num_connections"`
		Items []struct {
			CID           uint64  `json:"cid"`
			Subscriptions *uint32 `json:"subscriptions"`
			Details       []struct {
				Subject  string `json:"subject"`
				Queue    string `json:"qgroup"`
				SID      string `json:"sid"`
				Messages *int64 `json:"msgs"`
				Maximum  *int64 `json:"max,omitempty"`
				CID      uint64 `json:"cid"`
			} `json:"subscriptions_list_detail"`
		} `json:"connections"`
	}
	if json.Unmarshal(body, &wire) != nil || wire.ID != expectedID || wire.Now.IsZero() || wire.Count == nil || *wire.Count != 1 || len(wire.Items) != 1 || wire.Items[0].CID != cid || wire.Items[0].Subscriptions == nil {
		return nil, ErrConnectionUnavailable
	}
	currentCount := int(*wire.Items[0].Subscriptions)
	if currentCount > maxConnectionSubscriptions {
		return nil, ErrConnectionSubscriptionLimit
	}
	if currentCount != count || len(wire.Items[0].Details) != currentCount {
		return nil, ErrConnectionUnavailable
	}
	items := make([]ConnectionSubscription, 0, currentCount)
	seen := make(map[string]struct{}, currentCount)
	for _, row := range wire.Items[0].Details {
		if row.CID != cid || row.Messages == nil || *row.Messages < 0 || row.Maximum != nil && *row.Maximum < 0 || !validSubscriptionText(row.SID, 256, false) || !validSubscriptionText(row.Subject, 1024, false) || !validSubscriptionText(row.Queue, 512, true) {
			return nil, ErrConnectionUnavailable
		}
		if _, exists := seen[row.SID]; exists {
			return nil, ErrConnectionUnavailable
		}
		seen[row.SID] = struct{}{}
		items = append(items, ConnectionSubscription{SID: row.SID, Subject: row.Subject, Queue: row.Queue, Messages: *row.Messages, Maximum: row.Maximum})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SID < items[j].SID })
	return &ConnectionSubscriptions{NodeID: expectedID, CID: cid, ObservedAt: wire.Now, ReadAt: time.Now().UTC(), Items: items}, nil
}

func validSubscriptionText(value string, limit int, empty bool) bool {
	if !utf8.ValidString(value) || len(value) > limit || !empty && value == "" || strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) }) >= 0 {
		return false
	}
	return true
}
