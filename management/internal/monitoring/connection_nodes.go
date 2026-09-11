package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	ErrConnectionQuery         = errors.New("invalid node connection query")
	ErrConnectionNodeMissing   = errors.New("node not found in configured endpoints")
	ErrConnectionNodeAmbiguous = errors.New("node identity matches multiple configured endpoints")
	ErrConnectionUnavailable   = errors.New("node connection observation unavailable")
	ErrConnectionEndpointLimit = errors.New("configured endpoint count exceeds connection lookup limit")
	ErrConnectionSearchLimit   = errors.New("connection identity search exceeds result limit")
)

// NodeConnections resolves an exact server ID, never a caller-supplied URL.
// A missing result requires complete identity coverage. Even a unique observed
// match is not used when another endpoint failed: uniqueness is then unknown.
func validConnectionNodeID(id string) bool {
	return id != "" && len(id) <= 256 && !strings.ContainsAny(id, "/\\") && strings.IndexFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
}

func (c *Client) NodeConnectionIdentityPage(ctx context.Context, nodeID, kind, value string, offset, limit int) (*ConnectionPage, error) {
	if !validConnectionNodeID(nodeID) || !validConnectionIdentity(kind, value) || offset < 0 || offset > 1_000_000 || limit < 1 || limit > 200 {
		return nil, ErrConnectionQuery
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	selected, err := c.resolveConnectionNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return c.ConnectionIdentityPage(ctx, selected, nodeID, kind, value, offset, limit)
}

func (c *Client) NodeConnections(ctx context.Context, nodeID string, offset, limit int) (*ConnectionPage, error) {
	if !validConnectionNodeID(nodeID) || offset < 0 || offset > 1_000_000 || limit < 1 || limit > 200 {
		return nil, ErrConnectionQuery
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	selected, err := c.resolveConnectionNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	page, err := c.ConnectionPage(ctx, selected, nodeID, offset, limit)
	if err != nil {
		return nil, ErrConnectionUnavailable
	}
	return page, nil
}

func (c *Client) NodeConnection(ctx context.Context, nodeID string, cid uint64) (*ConnectionDetail, error) {
	if !validConnectionNodeID(nodeID) || cid == 0 {
		return nil, ErrConnectionQuery
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	selected, err := c.resolveConnectionNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return c.ConnectionDetail(ctx, selected, nodeID, cid)
}

func (c *Client) NodeConnectionSubscriptions(ctx context.Context, nodeID string, cid uint64) (*ConnectionSubscriptions, error) {
	if !validConnectionNodeID(nodeID) || cid == 0 {
		return nil, ErrConnectionQuery
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	selected, err := c.resolveConnectionNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return c.ConnectionSubscriptions(ctx, selected, nodeID, cid)
}

func (c *Client) NodeConnectionCIDPage(ctx context.Context, nodeID string, cid uint64, limit int) (*ConnectionPage, error) {
	if !validConnectionNodeID(nodeID) || cid == 0 || limit < 1 || limit > 200 {
		return nil, ErrConnectionQuery
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	selected, err := c.resolveConnectionNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return c.ConnectionCIDPage(ctx, selected, nodeID, cid, limit)
}

// Callers validate identity and impose one deadline covering resolution and data.
func (c *Client) resolveConnectionNode(ctx context.Context, nodeID string) (int, error) {
	if len(c.endpoints) > 32 {
		return -1, ErrConnectionEndpointLimit
	}
	if len(c.endpoints) == 0 {
		return -1, ErrConnectionUnavailable
	}
	type identity struct {
		id    string
		valid bool
	}
	identities := make([]identity, len(c.endpoints))
	semaphore := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for index := range c.endpoints {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()
			body, err := c.connectionRead(ctx, index, "/varz", nil)
			if err != nil {
				return
			}
			var value struct {
				ID string `json:"server_id"`
			}
			if json.Unmarshal(body, &value) != nil || !validConnectionNodeID(value.ID) {
				return
			}
			identities[index] = identity{id: value.ID, valid: true}
		}(index)
	}
	wg.Wait()
	matches, selected, incomplete := 0, -1, false
	for index, value := range identities {
		if !value.valid {
			incomplete = true
		}
		if value.valid && value.id == nodeID {
			matches++
			selected = index
		}
	}
	if matches > 1 {
		return -1, ErrConnectionNodeAmbiguous
	}
	if ctx.Err() != nil || incomplete {
		return -1, ErrConnectionUnavailable
	}
	if matches == 0 {
		return -1, ErrConnectionNodeMissing
	}
	return selected, nil
}
