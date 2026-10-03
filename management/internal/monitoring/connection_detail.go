package monitoring

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"
)

// ErrConnectionMissing means a valid observation of the expected node found
// no open connection with this CID. It is not a node-availability verdict.
var ErrConnectionMissing = errors.New("open connection not found on expected node")

type ConnectionDetail struct {
	NodeID     string           `json:"node_id"`
	ObservedAt time.Time        `json:"observed_at"`
	ReadAt     time.Time        `json:"read_at"`
	Item       ConnectionSample `json:"item"`
}

// ConnectionDetail asks the configured endpoint for an exact open CID. A CID
// is meaningful only together with the expected server identity. This does not
// enumerate pages or request sensitive client metadata/subscription subjects.
func (c *Client) ConnectionDetail(ctx context.Context, endpointIndex int, expectedID string, cid uint64) (*ConnectionDetail, error) {
	if endpointIndex < 0 || endpointIndex >= len(c.endpoints) || !validConnectionNodeID(expectedID) || cid == 0 {
		return nil, ErrConnectionQuery
	}
	body, err := c.connectionRead(ctx, endpointIndex, "/connz", url.Values{
		"cid": {strconv.FormatUint(cid, 10)}, "offset": {"0"}, "limit": {"1"},
		"sort": {"cid"}, "state": {"open"}, "auth": {"false"}, "subs": {"false"},
	})
	if err != nil {
		return nil, ErrConnectionUnavailable
	}
	page, err := decodeConnectionPage(body, expectedID, 0, 1)
	if err != nil {
		return nil, ErrConnectionUnavailable
	}
	// connz preserves the node total even when an exact CID filter returns
	// zero or one row. Only the validated rows establish this CID's presence.
	if len(page.Items) == 0 {
		return nil, ErrConnectionMissing
	}
	if page.Items[0].CID != cid {
		return nil, ErrConnectionUnavailable
	}
	return &ConnectionDetail{NodeID: page.NodeID, ObservedAt: page.ObservedAt, ReadAt: page.ReadAt, Item: page.Items[0]}, nil
}
